package kafkaandmysql

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	mqKit "github.com/superj80820/system-design/kit/mq"
	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

type testSetup struct {
	orm                  *ormKit.DB
	sequenceMessageTopic mqKit.MQTopic
	tradingSequencerRepo domain.SequencerRepo[domain.TradingEvent]
	teardown             func() error
}

func testSetupFn(t *testing.T) *testSetup {
	ctx := context.Background()

	kafkaContainer, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	assert.Nil(t, err)
	kafkaHost, err := kafkaContainer.Host(ctx)
	assert.Nil(t, err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9093") // TODO: is correct?
	assert.Nil(t, err)
	sequenceMessageTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port()),
		"SEQUENCE",
		kafkaMQKit.ConsumeByGroupID("test", true),
		kafkaMQKit.CreateTopic(1, 1),
	)
	assert.Nil(t, err)

	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(filepath.Join(".", "schema.sql")),
	)
	assert.Nil(t, err)
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	assert.Nil(t, err)
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(
		ormKit.UseMySQL(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			)))
	assert.Nil(t, err)

	tradingSequencerRepo, err := CreateTradingSequencerRepo(ctx, sequenceMessageTopic, mysqlDB)
	assert.Nil(t, err)

	return &testSetup{
		orm:                  mysqlDB,
		sequenceMessageTopic: sequenceMessageTopic,
		tradingSequencerRepo: tradingSequencerRepo,
		teardown: func() error {
			if err := kafkaContainer.Terminate(ctx); err != nil {
				return errors.Wrap(err, "terminate kafka container failed")
			}
			if err := mysqlContainer.Terminate(ctx); err != nil {
				return errors.Wrap(err, "terminate mysql container failed")
			}
			return nil
		},
	}
}

func TestTradingSequencerRepo(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test get max sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 1,
					PreviousID:  0,
					SequenceID:  1,
				}))
				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 2,
					PreviousID:  1,
					SequenceID:  2,
				}))
				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 3,
					PreviousID:  2,
					SequenceID:  3,
				}))
				maxSequenceID, err := testSetup.tradingSequencerRepo.GetMaxSequenceID()
				assert.Nil(t, err)
				assert.Equal(t, uint64(3), maxSequenceID)
			},
		},
		{
			scenario: "test init get max sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 1,
					PreviousID:  0,
					SequenceID:  1,
				}))
				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 2,
					PreviousID:  1,
					SequenceID:  2,
				}))
				assert.Nil(t, testSetup.tradingSequencerRepo.SaveEvent(&domain.SequencerEvent{
					ReferenceID: 3,
					PreviousID:  2,
					SequenceID:  3,
				}))
				tradingSequencerRepo, err := CreateTradingSequencerRepo(context.Background(), testSetup.sequenceMessageTopic, testSetup.orm)
				assert.Nil(t, err)
				assert.Equal(t, uint64(3), tradingSequencerRepo.GetCurrentSequenceID())
			},
		},
		{
			scenario: "test current and generate next sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				previousID := testSetup.tradingSequencerRepo.GetCurrentSequenceID()
				sequenceID := testSetup.tradingSequencerRepo.GenerateNextSequenceID()
				assert.Equal(t, uint64(0), previousID)
				assert.Equal(t, uint64(1), sequenceID)

				currentID := testSetup.tradingSequencerRepo.GetCurrentSequenceID()
				assert.Equal(t, uint64(1), currentID)
			},
		},
		{
			scenario: "test produce and consume event",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				sequenceIDCh := make(chan int)
				testSetup.tradingSequencerRepo.SubscribeTradeSequenceMessage(func(te *domain.TradingEvent, commitFn func() error) {
					sequenceIDCh <- te.SequenceID
				})
				time.Sleep(10 * time.Second)
				assert.Nil(t, testSetup.tradingSequencerRepo.SendTradeSequenceMessages(context.Background(), &domain.TradingEvent{
					SequenceID: 100,
				}))
				timer := time.NewTimer(6000 * time.Second)
				defer timer.Stop()
				select {
				case sequenceID := <-sequenceIDCh:
					assert.Equal(t, 100, sequenceID)
				case <-timer.C:
					assert.Fail(t, "get message timeout")
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
