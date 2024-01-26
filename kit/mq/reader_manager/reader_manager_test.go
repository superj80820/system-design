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
	"github.com/stretchr/testify/suite"
	"github.com/superj80820/system-design/kit/mq/reader_manager/mocks"
	"golang.org/x/sync/errgroup"
)

type unitTest struct {
	suite.Suite

	readerManagerForTestProvider func(...ReaderManagerConfigOption) (ReaderManager, error)

	readerManager   ReaderManager
	kafkaReaderMock *mocks.KafkaReader
	observer        *Observer
	messageCh       chan []byte
	hookCh          chan bool
	pauseHookCh     chan bool
	errorHandleCh   chan bool
	kafkaConnMock   *mocks.KafkaConn
}

func (u *unitTest) SetupTest() {
	kafkaReaderMock := new(mocks.KafkaReader)
	kafkaReaderMock.On("Config").Return(kafka.ReaderConfig{Partition: 1, StartOffset: LastOffset})

	kafkaConn := new(mocks.KafkaConn)
	kafkaConn.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil).Once()

	pauseHookCh := make(chan bool)
	errorHandleCh := make(chan bool)
	rm, err := u.readerManagerForTestProvider(
		useMockReaderProvider(
			func() KafkaReader {
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
	assert.NoError(u.T(), err)

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

	u.readerManager = rm
	u.kafkaReaderMock = kafkaReaderMock
	u.observer = observer
	u.messageCh = messageCh
	u.hookCh = hookCh
	u.pauseHookCh = pauseHookCh
	u.errorHandleCh = errorHandleCh
	u.kafkaConnMock = kafkaConn

}

func (u *unitTest) TestReadMessage() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	expectMessage := "message"
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh }).
		Return(kafka.Message{Value: []byte(expectMessage)}, nil)
	u.readerManager.SyncStartConsume(context.Background())

	timeout := time.After(3 * time.Second)
	for {
		select {
		case message := <-u.messageCh:
			assert.Equal(u.T(), []byte(expectMessage), []byte(message))
			return
		case <-timeout:
			assert.Fail(u.T(), "get message error")
			return
		default:
			go func() { execReadMessageCh <- true }()
		}
	}
}

func (u *unitTest) TestRemoveObserver() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh })
	u.readerManager.SyncStartConsume(context.Background())

	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-u.hookCh:
			return
		case <-timeout:
			assert.Fail(u.T(), "execute message error")
			return
		default:
			u.readerManager.RemoveObserverWithHook(u.observer)
		}
	}
}

func (u *unitTest) TestStopConsume() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
		}).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-u.pauseHookCh:
			return
		case <-timeout:
			assert.Fail(u.T(), "execute pause hook error")
			return
		default:
			u.readerManager.StopConsume()
		}
	}
}

func (u *unitTest) TestRemoveObserverWithDefaultHook() {
	u.kafkaReaderMock.On("SetOffset", LastOffset).Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh })
	u.readerManager.SyncStartConsume(context.Background())

	observerWithDefaultHook := CreateObserver("noHookKey", func(message []byte) error {
		return nil
	})
	u.readerManager.AddObserver(observerWithDefaultHook)
	u.readerManager.RemoveObserverWithHook(observerWithDefaultHook)
}

func (u *unitTest) TestStopConsumeWhenNoObservers() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
		}).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	observerWithDefaultHook := CreateObserver("noHookKey", func(message []byte) error {
		return nil
	})
	u.readerManager.AddObserver(observerWithDefaultHook)

	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-u.pauseHookCh:
			return
		case <-timeout:
			assert.Fail(u.T(), "execute pause hook error")
			return
		default:
			u.readerManager.RemoveObserverWithHook(u.observer)
			u.readerManager.RemoveObserverWithHook(observerWithDefaultHook)
			u.readerManager.IfNoObserversThenStopConsume()
		}
	}
}

func (u *unitTest) TestCanNotAddSameObserver() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh }).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	assert.False(u.T(), u.readerManager.AddObserver(u.observer))
}

func (u *unitTest) TestExecErrorHandle() {
	u.kafkaReaderMock.On("Close").Return(errors.New("get error"))
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh }).
		Return(kafka.Message{}, errors.New("get error"))

	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-u.errorHandleCh:
			return
		case <-timeout:
			assert.Fail(u.T(), "execute error handle failed")
			return
		default:
			u.readerManager.StartConsume(context.Background())
			go func() { execReadMessageCh <- true }()
		}
	}
}

func (u *unitTest) TestStartConsumeNoDeadlock() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh })
	u.readerManager.SyncStartConsume(context.Background())

	u.readerManager.StartConsume(context.Background())
	u.readerManager.StartConsume(context.Background())
	u.readerManager.StartConsume(context.Background())
}

func (u *unitTest) TestStopConsumeNoDeadlock() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	execReadMessageCh := make(chan bool)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) { <-execReadMessageCh })
	u.readerManager.SyncStartConsume(context.Background())

	u.readerManager.StopConsume()
	u.readerManager.StopConsume()
	u.readerManager.StopConsume()
}

