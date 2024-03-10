package memoryandmysql

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

var assetNameToIDMap = map[string]int{
	"USD": 1,
	"BTC": 0,
}

type assetRepo struct {
	usersAssetsMap map[int]map[int]*domain.UserAsset
	assetMQTopic   mq.MQTopic
	db             *ormKit.DB
	tableName      string
	lock           *sync.RWMutex
}

func CreateAssetRepo(assetMQTopic mq.MQTopic, db *ormKit.DB) domain.UserAssetRepo {
	return &assetRepo{
		assetMQTopic:   assetMQTopic,
		db:             db,
		tableName:      "assets",
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
				ID:        asset.ID,
				UserID:    asset.UserID,
				AssetID:   asset.AssetID,
				Available: asset.Available,
				Frozen:    asset.Frozen,
				CreatedAt: asset.CreatedAt,
				UpdatedAt: asset.UpdatedAt,
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

	timeNow := time.Now()
	userAssetIDInt64 := utilKit.GetSnowflakeIDInt64()
	userAssetID, err := utilKit.SafeInt64ToInt(userAssetIDInt64)
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	asset := &domain.UserAsset{
		ID:        userAssetID,
		UserID:    userID,
		AssetID:   assetID,
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
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
				ID:        userAsset.ID,
				UserID:    userAsset.UserID,
				AssetID:   userAsset.AssetID,
				Available: userAsset.Available,
				Frozen:    userAsset.Frozen,
				CreatedAt: userAsset.CreatedAt,
				UpdatedAt: userAsset.UpdatedAt,
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
	userAsset.UpdatedAt = time.Now()

	return nil
}

func (a *assetRepo) AddAssetFrozen(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Frozen = userAsset.Frozen.Add(amount)
	userAsset.UpdatedAt = time.Now()

	return nil
}

func (a *assetRepo) SubAssetAvailable(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Available = userAsset.Available.Sub(amount)
	userAsset.UpdatedAt = time.Now()

	return nil
}

func (a *assetRepo) SubAssetFrozen(userAsset *domain.UserAsset, amount decimal.Decimal) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	userAsset.Frozen = userAsset.Frozen.Sub(amount)
	userAsset.UpdatedAt = time.Now()

	return nil
}

type mqMessage struct {
	SequenceID  int
	UsersAssets []*domain.UserAsset
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.SequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (a *assetRepo) ProduceUserAssetByTradingResult(ctx context.Context, tradingResult *domain.TradingResult) error {
	if err := a.assetMQTopic.Produce(ctx, &mqMessage{
		SequenceID:  tradingResult.SequenceID,
		UsersAssets: tradingResult.TransferResult.UserAssets,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (a *assetRepo) ConsumeUsersAssets(ctx context.Context, key string, notify func(sequenceID int, usersAssets []*domain.UserAsset) error) { // TODO: error handle
	a.assetMQTopic.Subscribe(key, func(message []byte) error {
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

func (a *assetRepo) GetHistoryAssets(userID int) (map[int]*domain.UserAsset, error) {
	var userAssets []*domain.UserAsset
	if err := a.db.Table(a.tableName).Where("user_id = ?", userID).Find(&userAssets).Error; err != nil {
		return nil, errors.Wrap(err, "get history user assets failed")
	}
	userAssetMap := make(map[int]*domain.UserAsset, len(userAssets))
	for idx := range userAssets {
		userAssetMap[userAssets[idx].AssetID] = userAssets[idx]
	}
	return userAssetMap, nil
}

func (a *assetRepo) SaveAssets(sequenceID int, usersAssets []*domain.UserAsset) error {
	builder := sq.
		Insert(a.tableName).
		Columns("id", "user_id", "asset_id", "available", "frozen", "sequence_id", "updated_at", "created_at").
		Suffix("ON DUPLICATE KEY UPDATE "+
			"`available` = CASE WHEN `sequence_id` < ? THEN VALUES(`available`) ELSE `available` END,"+
			"`frozen` = CASE WHEN `sequence_id` < ? THEN VALUES(`frozen`) ELSE `frozen` END,"+
			"`updated_at` = CASE WHEN `sequence_id` < ? THEN VALUES(`updated_at`) ELSE `updated_at` END,"+
			"`sequence_id` = CASE WHEN `sequence_id` < ? THEN VALUES(`sequence_id`) ELSE `sequence_id` END",
			sequenceID,
			sequenceID,
			sequenceID,
			sequenceID,
		)
	for _, userAsset := range usersAssets {
		builder = builder.Values(userAsset.ID, userAsset.UserID, userAsset.AssetID, userAsset.Available, userAsset.Frozen, sequenceID, userAsset.UpdatedAt, userAsset.CreatedAt)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}

	if err := a.db.Exec(sql, args...).Error; err != nil {
		return errors.Wrap(err, "save users assets failed")
	}

	return nil
}
