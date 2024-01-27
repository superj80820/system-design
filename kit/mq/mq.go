package mq

import (
	"context"
)

type Notify func(message []byte) error
type NotifyWithManualCommit func(message []byte, commitFn func() error) error

type Observer interface {
	GetKey() string
	NotifyWithManualCommit(message []byte, commitFn func() error) error
	UnSubscribeHook()
}

type Message interface {
	GetKey() string
	Marshal() ([]byte, error)
}

type WriterManager interface {
	WriteMessages(ctx context.Context, msgs ...Message) error
}

type ReaderManager interface {
	AddObserver(observer Observer) bool
	SyncStartConsume(ctx context.Context) bool
	StartConsume(ctx context.Context) bool
	StopConsume() bool
	RemoveObserverWithHook(observer Observer) bool
	IfNoObserversThenStopConsume()
	GetObserversLen() int
	Wait()
}

type ObserverOption func(*ObserverOptionConfig)

type ObserverOptionConfig struct {
	UnSubscribeHook func() error
}

type MQTopic interface {
	Subscribe(key string, notify Notify, options ...ObserverOption) Observer
	SubscribeWithManualCommit(key string, notify NotifyWithManualCommit, options ...ObserverOption) Observer
	UnSubscribe(observer Observer)
	Produce(ctx context.Context, message Message) error
	Done() <-chan struct{}
	Err() error
	Shutdown() bool
}

func AddUnSubscribeHook(unSubscribeHook func() error) ObserverOption {
	return func(oc *ObserverOptionConfig) {
		oc.UnSubscribeHook = unSubscribeHook
	}
}
