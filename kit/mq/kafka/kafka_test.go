package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

type testMessageStruct struct {
	Data string
}

func (t *testMessageStruct) GetKey() string {
	return t.Data
}

func (t *testMessageStruct) Marshal() ([]byte, error) {
	marshal, err := json.Marshal(*t)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshal, nil
}

type testSetup struct {
	kafkaURI   string
	teardownFn func() error
}

func testSetupFn(ctx context.Context, t assert.TestingT) *testSetup {
	kafkaContainer, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		panic(err)
	}
	kafkaHost, err := kafkaContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9093") // TODO: is correct?
	if err != nil {
		panic(err)
	}

	return &testSetup{
		kafkaURI: fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port()),
		teardownFn: func() error {
			if err := kafkaContainer.Terminate(ctx); err != nil {
				return errors.Wrap(err, "terminate kafka failed")
			}
			return nil
		},
	}
}

func TestKafka(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test produce and consume by group id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				testTopicName := "TEST_TOPIC"
				groupID := "GROUP_ID"

				mqTopic, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, false),
					100,
					100*time.Millisecond,
					CreateTopic(1, 1),
				)
				assert.Nil(t, err)

				resultCh := make(chan *testMessageStruct)
				mqTopic.Subscribe("key", func(message []byte) error {
					var textMessage testMessageStruct
					if err := json.Unmarshal(message, &textMessage); err != nil {
						return errors.Wrap(err, "unmarshal failed")
					}
					resultCh <- &textMessage
					return nil
				})

				expect := "data"
				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: expect,
				}))

				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				select {
				case <-timeout.C:
					assert.Fail(t, "timeout")
				case message := <-resultCh:
					assert.Equal(t, expect, message.Data)
				}
			},
		},
		{
			scenario: "test produce and consume batch by group id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				testTopicName := "TEST_TOPIC"
				groupID := "GROUP_ID"

				mqTopic, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, false),
					100,
					100*time.Millisecond,
					CreateTopic(1, 1),
				)
				assert.Nil(t, err)

				resultCh := make(chan *testMessageStruct)
				mqTopic.SubscribeBatch("key", func(messages [][]byte) error {
					for _, messageRawData := range messages {
						var testMessage testMessageStruct
						assert.Nil(t, json.Unmarshal(messageRawData, &testMessage))
						resultCh <- &testMessage
					}
					return nil
				})

				expect := "data"
				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: expect,
				}))

				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				select {
				case <-timeout.C:
					assert.Fail(t, "timeout")
				case message := <-resultCh:
					assert.Equal(t, expect, message.Data)
				}
			},
		},
		{
			scenario: "test no commit by group id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				testTopicName := "TEST_TOPIC"
				groupID := "GROUP_ID"

				mqTopic, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, true),
					100,
					10*time.Second,
					CreateTopic(1, 1),
				)
				assert.Nil(t, err)

				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: "a",
				}))
				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: "b",
				}))

				resultCh := make(chan *testMessageStruct)
				mqTopic.SubscribeBatchWithManualCommit("key", func(messages [][]byte, commitFn func() error) error {
					assert.Equal(t, 2, len(messages))
					for _, message := range messages {
						var textMessage testMessageStruct
						if err := json.Unmarshal(message, &textMessage); err != nil {
							return errors.Wrap(err, "unmarshal failed")
						}
						resultCh <- &textMessage
					}
					return nil
				})

				var results []string

				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				for i := 0; i < 2; i++ {
					select {
					case <-timeout.C:
						assert.Fail(t, "timeout")
					case message := <-resultCh:
						results = append(results, message.Data)
					}
				}

				mqTopic2, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, true),
					100,
					100*time.Millisecond,
				)
				assert.Nil(t, err)

				mqTopic2.SubscribeWithManualCommit("key", func(message []byte, commitFn func() error) error {
					var textMessage testMessageStruct
					if err := json.Unmarshal(message, &textMessage); err != nil {
						return errors.Wrap(err, "unmarshal failed")
					}
					resultCh <- &textMessage
					return nil
				})

				timeout2 := time.NewTimer(30 * time.Second)
				defer timeout2.Stop()
				for i := 0; i < 2; i++ {
					select {
					case <-timeout2.C:
						assert.Fail(t, "timeout")
					case message := <-resultCh:
						results = append(results, message.Data)
					}
				}

				assert.ElementsMatch(t, []string{"a", "b", "a", "b"}, results)
			},
		},
		{
			scenario: "test commit by group id",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				testTopicName := "TEST_TOPIC"
				groupID := "GROUP_ID"

				mqTopic, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, true),
					100,
					10*time.Second,
					CreateTopic(1, 1),
				)
				assert.Nil(t, err)

				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: "a",
				}))
				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: "b",
				}))

				resultCh := make(chan *testMessageStruct)
				mqTopic.SubscribeBatchWithManualCommit("key", func(messages [][]byte, commitFn func() error) error {
					assert.Equal(t, 2, len(messages))
					for _, message := range messages {
						var textMessage testMessageStruct
						if err := json.Unmarshal(message, &textMessage); err != nil {
							return errors.Wrap(err, "unmarshal failed")
						}
						resultCh <- &textMessage
						assert.Nil(t, commitFn())
					}
					return nil
				})

				var results []string

				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				for i := 0; i < 2; i++ {
					select {
					case <-timeout.C:
						assert.Fail(t, "timeout")
					case message := <-resultCh:
						results = append(results, message.Data)
					}
				}

				mqTopic2, err := CreateMQTopic(
					ctx,
					testSetup.kafkaURI,
					testTopicName,
					ConsumeByGroupID(groupID, true),
					100,
					100*time.Millisecond,
				)
				assert.Nil(t, err)

				mqTopic2.SubscribeWithManualCommit("key", func(message []byte, commitFn func() error) error {
					var textMessage testMessageStruct
					if err := json.Unmarshal(message, &textMessage); err != nil {
						return errors.Wrap(err, "unmarshal failed")
					}
					resultCh <- &textMessage
					assert.Nil(t, commitFn())
					return nil
				})

				assert.Nil(t, mqTopic.Produce(ctx, &testMessageStruct{
					Data: "c",
				}))

				timeout2 := time.NewTimer(30 * time.Second)
				defer timeout2.Stop()
				select {
				case <-timeout2.C:
					assert.Fail(t, "timeout")
				case message := <-resultCh:
					results = append(results, message.Data)
				}

				assert.ElementsMatch(t, []string{"a", "b", "c"}, results)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
