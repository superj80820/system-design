package candle

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type candleNotifyRepo struct {
	candleNotifyMQTopic mq.MQTopic
	observers           utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateCandleNotifyRepo(candleNotifyMQTopic mq.MQTopic) domain.CandleNotifyRepo {
	return &candleNotifyRepo{
		candleNotifyMQTopic: candleNotifyMQTopic,
	}
}

func (c *candleNotifyRepo) ConsumeCandleMQ(ctx context.Context, key string, notify func(candleBar *domain.CandleBar) error) {
	observer := c.candleNotifyMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqCandleMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.CandleBar); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
	c.observers.Store(key, observer)
}

func (c *candleNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := c.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	c.candleNotifyMQTopic.UnSubscribe(observer)
}
