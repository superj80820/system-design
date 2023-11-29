package readermanager

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/superj80820/system-design/kit/mq/reader_manager/mocks"
	"golang.org/x/sync/errgroup"
)

type testSetupData struct {
	readerManager   ReaderManager
	kafkaReaderMock *mocks.KafkaReader
	observer        *Observer
	messageCh       chan []byte
	hookCh          chan bool
	pauseHookCh     chan bool
	errorHandleCh   chan bool
	kafkaConnMock   *mocks.KafkaConn
}

var tests = []struct {
	scenario string
	fn       func(testSetupData *testSetupData) func(*testing.T)
}{
	{
		scenario: "read message",
		fn:       readMessage,
	},
	{
		scenario: "remove observer",
		fn:       removeObserver,
	},
	{
		scenario: "remove observer with default hook",
		fn:       removeObserverWithDefaultHook,
	},
	{
		scenario: "stop consume",
		fn:       stopConsume,
	},
	{
		scenario: "stop consume when no observers",
		fn:       stopConsumeWhenNoObservers,
	},
	{
		scenario: "can not add same observer",
		fn:       canNotAddSameObserver,
	},
	{
		scenario: "execute error handle",
		fn:       execErrorHandle,
	},
	{
		scenario: "start consume no deadlock",
		fn:       startConsumeNoDeadlock,
	},
	{
		scenario: "stop consume no deadlock",
		fn:       stopConsumeNoDeadlock,
	},
	{
		scenario: "start and stop consume no deadlock",
		fn:       startAndStopConsumeNoDeadlock,
	},
	{
		scenario: "add observer no race condition",
		fn:       addObserverNoRaceCondition,
	},
	{
		scenario: "add observer no race condition with re-balance",
		fn:       addObserverWithReBalanceNoRaceCondition,
	},
}

func TestReaderByPartitionBindObserver(t *testing.T) {
	for _, test := range tests {
		t.Run(test.scenario, test.fn(testSetup(t, func(options ...readerManagerConfigOption) (ReaderManager, error) {
			return CreatePartitionBindObserverReaderManager("url", LastOffset, []string{}, "topic", options...)
		})))
	}
}

// func TestReaderBySpecPartition(t *testing.T) {
// 	for _, test := range tests {
// 		t.Run(test.scenario, test.fn(testSetup(t, func(options ...readerManagerConfigOption) (ReaderManager, error) {
// 			return CreateSpecPartitionReaderManager("topic", LastOffset, 1, []string{}, options...)
// 		})))
// 	}
// }

// func TestReaderByGroupID(t *testing.T) {
// 	for _, test := range tests {
// 		t.Run(test.scenario, test.fn(testSetup(t, func(options ...readerManagerConfigOption) (ReaderManager, error) {
// 			return CreateGroupIDReaderManager([]string{}, "topic", "groupID", LastOffset, options...)
// 		})))
// 	}
// }

func testSetup(t *testing.T, createReaderManagerFn func(options ...readerManagerConfigOption) (ReaderManager, error)) *testSetupData {
	kafkaReaderMock := new(mocks.KafkaReader)
	kafkaReaderMock.On("Config").Return(kafka.ReaderConfig{Partition: 1, StartOffset: LastOffset})

	kafkaConn := new(mocks.KafkaConn)
	kafkaConn.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil).Once()

	pauseHookCh := make(chan bool)
	errorHandleCh := make(chan bool)
	rm, err := createReaderManagerFn(
		useMockReaderProvider(
			func(config kafka.ReaderConfig) KafkaReader {
				return kafkaReaderMock
			}),
		AddReaderPauseHookFn(func() {
			pauseHookCh <- true
		}),
		AddErrorHandleFn(func(err error) {
			errorHandleCh <- true
		}),
		useMockKafkaControllerConnProvider(func() (KafkaConn, error) {
			return kafkaConn, nil
		}),
		setWatchBalanceDuration(100*time.Millisecond),
	)
	assert.NoError(t, err)

	messageCh := make(chan []byte)
	hookCh := make(chan bool)
	observer := CreateObserver("key", func(message []byte) error {
		messageCh <- message
		return nil
	}, AddUnSubscribeHook(func() error {
		hookCh <- true
		return nil
	}))
	rm.AddObserver(observer)

	rm.Run()

	return &testSetupData{
		readerManager:   rm,
		kafkaReaderMock: kafkaReaderMock,
		observer:        observer,
		messageCh:       messageCh,
		hookCh:          hookCh,
		pauseHookCh:     pauseHookCh,
		errorHandleCh:   errorHandleCh,
		kafkaConnMock:   kafkaConn,
	}
}

