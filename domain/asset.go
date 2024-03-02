package domain

import (
	"context"

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
	LessAmountErr         = errors.New("less amount error")
)

type UserAsset struct {
	UserID    int
	AssetID   int
	Available decimal.Decimal
	Frozen    decimal.Decimal
}

type UserAssetRepo interface {
	GetAssetIDByName(assetName string) (int, error)
	GetAssets(userID int) (map[int]*UserAsset, error)
	GetAsset(userID, assetID int) (*UserAsset, error)
	InitAssets(userID, assetID int) *UserAsset
	GetUsersAssetsData() (map[int]map[int]*UserAsset, error)
	RecoverBySnapshot(*TradingSnapshot) error

	ProduceUserAssetByTradingResult(ctx context.Context, tradingResult *TradingResult) error
	ConsumeUserAsset(ctx context.Context, key string, notify func(userAsset *UserAsset) error)
}

type UserAssetUseCase interface {
	LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	Transfer(ctx context.Context, transferType AssetTransferEnum, fromUserID, toUserID int, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Unfreeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	GetAssets(userID int) (map[int]*UserAsset, error)
	GetAsset(userID, assetID int) (*UserAsset, error)

	GetUsersAssetsData() (map[int]map[int]*UserAsset, error)
	RecoverBySnapshot(*TradingSnapshot) error
}

type TransferResult struct {
	UserAssets []*UserAsset
}
