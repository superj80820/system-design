package ormandmq

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type orderNotifyRepo struct {
	orderNotifyMQTopic mq.MQTopic
	observers          utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateOrderNotifyRepo(orderNotifyMQTopic mq.MQTopic) domain.OrderNotifyRepo {
	return &orderNotifyRepo{
		orderNotifyMQTopic: orderNotifyMQTopic,
	}
}

func (o *orderNotifyRepo) ConsumeOrderMQ(ctx context.Context, key string, notify func(sequenceID int, order []*domain.OrderEntity) error) {
	observer := o.orderNotifyMQTopic.SubscribeBatch(key, func(messages [][]byte) error {
		for _, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}

			if err := notify(mqMessage.SequenceID, mqMessage.Orders); err != nil {
				return errors.Wrap(err, "notify failed")
			}
		}
		return nil
	})
	o.observers.Store(key, observer)
}

func (o *orderNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := o.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	o.orderNotifyMQTopic.UnSubscribe(observer)
}
