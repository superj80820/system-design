package mysqlandmq

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type matchingNotifyRepo struct {
	matchingNotifyMQTopic mq.MQTopic
	observers             utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateMatchingNotifyRepo(matchingNotifyMQTopic mq.MQTopic) domain.MatchingNotifyRepo {
	return &matchingNotifyRepo{
		matchingNotifyMQTopic: matchingNotifyMQTopic,
	}
}

func (m *matchingNotifyRepo) ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*domain.MatchOrderDetail) error) {
	observer := m.matchingNotifyMQTopic.SubscribeBatch(key, func(messages [][]byte) error {
		details := make([]*domain.MatchOrderDetail, len(messages))
		for idx, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			details[idx] = mqMessage.MatchOrderDetail
		}

		if err := notify(details); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
	m.observers.Store(key, observer)
}

func (m *matchingNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := m.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	m.matchingNotifyMQTopic.UnSubscribe(observer)
}
