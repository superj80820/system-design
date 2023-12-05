package readermanager

type (
	Observer struct {
		key             string
		notify          Notify
		unSubscribeHook unSubscribeHook
	}
	Notify          func(message []byte) error
	unSubscribeHook func() error

	ObserverOption func(*Observer)
)

func defaultUnSubscribeHook() error { return nil }

func CreateObserver(key string, notify Notify, options ...ObserverOption) *Observer {
	observer := &Observer{
		key:             key,
		notify:          notify,
		unSubscribeHook: defaultUnSubscribeHook,
	}

	for _, option := range options {
		option(observer)
	}

	return observer
}

func AddUnSubscribeHook(unSubscribeHook unSubscribeHook) ObserverOption {
	return func(o *Observer) { o.unSubscribeHook = unSubscribeHook }
}

func (o *Observer) GetKey() string {
	return o.key
}
