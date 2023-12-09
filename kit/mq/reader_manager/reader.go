package readermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
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

func useMockReaderProvider(fn func(config kafka.ReaderConfig) KafkaReader) readerManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) { r.kafkaReaderProvider = fn })
	}
}

func AddReaderPauseHookFn(fn func()) readerManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) { r.pauseHookFn = fn })
	}
}

type Reader struct {
	kafkaReader KafkaReader

	kafkaReaderProvider func(config kafka.ReaderConfig) KafkaReader

	observers map[*Observer]*Observer

	pauseCh chan context.CancelFunc
	startCh chan context.Context

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
		kafkaReaderProvider: defaultKafkaReaderProvider,
		observers:           make(map[*Observer]*Observer),
		pauseCh:             make(chan context.CancelFunc, 1),
		startCh:             make(chan context.Context),
		pauseHookFn:         defaultPauseHookFn,
		errorHandleFn:       defaultErrorHandleFn,
	}
	for _, option := range options {
		option(r)
	}
	r.kafkaReader = r.kafkaReaderProvider(kafkaReaderConfig)
	return r
}

func (r *Reader) Run() {
	go func() {
		for ctx := range r.startCh {
			// if err := r.kafkaReader.SetOffset(r.kafkaReader.Config().StartOffset); err != nil {//TODO
			// 	fmt.Println("damn", err)
			// 	r.errorHandleFn(err)
			// 	continue
			// }
			for {
				fmt.Println("aa")
				m, err := r.kafkaReader.ReadMessage(ctx)
				fmt.Println("bb", string(m.Value))
				if err != nil {
					go r.pauseHookFn()
					break
				}

				r.RangeAllObservers(func(_ *Observer, observer *Observer) bool {
					fmt.Println("cc", string(m.Value))
					go func() {
						if err := observer.notify(m.Value); err != nil {
							r.errorHandleFn(err)
						}
					}()
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
	return true
}

func (r *Reader) StartConsume(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	select {
	case r.pauseCh <- cancel:
		r.startCh <- ctx
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

func (r *Reader) AddObserver(observer *Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observers[observer]; ok {
		return false
	}
	r.observers[observer] = observer

	return true
}

func (r *Reader) RemoveObserver(observer *Observer) bool {
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

func (r *Reader) RangeAllObservers(fn func(key *Observer, value *Observer) bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for key, value := range r.observers {
		if !fn(key, value) {
			return
		}
	}
}
