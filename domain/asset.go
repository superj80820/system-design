package domain

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

const LiabilityUserID = 1

var (
	NotFoundUserAssetsErr = errors.New("not found user assets")
	NotFoundUserAssetErr  = errors.New("not found user asset")
	LessAmountErr         = errors.New("less amount error")
	InvalidAmountErr      = errors.New("invalid amount error")
)

type UserAsset struct {
	ID        int
	UserID    int
	AssetID   int
	Available decimal.Decimal
	Frozen    decimal.Decimal
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserAssetRepo interface {
	GetAssetIDByName(assetName string) (int, error)
	GetAssets(userID int) (map[int]*UserAsset, error)
	GetAsset(userID, assetID int) (*UserAsset, error)
	GetAssetWithInit(userID, assetID int) (*UserAsset, error)
	AddAssetAvailable(userAsset *UserAsset, amount decimal.Decimal) error
	SubAssetAvailable(userAsset *UserAsset, amount decimal.Decimal) error
	AddAssetFrozen(userAsset *UserAsset, amount decimal.Decimal) error
	SubAssetFrozen(userAsset *UserAsset, amount decimal.Decimal) error
	GetUsersAssetsData() (map[int]map[int]*UserAsset, error)
	RecoverBySnapshot(*TradingSnapshot) error

	ProduceUserAssetByTradingResult(ctx context.Context, tradingResult *TradingResult) error
	ConsumeUsersAssets(ctx context.Context, key string, notify func(sequenceID int, usersAssets []*UserAsset) error)

	SaveAssets(sequenceID int, usersAssets []*UserAsset) error
	GetHistoryAssets(userID int) (userAssets map[int]*UserAsset, err error)
}

type UserAssetUseCase interface {
	LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	TransferFrozenToAvailable(ctx context.Context, fromUserID, toUserID int, assetID int, amount decimal.Decimal) (*TransferResult, error)
	TransferAvailableToAvailable(ctx context.Context, fromUserID, toUserID int, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Unfreeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	GetAssets(userID int) (map[int]*UserAsset, error)
	GetAsset(userID, assetID int) (*UserAsset, error)

	ConsumeTransferResultToSave(ctx context.Context, key string)

	GetUsersAssetsData() (map[int]map[int]*UserAsset, error)
	RecoverBySnapshot(*TradingSnapshot) error
}

type TransferResult struct {
	UserAssets []*UserAsset
}
