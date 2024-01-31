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

func AddReaderPauseHookFn(fn func()) ReaderManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) { r.pauseHookFn = fn })
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

	isManualCommit bool

	kafkaReaderProvider func() KafkaReader
	kafkaReaderHookFn   []func(kafkaReader KafkaReader)

	observers map[mq.Observer]mq.Observer

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
		observers:     make(map[mq.Observer]mq.Observer),
		pauseCh:       make(chan context.CancelFunc, 1),
		startCh:       make(chan context.Context),
		readyCh:       make(chan struct{}),
		pauseHookFn:   defaultPauseHookFn,
		errorHandleFn: defaultErrorHandleFn,
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
	go func() {
		for ctx := range r.startCh {
			if r.kafkaReader != nil {
				fmt.Println("york already create")
				r.kafkaReader.Close()
			} else {
				fmt.Println("york first create")
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

				r.RangeAllObservers(func(_ mq.Observer, observer mq.Observer) bool {
					if err := observer.NotifyWithManualCommit(m.Value, func() error {
						if err := r.kafkaReader.CommitMessages(ctx, m); err != nil {
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
