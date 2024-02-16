package asset

import (
	"context"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type userAsset struct {
	assetRepo   domain.UserAssetRepo
	tradingRepo domain.TradingRepo
}

func CreateUserAssetUseCase(assetRepo domain.UserAssetRepo, tradingRepo domain.TradingRepo) domain.UserAssetUseCase {
	return &userAsset{
		assetRepo:   assetRepo,
		tradingRepo: tradingRepo,
	}
}

func (u *userAsset) LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) error {
	err := u.tryTransfer(ctx, domain.AssetTransferAvailableToAvailable, domain.LiabilityUserID, toUserID, assetID, amount, false)
	if err != nil {
		return errors.Wrap(err, "try transfer failed")
	}
	return nil
}

func (u *userAsset) Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) error {
	err := u.tryTransfer(ctx, domain.AssetTransferAvailableToFrozen, userID, userID, assetID, amount, true)
	if err != nil {
		return errors.Wrap(err, "try transfer failed")
	}
	return nil
}

func (u *userAsset) Transfer(ctx context.Context, transferType domain.AssetTransferEnum, fromUserID, toUserID, assetID int, amount decimal.Decimal) error {
	err := u.tryTransfer(ctx, transferType, fromUserID, toUserID, assetID, amount, true)
	if err != nil {
		return errors.Wrap(err, "try transfer failed")
	}
	return nil
}

func (u *userAsset) Unfreeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) error {
	err := u.tryTransfer(ctx, domain.AssetTransferFrozenToAvailable, userID, userID, assetID, amount, true)
	if err != nil {
		return errors.Wrap(err, "try transfer failed")
	}
	return nil
}

func (u *userAsset) GetAsset(userID int, assetID int) (*domain.UserAsset, error) {
	return u.assetRepo.GetAsset(userID, assetID)
}

func (u *userAsset) GetAssets(userID int) (map[int]*domain.UserAsset, error) {
	return u.assetRepo.GetAssets(userID)
}

func (u *userAsset) tryTransfer(ctx context.Context, assetTransferType domain.AssetTransferEnum, fromUserID, toUserID, assetID int, amount decimal.Decimal, checkBalance bool) error {
	if amount.IsZero() {
		return nil // TODO: maybe error handle out side
	}
	if amount.LessThan(decimal.Zero) {
		return errors.New("can not operate less zero amount")
	}

	fromUserAsset, err := u.assetRepo.GetAsset(fromUserID, assetID)
	if errors.Is(err, domain.NotFoundUserAssetErr) {
		fromUserAsset = u.assetRepo.InitAssets(fromUserID, assetID)
	} else if err != nil {
		return errors.Wrap(err, "get from user asset failed")
	}
	toUserAsset, err := u.assetRepo.GetAsset(toUserID, assetID)
	if errors.Is(err, domain.NotFoundUserAssetErr) {
		toUserAsset = u.assetRepo.InitAssets(toUserID, assetID)
	} else if err != nil {
		return errors.Wrap(err, "get to user asset failed")
	}

	switch assetTransferType {
	case domain.AssetTransferAvailableToAvailable:
		if checkBalance && fromUserAsset.Available.Cmp(amount) < 0 {
			return errors.Wrap(domain.LessAmountErr, "less amount err")
		}
		fromUserAsset.Available = fromUserAsset.Available.Sub(amount)
		toUserAsset.Available = toUserAsset.Available.Add(amount)

		u.assetRepo.ProduceUserAsset(ctx, fromUserID, assetID, fromUserAsset)
		u.assetRepo.ProduceUserAsset(ctx, toUserID, assetID, toUserAsset)

		return nil
	case domain.AssetTransferAvailableToFrozen:
		if checkBalance && fromUserAsset.Available.Cmp(amount) < 0 {
			return errors.Wrap(domain.LessAmountErr, "less amount err")
		}
		fromUserAsset.Available = fromUserAsset.Available.Sub(amount)
		toUserAsset.Frozen = toUserAsset.Frozen.Add(amount)

		u.assetRepo.ProduceUserAsset(ctx, fromUserID, assetID, fromUserAsset)
		if fromUserID != toUserID { // same user no need produce two times
			u.assetRepo.ProduceUserAsset(ctx, toUserID, assetID, toUserAsset)
		}

		return nil
	case domain.AssetTransferFrozenToAvailable:
		if checkBalance && fromUserAsset.Frozen.Cmp(amount) < 0 {
			return errors.Wrap(domain.LessAmountErr, "less amount err")
		}
		fromUserAsset.Frozen = fromUserAsset.Frozen.Sub(amount)
		toUserAsset.Available = toUserAsset.Available.Add(amount)

		u.assetRepo.ProduceUserAsset(ctx, fromUserID, assetID, fromUserAsset)
		u.assetRepo.ProduceUserAsset(ctx, toUserID, assetID, toUserAsset)

		return nil
	default:
		return errors.New("unknown transfer type")
	}
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
