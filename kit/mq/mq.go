package mq

import (
	"context"
)

type Notify func(message []byte) error
type NotifyBatch func(messages [][]byte) error
type NotifyWithManualCommit func(message []byte, commitFn func() error) error
type NotifyBatchWithManualCommit func(message [][]byte, commitFn func() error) error

type Observer interface {
	GetKey() string
	NotifyWithManualCommit(message []byte, commitFn func() error) error
	NotifyBatchWithManualCommit(messages [][]byte, commitFn func() error) error
	UnSubscribeHook()
	ErrorHandler(error)
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
	AddObserverBatch(observer Observer) bool
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
	ErrorHandler    func(error)
}

type MQTopic interface {
	Subscribe(key string, notify Notify, options ...ObserverOption) Observer
	SubscribeWithManualCommit(key string, notify NotifyWithManualCommit, options ...ObserverOption) Observer
	SubscribeBatch(key string, notifyBatch NotifyBatch, options ...ObserverOption) Observer
	SubscribeBatchWithManualCommit(key string, notifyBatch NotifyBatchWithManualCommit, options ...ObserverOption) Observer
	UnSubscribe(observer Observer)
	Produce(ctx context.Context, message Message) error
	Done() <-chan struct{}
	Err() error
	Shutdown() bool
}

func AddUnSubscribeHook(unSubscribeHook func() error) ObserverOption {
	return func(ooc *ObserverOptionConfig) {
		ooc.UnSubscribeHook = unSubscribeHook
	}
}

func AddErrorHandler(errorHandler func(error)) ObserverOption {
	return func(ooc *ObserverOptionConfig) {
		ooc.ErrorHandler = errorHandler
	}
}
