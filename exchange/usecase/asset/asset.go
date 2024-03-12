package asset

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

func createTransferResult() *transferResult {
	return &transferResult{
		TransferResult: &domain.TransferResult{},
	}
}

type transferResult struct {
	*domain.TransferResult
}

func (t *transferResult) addUserAsset(userAsset *domain.UserAsset) {
	t.TransferResult.UserAssets = append(t.TransferResult.UserAssets, &domain.UserAsset{
		ID:        userAsset.ID,
		UserID:    userAsset.UserID,
		AssetID:   userAsset.AssetID,
		Available: userAsset.Available,
		Frozen:    userAsset.Frozen,
		CreatedAt: userAsset.CreatedAt,
		UpdatedAt: userAsset.UpdatedAt,
	})
}

type userAsset struct {
	assetRepo domain.UserAssetRepo
}

func CreateUserAssetUseCase(assetRepo domain.UserAssetRepo) domain.UserAssetUseCase {
	return &userAsset{
		assetRepo: assetRepo,
	}
}

func (u *userAsset) LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("can not operate less or equal zero amount")
	}

	transferResult := createTransferResult()

	liabilityUserAsset, err := u.assetRepo.GetAssetWithInit(domain.LiabilityUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}
	toUserAsset, err := u.assetRepo.GetAssetWithInit(toUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	u.assetRepo.SubAssetAvailable(liabilityUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(liabilityUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}

func (u *userAsset) TransferFrozenToAvailable(ctx context.Context, fromUserID, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("can not operate less or equal zero amount")
	}

	transferResult := createTransferResult()

	fromUserAsset, err := u.assetRepo.GetAssetWithInit(fromUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}
	toUserAsset, err := u.assetRepo.GetAssetWithInit(toUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	if fromUserAsset.Frozen.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, fmt.Sprintf("less amount err, from user frozen: %s, amount: %s", fromUserAsset.Frozen, amount))
	}
	u.assetRepo.SubAssetFrozen(fromUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(fromUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}

func (u *userAsset) TransferAvailableToAvailable(ctx context.Context, fromUserID, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("can not operate less or equal zero amount")
	}

	transferResult := createTransferResult()

	fromUserAsset, err := u.assetRepo.GetAssetWithInit(fromUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}
	toUserAsset, err := u.assetRepo.GetAssetWithInit(toUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	if fromUserAsset.Available.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetAvailable(fromUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(fromUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}

func (u *userAsset) Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.Wrap(domain.InvalidAmountErr, fmt.Sprintf("can not operate less or equal zero amount, amount: %s", amount.String()))
	}

	transferResult := createTransferResult()

	userAsset, err := u.assetRepo.GetAssetWithInit(userID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	if userAsset.Available.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetAvailable(userAsset, amount)
	u.assetRepo.AddAssetFrozen(userAsset, amount)

	transferResult.addUserAsset(userAsset)

	return transferResult.TransferResult, nil
}

func (u *userAsset) Unfreeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("can not operate less or equal zero amount")
	}

	transferResult := createTransferResult()

	userAsset, err := u.assetRepo.GetAssetWithInit(userID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	if userAsset.Frozen.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetFrozen(userAsset, amount)
	u.assetRepo.AddAssetAvailable(userAsset, amount)

	transferResult.addUserAsset(userAsset)

	return transferResult.TransferResult, nil
}

func (u *userAsset) GetAsset(userID int, assetID int) (*domain.UserAsset, error) {
	userAsset, err := u.assetRepo.GetAsset(userID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset failed")
	}
	return userAsset, nil
}

func (u *userAsset) GetAssets(userID int) (map[int]*domain.UserAsset, error) {
	userAssets, err := u.assetRepo.GetAssets(userID)
	if err != nil {
		return nil, errors.Wrap(err, "get assets failed")
	}
	return userAssets, nil
}

func (u *userAsset) GetUsersAssetsData() (map[int]map[int]*domain.UserAsset, error) {
	usersAssetsData, err := u.assetRepo.GetUsersAssetsData()
	if err != nil {
		return nil, errors.Wrap(err, "get users assets data failed")
	}
	return usersAssetsData, nil
}

func (u *userAsset) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	if err := u.assetRepo.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot")
	}
	return nil
}

func (u *userAsset) ConsumeTransferResultToSave(ctx context.Context, key string) {
	u.assetRepo.ConsumeUsersAssetsWithCommit(ctx, key, func(sequenceID int, usersAssets []*domain.UserAsset, commitFn func() error) error { // TODO: error handle
		if err := u.assetRepo.SaveAssets(sequenceID, usersAssets); err != nil {
			return errors.Wrap(err, "save assets failed")
		}
		if err := commitFn(); err != nil {
			return errors.Wrap(err, "commit failed")
		}
		return nil
	})
}
