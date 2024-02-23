package readermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/superj80820/system-design/kit/mq"
)

type ReaderWay int

const (
	DefaultReader ReaderWay = iota // no set
	GroupIDReader
	SpecPartitionReader
	PartitionsBindObserverReader
)

const (
	FirstOffset = kafka.FirstOffset
	LastOffset  = kafka.LastOffset
)

type KafkaReader interface {
	Config() kafka.ReaderConfig
	Close() error
	ReadMessage(ctx context.Context) (kafka.Message, error)
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	ReadLag(ctx context.Context) (lag int64, err error)
	Offset() int64
	Lag() int64
	SetOffset(offset int64) error
	SetOffsetAt(ctx context.Context, t time.Time) error
	Stats() kafka.ReaderStats
}

type readerOption func(*Reader)

func useMockReaderProvider(fn func() KafkaReader) ReaderManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) { r.kafkaReaderProvider = fn })
	}
}

func AddKafkaReaderHookFn(fn func(kafkaReader KafkaReader)) ReaderManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) {
			r.kafkaReaderHookFn = append(r.kafkaReaderHookFn, fn)
		})
	}
}

func ManualCommit(rmc *readerManagerConfig) {
	rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) {
		r.isManualCommit = true
	})
}

type Reader struct {
	kafkaReader KafkaReader

	messages     []*kafka.Message
	messagesLock *sync.Mutex

	runMaxMessagesLength int
	runDuration          time.Duration

	isManualCommit bool

	kafkaReaderProvider func() KafkaReader
	kafkaReaderHookFn   []func(kafkaReader KafkaReader)

	observers       map[mq.Observer]mq.Observer
	observerBatches map[mq.Observer]mq.Observer

	pauseCh chan context.CancelFunc
	startCh chan context.Context
	readyCh chan struct{}

	pauseHookFn   func()
	errorHandleFn func(error)

	lock sync.RWMutex
}

func defaultPauseHookFn() {}

func defaultKafkaReaderProvider(config kafka.ReaderConfig) KafkaReader {
	return kafka.NewReader(config)
}

func createReader(kafkaReaderConfig kafka.ReaderConfig, options ...readerOption) *Reader {
	r := &Reader{
		messagesLock:         new(sync.Mutex),
		observers:            make(map[mq.Observer]mq.Observer),
		observerBatches:      make(map[mq.Observer]mq.Observer),
		pauseCh:              make(chan context.CancelFunc, 1),
		startCh:              make(chan context.Context),
		readyCh:              make(chan struct{}),
		pauseHookFn:          defaultPauseHookFn,
		errorHandleFn:        defaultErrorHandleFn,
		runDuration:          100 * time.Millisecond,
		runMaxMessagesLength: 1000,
	}
	for _, option := range options {
		option(r)
	}
	r.kafkaReaderProvider = func() KafkaReader {
		kafkaReader := defaultKafkaReaderProvider(kafkaReaderConfig)
		for _, fn := range r.kafkaReaderHookFn {
			fn(kafkaReader)
		}
		return kafkaReader
	}
	return r
}

