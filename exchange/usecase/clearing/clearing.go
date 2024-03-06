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
	if err := c.orderUseCase.UpdateOrder(ctx, taker.ID, taker.UnfilledQuantity, taker.Status, taker.UpdatedAt); err != nil {
		return nil, errors.Wrap(err, "update order failed")
	}
	switch matchResult.TakerOrder.Direction {
	case domain.DirectionSell:
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
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
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
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

func (c *clearingUseCase) ClearCancelOrder(ctx context.Context, cancelOrderResult *domain.CancelResult) (*domain.TransferResult, error) {
	var err error
	transferResult := new(domain.TransferResult)
	switch cancelOrderResult.CancelOrder.Direction {
	case domain.DirectionSell:
		transferResult, err = c.userAssetUseCase.Unfreeze(ctx, cancelOrderResult.CancelOrder.UserID, c.baseCurrencyID, cancelOrderResult.CancelOrder.UnfilledQuantity)
		if err != nil {
			return nil, errors.Wrap(err, "unfreeze sell order failed")
		}
	case domain.DirectionBuy:
		transferResult, err = c.userAssetUseCase.Unfreeze(ctx, cancelOrderResult.CancelOrder.UserID, c.quoteCurrencyID, cancelOrderResult.CancelOrder.Price.Mul(cancelOrderResult.CancelOrder.UnfilledQuantity))
		if err != nil {
			return nil, errors.Wrap(err, "unfreeze buy order failed")
		}
	default:
		return nil, errors.New("unknown direction")
	}

	if err := c.orderUseCase.UpdateOrder(ctx, cancelOrderResult.CancelOrder.ID, cancelOrderResult.CancelOrder.UnfilledQuantity, cancelOrderResult.CancelOrder.Status, cancelOrderResult.CreatedAt); err != nil {
		return nil, errors.Wrap(err, "update failed")
	}

	if err := c.orderUseCase.RemoveOrder(ctx, cancelOrderResult.CancelOrder.ID); err != nil {
		return nil, errors.Wrap(err, "remove failed")
	}

	return transferResult, nil
}
