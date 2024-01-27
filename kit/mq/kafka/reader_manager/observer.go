package readermanager

import (
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/mq"
)

type observer struct {
	key             string
	notify          mq.NotifyWithManualCommit
	unSubscribeHook unSubscribeHook
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

	setObserverByOptionConfig(observer, options...)

	return observer
}

func CreateObserverWithManualCommit(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := &observer{
		key:    key,
		notify: notify,
	}

	setObserverByOptionConfig(observer, options...)

	return observer
}

func setObserverByOptionConfig(o *observer, options ...mq.ObserverOption) {
	var observerOptionConfig mq.ObserverOptionConfig
	for _, option := range options {
		option(&observerOptionConfig)
	}
	if observerOptionConfig.UnSubscribeHook != nil {
		o.unSubscribeHook = observerOptionConfig.UnSubscribeHook
	}
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

func (o *observer) GetKey() string {
	return o.key
}