func (r *Reader) Run() {
	ticker := time.NewTicker(r.runDuration)
	isMessagesFullCh := make(chan struct{})

	go func() {
		for ctx := range r.startCh {
			if r.kafkaReader != nil {
				r.kafkaReader.Close()
			} else {
				// TODO: log here
			}
			r.kafkaReader = r.kafkaReaderProvider()
			r.readyCh <- struct{}{}
			for {
				var getMessageFn func(ctx context.Context) (kafka.Message, error)
				if r.isManualCommit {
					getMessageFn = r.kafkaReader.FetchMessage // TODO: is correct?
				} else {
					getMessageFn = r.kafkaReader.ReadMessage // TODO: is correct?
				}
				// fmt.Println("start consume")
				m, err := getMessageFn(ctx)
				if err != nil {
					fmt.Println("stop consume")
					go r.pauseHookFn()
					if err := r.kafkaReader.Close(); err != nil {
						r.errorHandleFn(err)
					}
					break
				}

				r.messagesLock.Lock()
				r.messages = append(r.messages, &m)
				messagesLength := len(r.messages)
				r.messagesLock.Unlock()

				if messagesLength >= r.runMaxMessagesLength {
					isMessagesFullCh <- struct{}{}
				}
			}
		}
	}()
	go func() {
		defer ticker.Stop()

		fn := func() {
			noopErr := errors.New("no op error")
			cloneMessagesFn := func() ([][]byte, *kafka.Message, error) {
				r.messagesLock.Lock()
				defer r.messagesLock.Unlock()

				if len(r.messages) == 0 {
					return nil, nil, errors.Wrap(noopErr, "no messages")
				}
				latestKafkaMessage := new(kafka.Message)
				cloneMessages := make([][]byte, len(r.messages))
				for idx, message := range r.messages {
					cloneValue := make([]byte, len(message.Value))
					copy(cloneValue, message.Value)
					cloneMessages[idx] = cloneValue
					latestKafkaMessage = message
				}
				r.messages = nil

				return cloneMessages, latestKafkaMessage, nil
			}

			cloneMessages, latestKafkaMessage, err := cloneMessagesFn()
			if errors.Is(err, noopErr) {
				return
			} else if err != nil {
				panic(fmt.Sprintf("kafka read messages get error, error: %+v", err)) // TODO: maybe not panic
			}

			r.RangeAllObserverBatches(func(_, observerBatch mq.Observer) bool {
				if err := observerBatch.NotifyBatchWithManualCommit(cloneMessages, func() error {
					if err := r.kafkaReader.CommitMessages(context.Background(), *latestKafkaMessage); err != nil { // TODO: if context done. need time out
						return errors.Wrap(err, "commit message failed")
					}
					return nil
				}); err != nil { // TODO: think async
					r.errorHandleFn(err)
				}
				return true
			})

			for _, m := range cloneMessages {
				r.RangeAllObservers(func(_, observer mq.Observer) bool {
					if err := observer.NotifyWithManualCommit(m, func() error {
						if err := r.kafkaReader.CommitMessages(context.Background(), *latestKafkaMessage); err != nil { // TODO: if context done. need time out
							return errors.Wrap(err, "commit message failed")
						}
						return nil
					}); err != nil { // TODO: think async
						r.errorHandleFn(err)
					}
					return true
				})
			}
		}

		for {
			select {
			case <-ticker.C:
				fn()
			case <-isMessagesFullCh:
				fn()
			}
		}
	}()
}

func (r *Reader) SyncStartConsume(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	r.pauseCh <- cancel
	r.startCh <- ctx
	<-r.readyCh
	return true
}

func (r *Reader) StartConsume(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	select {
	case r.pauseCh <- cancel:
		r.startCh <- ctx
		<-r.readyCh
		return true
	default:
		cancel()
		return false
	}
}

func (r *Reader) StopConsume() bool {
	select {
	case cancel := <-r.pauseCh:
		cancel()
		return true
	default:
		return false
	}
}

func (r *Reader) AddObserverBatch(observerBatch mq.Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observerBatches[observerBatch]; ok {
		return false
	}
	r.observerBatches[observerBatch] = observerBatch

	return true
}

func (r *Reader) AddObserver(observer mq.Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observers[observer]; ok {
		return false
	}
	r.observers[observer] = observer

	return true
}

func (r *Reader) RemoveObserver(observer mq.Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observers[observer]; !ok {
		return false
	}
	delete(r.observers, observer)

	return true
}

func (r *Reader) GetObserversLen() int {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return len(r.observers)
}

func (r *Reader) RangeAllObservers(fn func(key mq.Observer, value mq.Observer) bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for key, value := range r.observers {
		if !fn(key, value) {
			return
		}
	}
}

func (r *Reader) RangeAllObserverBatches(fn func(key mq.Observer, value mq.Observer) bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for key, value := range r.observerBatches {
		if !fn(key, value) {
			return
		}
	}
}
