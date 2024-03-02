package memory

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
)

var assetNameToIDMap = map[string]int{
	"USD": 1,
	"BTC": 0,
}

type assetRepo struct {
	usersAssetsMap map[int]map[int]*domain.UserAsset
	assetMQTopic   mq.MQTopic
	lock           *sync.RWMutex
}

func CreateAssetRepo(assetMQTopic mq.MQTopic) domain.UserAssetRepo {
	return &assetRepo{
		assetMQTopic:   assetMQTopic,
		usersAssetsMap: make(map[int]map[int]*domain.UserAsset),
		lock:           new(sync.RWMutex),
	}
}

func (*assetRepo) GetAssetIDByName(assetName string) (int, error) {
	if val, ok := assetNameToIDMap[assetName]; ok {
		return val, nil
	}
	return 0, errors.New("not found asset id by name")
}

func (a *assetRepo) GetAssets(userID int) (map[int]*domain.UserAsset, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	userAssetsClone := make(map[int]*domain.UserAsset)
	if userAssets, ok := a.usersAssetsMap[userID]; ok {
		for assetID, asset := range userAssets {
			userAssetsClone[assetID] = &domain.UserAsset{
				UserID:    asset.UserID,
				AssetID:   asset.AssetID,
				Available: asset.Available,
				Frozen:    asset.Frozen,
			}
		}
	}
	return userAssetsClone, nil
}

func (a *assetRepo) GetAsset(userID int, assetID int) (*domain.UserAsset, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	if userAssets, ok := a.usersAssetsMap[userID]; ok {
		if asset, ok := userAssets[assetID]; ok {
			return asset, nil
		}
		return nil, domain.NotFoundUserAssetErr
	}
	return nil, domain.NotFoundUserAssetErr
}

func (a *assetRepo) GetAssetWithInit(userID int, assetID int) (*domain.UserAsset, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if userAssets, ok := a.usersAssetsMap[userID]; ok {
		if asset, ok := userAssets[assetID]; ok {
			return asset, nil
		}
	} else {
		a.usersAssetsMap[userID] = make(map[int]*domain.UserAsset)
	}
	asset := &domain.UserAsset{
		UserID:  userID,
		AssetID: assetID,
	}
	a.usersAssetsMap[userID][assetID] = asset
	return asset, nil
}

func (a *assetRepo) GetUsersAssetsData() (map[int]map[int]*domain.UserAsset, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	usersAssetsClone := make(map[int]map[int]*domain.UserAsset)
	for userID, userAssets := range a.usersAssetsMap {
		userAssetsMap := make(map[int]*domain.UserAsset)
		for assetID, userAsset := range userAssets {
			userAssetsMap[assetID] = &domain.UserAsset{
				UserID:    userAsset.UserID,
				AssetID:   userAsset.AssetID,
				Available: userAsset.Available,
				Frozen:    userAsset.Frozen,
			}
		}
		usersAssetsClone[userID] = userAssetsMap
	}
	return usersAssetsClone, nil
}

func (a *assetRepo) AddAssetAvailable(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Available = userAsset.Available.Add(amount)
	return nil
}

func (a *assetRepo) AddAssetFrozen(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Frozen = userAsset.Frozen.Add(amount)
	return nil
}

func (a *assetRepo) SubAssetAvailable(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Available = userAsset.Available.Sub(amount)
	return nil
}

func (a *assetRepo) SubAssetFrozen(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Frozen = userAsset.Frozen.Sub(amount)
	return nil
}

type mqMessage struct {
	UserAsset *domain.UserAsset
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.UserAsset.UserID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (a *assetRepo) ProduceUserAssetByTradingResult(ctx context.Context, tradingResult *domain.TradingResult) error {
	for _, userAsset := range tradingResult.TransferResult.UserAssets {
		a.assetMQTopic.Produce(ctx, &mqMessage{
			UserAsset: userAsset,
		})
	}
	return nil
}

func (a *assetRepo) ConsumeUserAsset(ctx context.Context, key string, notify func(userAsset *domain.UserAsset) error) { // TODO: maybe need collect
	a.assetMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.UserAsset); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (a *assetRepo) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	usersAssetsMap := make(map[int]map[int]*domain.UserAsset)
	for userID, userAssets := range tradingSnapshot.UsersAssets { //TODO: test user id unmarshal correct?
		userAssetsMap := make(map[int]*domain.UserAsset)
		for assetID, asset := range userAssets {
			userAssetsMap[assetID] = asset
		}
		usersAssetsMap[userID] = userAssetsMap
	}
	a.usersAssetsMap = usersAssetsMap
	return nil
}
