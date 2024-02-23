package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/mq"
	"github.com/superj80820/system-design/kit/util"
)

type memoryMQ struct {
	observers       util.GenericSyncMap[mq.Observer, mq.Observer] // TODO: test key safe?
	observerBatches util.GenericSyncMap[mq.Observer, mq.Observer] // TODO: test key safe?
	messageCh       chan []byte
	messages        [][]byte
	doneCh          chan struct{}
	cancel          context.CancelFunc
	lock            *sync.Mutex
	err             error
}

var _ mq.MQTopic = (*memoryMQ)(nil)

func CreateMemoryMQ(ctx context.Context, messageChannelBuffer int, messageCollectDuration time.Duration) mq.MQTopic {
	ctx, cancel := context.WithCancel(ctx)

	m := &memoryMQ{
		messageCh: make(chan []byte, messageChannelBuffer),
		doneCh:    make(chan struct{}),
		lock:      new(sync.Mutex),
		cancel:    cancel,
	}

	go func() {
		for {
			select {
			case message := <-m.messageCh:
				m.lock.Lock()
				m.messages = append(m.messages, message)
				m.lock.Unlock()
			case <-ctx.Done():
				close(m.doneCh)
			}
		}
	}()

	ticker := time.NewTicker(messageCollectDuration)
	go func() {
		defer ticker.Stop()

		continueErr := errors.New("continue error")
		for range ticker.C {
			cloneMessages, err := func() ([][]byte, error) {
				m.lock.Lock()
				defer m.lock.Unlock()
				if len(m.messages) == 0 {
					return nil, errors.Wrap(continueErr, "message length is 0")
				}
				cloneMessages := make([][]byte, len(m.messages))
				copy(cloneMessages, m.messages)
				m.messages = nil

				return cloneMessages, nil
			}()
			if errors.Is(err, continueErr) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("clone message get except error, error: %+v", err))
			}

			m.observerBatches.Range(func(key, value mq.Observer) bool {
				if err := value.NotifyBatchWithManualCommit(cloneMessages, func() error {
					return nil
				}); err != nil {
					value.ErrorHandler(err) // handle error then continue
					return true
				}
				return true
			})

			for _, message := range cloneMessages {
				m.observers.Range(func(key, value mq.Observer) bool {
					if err := value.NotifyWithManualCommit(message, func() error {
						return nil
					}); err != nil {
						value.ErrorHandler(err) // handle error then continue
						return true
					}
					return true
				})
			}
		}
	}()

	return m
}

func (m *memoryMQ) Done() <-chan struct{} {
	return m.doneCh
}

func (m *memoryMQ) Err() error {
	return m.err
}

func (m *memoryMQ) Produce(ctx context.Context, message mq.Message) error {
	marshalData, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal failed")
	}

	m.messageCh <- marshalData

	return nil
}

func (m *memoryMQ) Shutdown() bool {
	m.cancel()
	<-m.doneCh
	return true
}

func (m *memoryMQ) Subscribe(key string, notify mq.Notify, options ...mq.ObserverOption) mq.Observer {
	observer := createObserver(key, func(message []byte, commitFn func() error) error {
		if err := notify(message); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})

	m.observers.Store(observer, observer)

	return observer
}

func (m *memoryMQ) SubscribeBatch(key string, notifyBatch mq.NotifyBatch, options ...mq.ObserverOption) mq.Observer {
	observer := createObserverBatch(key, func(messages [][]byte, commitFn func() error) error {
		if err := notifyBatch(messages); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})

	m.observerBatches.Store(observer, observer)

	return observer
}

func (m *memoryMQ) SubscribeWithManualCommit(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := createObserver(key, notify)

	m.observers.Store(observer, observer)

	return observer
}

func (m *memoryMQ) SubscribeBatchWithManualCommit(key string, notifyBatch mq.NotifyBatchWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := createObserverBatch(key, notifyBatch)

	m.observerBatches.Store(observer, observer)

	return observer
}

func (m *memoryMQ) UnSubscribe(observer mq.Observer) {
	m.observers.Delete(observer) // TODO: return success or failure
	observer.UnSubscribeHook()
}

type observer struct {
	key             string
	notify          mq.NotifyWithManualCommit
	notifyBatch     mq.NotifyBatchWithManualCommit
	unSubscribeHook func() error
	errorHandler    func(error)
}

var _ mq.Observer = (*observer)(nil)

func applyObserverOptions(o *observer, options []mq.ObserverOption) {
	var observerOptionConfig mq.ObserverOptionConfig
	for _, option := range options {
		option(&observerOptionConfig)
	}
	if observerOptionConfig.UnSubscribeHook != nil {
		o.unSubscribeHook = observerOptionConfig.UnSubscribeHook
	}
	if observerOptionConfig.ErrorHandler != nil {
		o.errorHandler = observerOptionConfig.ErrorHandler
	}
}

func createObserver(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	o := &observer{
		key:    key,
		notify: notify,
	}

	applyObserverOptions(o, options)

	return o
}

func createObserverBatch(key string, notifyBatch mq.NotifyBatchWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	o := &observer{
		key:         key,
		notifyBatch: notifyBatch,
	}

	applyObserverOptions(o, options)

	return o
}

func (o *observer) GetKey() string {
	return o.key
}

func (o *observer) NotifyWithManualCommit(message []byte, commitFn func() error) error {
	if err := o.notify(message, commitFn); err != nil {
		return errors.Wrap(err, "notify failed")
	}
	return nil
}

func (o *observer) NotifyBatchWithManualCommit(messages [][]byte, commitFn func() error) error {
	if err := o.notifyBatch(messages, commitFn); err != nil {
		return errors.Wrap(err, "notify failed")
	}
	return nil
}

func (o *observer) UnSubscribeHook() {
	if o.unSubscribeHook == nil {
		return
	}
	o.unSubscribeHook()
}

func (o *observer) ErrorHandler(err error) {
	if o.errorHandler != nil {
		o.errorHandler(err)
	}
}