func (u *unitTest) TestStartAndStopConsumeNoDeadlock() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
		}).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	eg := new(errgroup.Group)
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			u.readerManager.StopConsume()
		}
		return nil
	})
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			u.readerManager.StopConsume()
		}
		return nil
	})
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			u.readerManager.StartConsume(context.Background())
		}
		return nil
	})
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			u.readerManager.StartConsume(context.Background())
		}
		return nil
	})
	eg.Wait()
}

func (u *unitTest) TestAddObserverNoRaceCondition() {
	u.kafkaReaderMock.On("Close").Return(nil)
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return([]kafka.Partition{
		{ID: 0}, {ID: 1}, {ID: 2},
	}, nil)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
		}).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	eg := new(errgroup.Group)
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			observerWithDefaultHook := CreateObserver("a"+strconv.Itoa(i), func(message []byte) error {
				return nil
			})
			u.readerManager.AddObserver(observerWithDefaultHook)
		}
		return nil
	})
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			observerWithDefaultHook := CreateObserver("b"+strconv.Itoa(i), func(message []byte) error {
				return nil
			})
			u.readerManager.AddObserver(observerWithDefaultHook)
		}
		return nil
	})
	eg.Wait()

	assert.Equal(u.T(), u.readerManager.GetObserversLen(), 2000001)
}

func (u *unitTest) TestAddObserverWithReBalanceNoRaceCondition() {
	u.kafkaReaderMock.On("Close").Return(nil)
	for i := 0; i < 8; i++ {
		partitions := make([]kafka.Partition, i+1)
		for j := 0; j <= i; j++ {
			partitions[j] = kafka.Partition{ID: j}
		}
		u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return(
			partitions,
			nil,
		).Once()
	}
	partitions := make([]kafka.Partition, 10)
	for j := 0; j <= 9; j++ {
		partitions[j] = kafka.Partition{ID: j}
	}
	u.kafkaConnMock.On("ReadPartitions", mock.Anything).Return(
		partitions,
		nil,
	)
	u.kafkaReaderMock.On("ReadMessage", mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
		}).
		Return(kafka.Message{}, errors.New("stop consume"))
	u.readerManager.SyncStartConsume(context.Background())

	eg := new(errgroup.Group)
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			observerWithDefaultHook := CreateObserver("a"+strconv.Itoa(i), func(message []byte) error {
				return nil
			})
			u.readerManager.AddObserver(observerWithDefaultHook)
		}
		return nil
	})
	removeCh := make(chan *Observer, 10000)
	eg.Go(func() error {
		for i := 0; i < 1000000; i++ {
			observerWithDefaultHook := CreateObserver("b"+strconv.Itoa(i), func(message []byte) error {
				return nil
			})
			u.readerManager.AddObserver(observerWithDefaultHook)
			if i%2 == 0 {
				removeCh <- observerWithDefaultHook
			}
		}
		close(removeCh)
		return nil
	})
	eg.Go(func() error {
		for observer := range removeCh {
			if ok := u.readerManager.RemoveObserverWithHook(observer); !ok {
				assert.Fail(u.T(), "remove failed")
			}
		}
		return nil
	})
	eg.Go(func() error {
		for observer := range removeCh {
			if ok := u.readerManager.RemoveObserverWithHook(observer); !ok {
				assert.Fail(u.T(), "remove failed")
			}
		}
		return nil
	})
	eg.Wait()

	assert.Equal(u.T(), 1500001, u.readerManager.GetObserversLen())
}

type unitTestReaderBySpecPartition struct {
	unitTest
}

func (u *unitTestReaderBySpecPartition) SetupSuite() {
	u.readerManagerForTestProvider = func(rmco ...ReaderManagerConfigOption) (ReaderManager, error) {
		return CreateSpecPartitionReaderManager(context.Background(), "topic", LastOffset, 1, []string{}, rmco...)
	}
}

type unitTestReaderByGroupID struct {
	unitTest
}

func (u *unitTestReaderByGroupID) SetupSuite() {
	u.readerManagerForTestProvider = func(rmco ...ReaderManagerConfigOption) (ReaderManager, error) {
		return CreateGroupIDReaderManager(context.Background(), []string{}, "topic", "groupID", rmco...)
	}
}

type unitTestPartitionBindObserverReaderManager struct {
	unitTest
}

func (u *unitTestPartitionBindObserverReaderManager) SetupSuite() {
	u.readerManagerForTestProvider = func(rmco ...ReaderManagerConfigOption) (ReaderManager, error) {
		return CreatePartitionBindObserverReaderManager(context.Background(), "url", LastOffset, []string{}, "topic", rmco...)
	}
}

func TestReaderManager(t *testing.T) {
	suite.Run(t, new(unitTestReaderBySpecPartition))
	suite.Run(t, new(unitTestReaderByGroupID))
	suite.Run(t, new(unitTestPartitionBindObserverReaderManager))
}
