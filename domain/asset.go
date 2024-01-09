package domain

import (
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

type AssetTransferEnum int

const LiabilityUserID = 1

const (
	AssetTransferUnknown AssetTransferEnum = iota
	AssetTransferAvailableToAvailable
	AssetTransferAvailableToFrozen
	AssetTransferFrozenToAvailable
)

var (
	NotFoundUserAssetsErr = errors.New("not found user assets")
	NotFoundUserAssetErr  = errors.New("not found user asset")
)

type UserAsset struct {
	Available decimal.Decimal
	Frozen    decimal.Decimal
}

type UserAssetRepo interface {
	GetAssetIDByName(assetName string) (int, error)
	GetAssets(userID int) (map[int]*UserAsset, error)
	GetAsset(userID, assetID int) (*UserAsset, error)
	InitAssets(userID, assetID int) *UserAsset
}

type UserAssetUseCase interface {
	LiabilityUserTransfer(toUserID, assetID int, amount decimal.Decimal) error

	Transfer(transferType AssetTransferEnum, fromUserID, toUserID int, assetID int, amount decimal.Decimal) error
	Freeze(userID, assetID int, amount decimal.Decimal) error
	Unfreeze(userID, assetID int, amount decimal.Decimal) error
}
