package memory

import (
	"errors"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

var assetNameToIDMap = map[string]int{
	"USD": 1,
	"BTC": 0,
}

type assetRepo struct {
	userAssetsMap util.GenericSyncMap[int, *util.GenericSyncMap[int, *domain.UserAsset]]
}

func CreateAssetRepo() domain.UserAssetRepo {
	return &assetRepo{}
}

func (*assetRepo) GetAssetIDByName(assetName string) (int, error) {
	if val, ok := assetNameToIDMap[assetName]; ok {
		return val, nil
	}
	return 0, errors.New("not found asset id by name")
}

func (a *assetRepo) InitAssets(userID int, assetID int) *domain.UserAsset {
	var userAssets util.GenericSyncMap[int, *domain.UserAsset]
	val, _ := a.userAssetsMap.LoadOrStore(userID, &userAssets)

	var asset domain.UserAsset
	val.Store(assetID, &asset)

	return &asset
}

// TODO: is best way?
func (a *assetRepo) GetAssets(userID int) (map[int]*domain.UserAsset, error) {
	userAssetsClone := make(map[int]*domain.UserAsset)
	if userAssets, ok := a.userAssetsMap.Load(userID); ok {
		userAssets.Range(func(key int, value *domain.UserAsset) bool {
			userAssetsClone[key] = value
			return true
		})
	} else {
		return nil, domain.NotFoundUserAssetsErr
	}
	return userAssetsClone, nil
}

func (a *assetRepo) GetAsset(userID int, assetID int) (*domain.UserAsset, error) {
	if userAssets, ok := a.userAssetsMap.Load(userID); ok {
		if asset, ok := userAssets.Load(assetID); ok {
			return asset, nil
		}
		return nil, domain.NotFoundUserAssetErr
	}
	return nil, domain.NotFoundUserAssetErr
}

func (a *assetRepo) GetUsersAssetsData() (map[int]map[int]*domain.UserAsset, error) {
	usersAssetsClone := make(map[int]map[int]*domain.UserAsset)
	a.userAssetsMap.Range(func(userID int, assetsMap *util.GenericSyncMap[int, *domain.UserAsset]) bool {
		userAssetsMap := make(map[int]*domain.UserAsset)
		assetsMap.Range(func(assetID int, asset *domain.UserAsset) bool {
			userAssetsMap[assetID] = &domain.UserAsset{
				Available: asset.Available,
				Frozen:    asset.Frozen,
			}
			return true
		})
		usersAssetsClone[userID] = userAssetsMap
		return true
	})
	return usersAssetsClone, nil
}

func (a *assetRepo) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	for userID, assetsMap := range tradingSnapshot.UsersAssets { //TODO: test user id unmarshal correct?
		var storeAssetsMap util.GenericSyncMap[int, *domain.UserAsset]
		for assetID, asset := range assetsMap {
			storeAssetsMap.Store(assetID, asset)
		}
		a.userAssetsMap.Store(userID, &storeAssetsMap)
	}
	return nil
}