func readMessage(testSetupData *testSetupData) func(*testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		expectMessage := "message"
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh }).
			Return(kafka.Message{Value: []byte(expectMessage)}, nil)
		testSetupData.readerManager.SyncStartConsume(context.Background())

		timeout := time.After(3 * time.Second)
		for {
			select {
			case message := <-testSetupData.messageCh:
				assert.Equal(t, []byte(expectMessage), []byte(message))
				return
			case <-timeout:
				assert.Fail(t, "get message error")
				return
			default:
				go func() { execReadMessageCh <- true }()
			}
		}
	}
}

func removeObserver(testSetupData *testSetupData) func(*testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh })
		testSetupData.readerManager.SyncStartConsume(context.Background())

		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-testSetupData.hookCh:
				return
			case <-timeout:
				assert.Fail(t, "execute message error")
				return
			default:
				testSetupData.readerManager.RemoveObserverWithHook(testSetupData.observer)
			}
		}
	}
}

func stopConsume(testSetupData *testSetupData) func(*testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				<-ctx.Done()
			}).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-testSetupData.pauseHookCh:
				return
			case <-timeout:
				assert.Fail(t, "execute pause hook error")
				return
			default:
				testSetupData.readerManager.StopConsume()
			}
		}
	}
}

func removeObserverWithDefaultHook(testSetupData *testSetupData) func(*testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh })
		testSetupData.readerManager.SyncStartConsume(context.Background())

		observerWithDefaultHook := CreateObserver("noHookKey", func(message []byte) error {
			return nil
		})
		testSetupData.readerManager.AddObserver(observerWithDefaultHook)
		testSetupData.readerManager.RemoveObserverWithHook(observerWithDefaultHook)
	}
}

func stopConsumeWhenNoObservers(testSetupData *testSetupData) func(*testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				<-ctx.Done()
			}).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		observerWithDefaultHook := CreateObserver("noHookKey", func(message []byte) error {
			return nil
		})
		testSetupData.readerManager.AddObserver(observerWithDefaultHook)

		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-testSetupData.pauseHookCh:
				return
			case <-timeout:
				assert.Fail(t, "execute pause hook error")
				return
			default:
				testSetupData.readerManager.RemoveObserverWithHook(testSetupData.observer)
				testSetupData.readerManager.RemoveObserverWithHook(observerWithDefaultHook)
				testSetupData.readerManager.IfNoObserversThenStopConsume()
			}
		}

	}
}

func canNotAddSameObserver(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh }).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		assert.False(t, testSetupData.readerManager.AddObserver(testSetupData.observer))
	}
}

func execErrorHandle(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(errors.New("get error"))
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh })

		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-testSetupData.errorHandleCh:
				return
			case <-timeout:
				assert.Fail(t, "execute error handle failed")
				return
			default:
				testSetupData.readerManager.StartConsume(context.Background())
			}
		}
	}
}

func startConsumeNoDeadlock(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh })
		testSetupData.readerManager.SyncStartConsume(context.Background())

		testSetupData.readerManager.StartConsume(context.Background())
		testSetupData.readerManager.StartConsume(context.Background())
		testSetupData.readerManager.StartConsume(context.Background())
	}
}

