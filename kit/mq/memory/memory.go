package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/mq"
	"github.com/superj80820/system-design/kit/util"
)

type memoryMQ struct {
	observers util.GenericSyncMap[mq.Observer, mq.Observer] // TODO: test key safe?
	messageCh chan []byte
	doneCh    chan struct{}
	cancel    context.CancelFunc
	lock      *sync.Mutex
	err       error
}

var _ mq.MQTopic = (*memoryMQ)(nil)

func CreateMemoryMQ(ctx context.Context, messageChannelBuffer int) mq.MQTopic {
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
				m.observers.Range(func(key, value mq.Observer) bool {
					if err := value.NotifyWithManualCommit(message, func() error {
						return nil
					}); err != nil {
						fmt.Println(fmt.Sprintf("TODO: error: %+v", errors.Wrap(err, "notify failed")))
						return true
					}
					return true
				})
			case <-ctx.Done():
				close(m.doneCh)
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

func (m *memoryMQ) SubscribeWithManualCommit(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := createObserver(key, notify)

	m.observers.Store(observer, observer)

	return observer
}

func (m *memoryMQ) UnSubscribe(observer mq.Observer) {
	m.observers.Delete(observer) // TODO: return success or failure
	observer.UnSubscribeHook()
}

type observer struct {
	key             string
	notify          mq.NotifyWithManualCommit
	unSubscribeHook func() error
}

var _ mq.Observer = (*observer)(nil)

func createObserver(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	o := &observer{
		key:    key,
		notify: notify,
	}

	var observerOptionConfig mq.ObserverOptionConfig
	for _, option := range options {
		option(&observerOptionConfig)
	}
	if observerOptionConfig.UnSubscribeHook != nil {
		o.unSubscribeHook = observerOptionConfig.UnSubscribeHook
	}

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

func (o *observer) UnSubscribeHook() {
	if o.unSubscribeHook == nil {
		return
	}
	o.unSubscribeHook()
}
