package memoryandredis

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type matchingOrderBookNotifyRepo struct {
	l2OrderBookNotifyMQTopic mq.MQTopic
	observers                utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateMatchingOrderBookNotifyRepo(l2OrderBookNotifyMQTopic mq.MQTopic) domain.MatchingOrderBookNotifyRepo {
	return &matchingOrderBookNotifyRepo{
		l2OrderBookNotifyMQTopic: l2OrderBookNotifyMQTopic,
	}
}

func (m *matchingOrderBookNotifyRepo) ConsumeL2OrderBookWithRingBuffer(ctx context.Context, key string, notify func(*domain.OrderBookL2Entity) error) {
	observer := m.l2OrderBookNotifyMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqL2OrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookL2Entity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
	m.observers.Store(key, observer)
}

func (m *matchingOrderBookNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := m.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	m.l2OrderBookNotifyMQTopic.UnSubscribe(observer)
}
