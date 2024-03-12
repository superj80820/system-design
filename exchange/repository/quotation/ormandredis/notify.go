package ormandredis

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type quotationNotifyRepo struct {
	tickNotifyMQTopic mq.MQTopic
	observers         utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateQuotationNotifyRepo(tickNotifyMQTopic mq.MQTopic) domain.QuotationNotifyRepo {
	return &quotationNotifyRepo{
		tickNotifyMQTopic: tickNotifyMQTopic,
	}
}

func (q *quotationNotifyRepo) ConsumeTicksMQWithRingBuffer(ctx context.Context, key string, notify func(sequenceID int, ticks []*domain.TickEntity) error) {
	observer := q.tickNotifyMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.SequenceID, mqMessage.Ticks); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
	q.observers.Store(key, observer)
}

func (q *quotationNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := q.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	q.tickNotifyMQTopic.UnSubscribe(observer)
}
