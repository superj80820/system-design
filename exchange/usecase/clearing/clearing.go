package clearing

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type clearingUseCase struct {
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase
	baseCurrencyID   int
	quoteCurrencyID  int
}

func CreateClearingUseCase(
	userAssetUseCase domain.UserAssetUseCase,
	orderUseCase domain.OrderUseCase,
) domain.ClearingUseCase {
	return &clearingUseCase{
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		baseCurrencyID:   int(domain.BaseCurrencyType),
		quoteCurrencyID:  int(domain.QuoteCurrencyType),
	}
}

func (c *clearingUseCase) ClearMatchResult(ctx context.Context, matchResult *domain.MatchResult) (*domain.TransferResult, error) {
	transferResult := new(domain.TransferResult)

	taker := matchResult.TakerOrder
	switch matchResult.TakerOrder.Direction {
	case domain.DirectionSell:
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			matched := matchDetail.Quantity

			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove failed")
				}
			}
		}
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(ctx, taker.ID); err != nil {
				return nil, errors.Wrap(err, "remove failed")
			}
		}
	case domain.DirectionBuy:
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			matched := matchDetail.Quantity

			if taker.Price.Cmp(maker.Price) > 0 {
				unfreezeQuote := taker.Price.Sub(maker.Price).Mul(matched)
				_, err := c.userAssetUseCase.Unfreeze(ctx, taker.UserID, c.quoteCurrencyID, unfreezeQuote)
				if err != nil {
					return nil, errors.Wrap(err, "unfreeze taker failed")
				}
			}
			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove maker order failed, maker order id: "+strconv.Itoa(maker.ID))
				}
			}
		}
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(ctx, taker.ID); err != nil {
				return nil, errors.Wrap(err, "remove taker order failed")
			}
		}
	default:
		return nil, errors.New("unknown direction")
	}

	return transferResult, nil
}

func (c *clearingUseCase) ClearCancelOrder(ctx context.Context, order *domain.OrderEntity) (*domain.TransferResult, error) {
	var err error
	transferResult := new(domain.TransferResult)
	switch order.Direction {
	case domain.DirectionSell:
		transferResult, err = c.userAssetUseCase.Unfreeze(ctx, order.UserID, c.baseCurrencyID, order.UnfilledQuantity)
		if err != nil {
			return nil, errors.Wrap(err, "unfreeze sell order failed")
		}
	case domain.DirectionBuy:
		transferResult, err = c.userAssetUseCase.Unfreeze(ctx, order.UserID, c.quoteCurrencyID, order.Price.Mul(order.UnfilledQuantity))
		if err != nil {
			return nil, errors.Wrap(err, "unfreeze buy order failed")
		}
	default:
		return nil, errors.New("unknown direction")
	}

	if err := c.orderUseCase.RemoveOrder(ctx, order.ID); err != nil {
		return nil, errors.Wrap(err, "remove failed")
	}

	return transferResult, nil
}
