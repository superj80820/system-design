package memoryandmysql

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type assetNotifyRepo struct {
	assetNotifyMQTopic mq.MQTopic
	observers          utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateAssetNotifyRepo(assetNotifyMQTopic mq.MQTopic) domain.UserAssetNotifyRepo {
	return &assetNotifyRepo{
		assetNotifyMQTopic: assetNotifyMQTopic,
	}
}

func (a *assetNotifyRepo) ConsumeUsersAssetsWithRingBuffer(ctx context.Context, key string, notify func(sequenceID int, usersAssets []*domain.UserAsset) error) { // TODO: error handle
	observer := a.assetNotifyMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.SequenceID, mqMessage.UsersAssets); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
	a.observers.Store(key, observer)
}

func (a *assetNotifyRepo) StopConsume(ctx context.Context, key string) {
	observer, ok := a.observers.Load(key)
	if !ok {
		return // TODO: error handle
	}
	a.assetNotifyMQTopic.UnSubscribe(observer)
}
