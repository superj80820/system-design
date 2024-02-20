package readermanager

import (
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/mq"
)

type observer struct {
	key             string
	notify          mq.NotifyWithManualCommit
	notifyBatch     mq.NotifyBatchWithManualCommit
	unSubscribeHook unSubscribeHook
	errorHandler    func(error)
}

type unSubscribeHook func() error

func CreateObserver(key string, notify mq.Notify, options ...mq.ObserverOption) mq.Observer {
	observer := &observer{
		key: key,
		notify: func(message []byte, commitFn func() error) error {
			if err := notify(message); err != nil {
				return errors.Wrap(err, "notify failed")
			}
			return nil
		},
	}

	applyObserverByOptionConfig(observer, options...)

	return observer
}

func CreateObserverBatch(key string, notifyBatch mq.NotifyBatch, options ...mq.ObserverOption) mq.Observer {
	observer := &observer{
		key: key,
		notifyBatch: func(messages [][]byte, commitFn func() error) error {
			if err := notifyBatch(messages); err != nil {
				return errors.Wrap(err, "notify failed")
			}
			return nil
		},
	}

	applyObserverByOptionConfig(observer, options...)

	return observer
}

func CreateObserverWithManualCommit(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := &observer{
		key:    key,
		notify: notify,
	}

	applyObserverByOptionConfig(observer, options...)

	return observer
}

func CreateObserverBatchWithManualCommit(key string, notifyBatch mq.NotifyBatchWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := &observer{
		key:         key,
		notifyBatch: notifyBatch,
	}

	applyObserverByOptionConfig(observer, options...)

	return observer
}

func applyObserverByOptionConfig(o *observer, options ...mq.ObserverOption) {
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

func (o *observer) GetKey() string {
	return o.key
}

func (o *observer) ErrorHandler(err error) {
	if o.errorHandler != nil {
		o.errorHandler(err)
	}
}