func stopConsumeNoDeadlock(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		execReadMessageCh := make(chan bool)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) { <-execReadMessageCh })
		testSetupData.readerManager.SyncStartConsume(context.Background())

		testSetupData.readerManager.StopConsume()
		testSetupData.readerManager.StopConsume()
		testSetupData.readerManager.StopConsume()
	}
}

func startAndStopConsumeNoDeadlock(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				<-ctx.Done()
			}).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		eg := new(errgroup.Group)
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				testSetupData.readerManager.StopConsume()
			}
			return nil
		})
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				testSetupData.readerManager.StopConsume()
			}
			return nil
		})
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				testSetupData.readerManager.StartConsume(context.Background())
			}
			return nil
		})
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				testSetupData.readerManager.StartConsume(context.Background())
			}
			return nil
		})
		eg.Wait()
	}
}

func addObserverNoRaceCondition(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
			{ID: 0}, {ID: 1}, {ID: 2},
		}, nil)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				<-ctx.Done()
			}).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		eg := new(errgroup.Group)
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				observerWithDefaultHook := CreateObserver("a"+strconv.Itoa(i), func(message []byte) error {
					return nil
				})
				testSetupData.readerManager.AddObserver(observerWithDefaultHook)
			}
			return nil
		})
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				observerWithDefaultHook := CreateObserver("b"+strconv.Itoa(i), func(message []byte) error {
					return nil
				})
				testSetupData.readerManager.AddObserver(observerWithDefaultHook)
			}
			return nil
		})
		eg.Wait()

		assert.Equal(t, testSetupData.readerManager.GetObserversLen(), 2000001)
	}
}
func addObserverWithReBalanceNoRaceCondition(testSetupData *testSetupData) func(t *testing.T) {
	return func(t *testing.T) {
		testSetupData.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
		for i := 0; i < 8; i++ {
			partitions := make([]kafka.Partition, i+1)
			for j := 0; j <= i; j++ {
				partitions[j] = kafka.Partition{ID: j}
			}
			testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return(
				partitions,
				nil,
			).Once()
		}
		partitions := make([]kafka.Partition, 10)
		for j := 0; j <= 9; j++ {
			partitions[j] = kafka.Partition{ID: j}
		}
		testSetupData.kafkaConnMock.On("ReadPartitions", mock.Anything).Return(
			partitions,
			nil,
		)
		testSetupData.kafkaReaderMock.On("ReadMessage", mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				<-ctx.Done()
			}).
			Return(kafka.Message{}, errors.New("stop consume"))
		testSetupData.readerManager.SyncStartConsume(context.Background())

		eg := new(errgroup.Group)
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				observerWithDefaultHook := CreateObserver("a"+strconv.Itoa(i), func(message []byte) error {
					return nil
				})
				testSetupData.readerManager.AddObserver(observerWithDefaultHook)
			}
			return nil
		})
		removeCh := make(chan *Observer, 10000)
		eg.Go(func() error {
			for i := 0; i < 1000000; i++ {
				observerWithDefaultHook := CreateObserver("b"+strconv.Itoa(i), func(message []byte) error {
					return nil
				})
				testSetupData.readerManager.AddObserver(observerWithDefaultHook)
				if i%2 == 0 {
					removeCh <- observerWithDefaultHook
				}
			}
			close(removeCh)
			return nil
		})
		eg.Go(func() error {
			for observer := range removeCh {
				if ok := testSetupData.readerManager.RemoveObserverWithHook(observer); !ok {
					assert.Fail(t, "remove failed")
				}
			}
			return nil
		})
		eg.Go(func() error {
			for observer := range removeCh {
				if ok := testSetupData.readerManager.RemoveObserverWithHook(observer); !ok {
					assert.Fail(t, "remove failed")
				}
			}
			return nil
		})
		eg.Wait()

		assert.Equal(t, 1500001, testSetupData.readerManager.GetObserversLen())
	}
}
