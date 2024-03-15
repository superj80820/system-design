package memory

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/kit/mq"
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

func TestMemory(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test consume 10000 message when no use ring buffer",
			fn: func(t *testing.T) {
				mqTopic := CreateMemoryMQ(ctx, 100, 1*time.Millisecond)

				resultCh := make(chan *testMessageStruct)
				mqTopic.Subscribe("key", func(message []byte) error {
					var textMessage testMessageStruct
					if err := json.Unmarshal(message, &textMessage); err != nil {
						return errors.Wrap(err, "unmarshal failed")
					}
					resultCh <- &textMessage
					return nil
				})

				messages := make([]mq.Message, 10000)
				for i := 0; i < 10000; i++ {
					messages[i] = &testMessageStruct{
						Data: strconv.Itoa(i),
					}
				}

				go func() {
					for _, message := range messages {
						assert.Nil(t, mqTopic.Produce(ctx, message))
					}
				}()

				var results []mq.Message
				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				func() {
					for {
						select {
						case <-timeout.C:
							assert.Fail(t, "timeout")
						case message := <-resultCh:
							results = append(results, message)
							time.Sleep(1 * time.Millisecond)
							if message.Data == "9999" {
								assert.Equal(t, "9999", message.Data)
								return
							}
						}
					}
				}()
				for idx, message := range messages {
					expected, err := message.Marshal()
					assert.Nil(t, err)
					actual, err := results[idx].Marshal()
					assert.Nil(t, err)
					assert.Equal(t, expected, actual)
				}
			},
		},
		{
			scenario: "test consume less 10000 message when use ring buffer and last message is correct",
			fn: func(t *testing.T) {
				mqTopic := CreateMemoryMQ(ctx, 10, 100*time.Millisecond, UseRingBuffer())

				resultCh := make(chan *testMessageStruct)
				mqTopic.Subscribe("key", func(message []byte) error {
					var textMessage testMessageStruct
					if err := json.Unmarshal(message, &textMessage); err != nil {
						return errors.Wrap(err, "unmarshal failed")
					}
					resultCh <- &textMessage
					return nil
				})

				messages := make([]mq.Message, 10000)
				for i := 0; i < 10000; i++ {
					messages[i] = &testMessageStruct{
						Data: strconv.Itoa(i),
					}
				}

				go func() {
					assert.Nil(t, mqTopic.ProduceBatch(ctx, messages))
				}()

				var results []mq.Message
				timeout := time.NewTimer(30 * time.Second)
				defer timeout.Stop()
				func() {
					for {
						select {
						case <-timeout.C:
							assert.Fail(t, "timeout")
						case message := <-resultCh:
							results = append(results, message)
							if message.Data == "9999" {
								assert.Equal(t, "9999", message.Data)
								return
							}
						}
					}
				}()
				assert.True(t, len(results) < 10000)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}

func BenchmarkMemory(b *testing.B) {
	ctx := context.Background()

	mqTopic := CreateMemoryMQ(ctx, 100, 10*time.Millisecond)
	messages := make([]mq.Message, 10000)
	for i := 0; i < 10000; i++ {
		messages[i] = &testMessageStruct{
			Data: strconv.Itoa(i),
		}
	}
	resultCh := make(chan *testMessageStruct)
	mqTopic.Subscribe("key", func(message []byte) error {
		var textMessage testMessageStruct
		if err := json.Unmarshal(message, &textMessage); err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		resultCh <- &textMessage
		return nil
	})

	for i := 0; i < b.N; i++ {
		go func() {
			for _, message := range messages {
				assert.Nil(b, mqTopic.Produce(ctx, message))
			}

		}()
		func() {
			for message := range resultCh {
				if message.Data == "9999" {
					assert.Equal(b, "9999", message.Data)
					return
				}
			}
		}()
	}
}
