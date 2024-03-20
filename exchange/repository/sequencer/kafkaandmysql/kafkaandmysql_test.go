package kafkaandmysql

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	mqKit "github.com/superj80820/system-design/kit/mq"
	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	ormKit "github.com/superj80820/system-design/kit/orm"
	kafkaContainer "github.com/superj80820/system-design/kit/testing/kafka/container"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
)

type testSetup struct {
	orm                  *ormKit.DB
	sequenceMessageTopic mqKit.MQTopic
	sequencerRepo        domain.SequencerRepo
	teardown             func() error
}

func testSetupFn(t *testing.T) *testSetup {
	ctx := context.Background()

	kafkaContainer, err := kafkaContainer.CreateKafka(ctx)
	assert.Nil(t, err)
	sequenceMessageTopic, err := kafkaMQKit.CreateMQTopic(
		ctx,
		kafkaContainer.GetURI(),
		"SEQUENCE",
		kafkaMQKit.ConsumeByGroupID("test", true),
		1000,
		100*time.Millisecond,
		kafkaMQKit.CreateTopic(1, 1),
	)
	assert.Nil(t, err)

	mysqlContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(filepath.Join(".", "schema.sql")))
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlContainer.GetURI()))
	assert.Nil(t, err)

	sequencerRepo, err := CreateSequencerRepo(ctx, sequenceMessageTopic, mysqlDB)
	assert.Nil(t, err)

	return &testSetup{
		orm:                  mysqlDB,
		sequenceMessageTopic: sequenceMessageTopic,
		sequencerRepo:        sequencerRepo,
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
			scenario: "test get history events",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.sequencerRepo.SaveEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
				}))
				sequencerEvents, err := testSetup.sequencerRepo.FilterEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
					{
						ReferenceID: 4,
						SequenceID:  4,
					},
				})
				assert.Nil(t, err)
				assert.Equal(t, []*domain.SequencerEvent{{
					ReferenceID: 4,
					SequenceID:  4,
				}}, sequencerEvents)

				sequencerEvents, err = testSetup.sequencerRepo.FilterEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
				})
				assert.ErrorIs(t, err, domain.ErrNoop)
				assert.Nil(t, sequencerEvents)
			},
		},
		{
			scenario: "test get history events",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.sequencerRepo.SaveEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
				}))
				expectedResults := []int{1, 2, 3}
				var count int
				for page, isEnd := 1, false; !isEnd; page++ {
					var (
						historySequenceEvents []*domain.SequencerEvent
						err                   error
					)
					historySequenceEvents, isEnd, err = testSetup.sequencerRepo.GetHistoryEvents(1, page, 2)
					assert.Nil(t, err)
					for _, event := range historySequenceEvents {
						assert.Equal(t, expectedResults[count], event.SequenceID)
						count++
					}
				}
				assert.Equal(t, 3, count)
			},
		},
		{
			scenario: "test get max sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.sequencerRepo.SaveEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
				}))
				maxSequenceID, err := testSetup.sequencerRepo.GetMaxSequenceID()
				assert.Nil(t, err)
				assert.Equal(t, uint64(3), maxSequenceID)
			},
		},
		{
			scenario: "test init get max sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				assert.Nil(t, testSetup.sequencerRepo.SaveEvents([]*domain.SequencerEvent{
					{
						ReferenceID: 1,
						SequenceID:  1,
					},
					{
						ReferenceID: 2,
						SequenceID:  2,
					},
					{
						ReferenceID: 3,
						SequenceID:  3,
					},
				}))
				sequencerRepo, err := CreateSequencerRepo(context.Background(), testSetup.sequenceMessageTopic, testSetup.orm)
				assert.Nil(t, err)
				assert.Equal(t, uint64(3), sequencerRepo.GetSequenceID())
			},
		},
		{
			scenario: "test current and generate next sequence id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				previousID := testSetup.sequencerRepo.GetSequenceID()
				testSetup.sequencerRepo.SetSequenceID(previousID + 1)
				sequenceID := testSetup.sequencerRepo.GetSequenceID()
				assert.Equal(t, uint64(0), previousID)
				assert.Equal(t, uint64(1), sequenceID)

				currentID := testSetup.sequencerRepo.GetSequenceID()
				assert.Equal(t, uint64(1), currentID)
			},
		},
		{
			scenario: "test produce and consume event",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardown()

				sequenceIDCh := make(chan int)
				testSetup.sequencerRepo.ConsumeSequenceMessages(func(events []*domain.SequencerEvent, commitFn func() error) {
					for _, event := range events {
						sequenceIDCh <- event.SequenceID
					}
				})
				time.Sleep(10 * time.Second)
				assert.Nil(t, testSetup.sequencerRepo.ProduceSequenceMessages(context.Background(), &domain.SequencerEvent{
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
