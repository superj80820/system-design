package asset

import (
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type userAsset struct {
	assetRepo domain.UserAssetRepo
}

func CreateUserAssetUseCase(assetRepo domain.UserAssetRepo) domain.UserAssetUseCase {
	return &userAsset{
		assetRepo: assetRepo,
	}
}

func (u *userAsset) LiabilityUserTransfer(toUserID, assetID int, amount decimal.Decimal) error {
	return u.tryTransfer(domain.AssetTransferAvailableToAvailable, domain.LiabilityUserID, toUserID, assetID, amount, false)
}

func (u *userAsset) Freeze(userID, assetID int, amount decimal.Decimal) error {
	return u.tryTransfer(domain.AssetTransferAvailableToFrozen, userID, userID, assetID, amount, true)
}

func (u *userAsset) Transfer(transferType domain.AssetTransferEnum, fromUserID, toUserID, assetID int, amount decimal.Decimal) error {
	return u.tryTransfer(transferType, fromUserID, toUserID, assetID, amount, true)
}

func (u *userAsset) Unfreeze(userID, assetID int, amount decimal.Decimal) error {
	return u.tryTransfer(domain.AssetTransferFrozenToAvailable, userID, userID, assetID, amount, true)
}

func (u *userAsset) GetAsset(userID int, assetID int) (*domain.UserAsset, error) {
	return u.assetRepo.GetAsset(userID, assetID)
}

func (u *userAsset) GetAssets(userID int) (map[int]*domain.UserAsset, error) {
	return u.assetRepo.GetAssets(userID)
}

func (u *userAsset) tryTransfer(assetTransferType domain.AssetTransferEnum, fromUserID, toUserID, assetID int, amount decimal.Decimal, checkBalance bool) error {
	if amount.IsZero() {
		return nil
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
			return domain.LessAmountErr
		}
		fromUserAsset.Available = fromUserAsset.Available.Sub(amount)
		toUserAsset.Available = toUserAsset.Available.Add(amount)
		return nil
	case domain.AssetTransferAvailableToFrozen:
		if checkBalance && fromUserAsset.Available.Cmp(amount) < 0 {
			return domain.LessAmountErr
		}
		fromUserAsset.Available = fromUserAsset.Available.Sub(amount)
		toUserAsset.Frozen = toUserAsset.Frozen.Add(amount)
		return nil
	case domain.AssetTransferFrozenToAvailable:
		if checkBalance && fromUserAsset.Frozen.Cmp(amount) < 0 {
			return domain.LessAmountErr
		}
		fromUserAsset.Frozen = fromUserAsset.Frozen.Sub(amount)
		toUserAsset.Available = toUserAsset.Available.Add(amount)
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
