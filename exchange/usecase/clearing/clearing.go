package clearing

import (
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
	baseCurrencyID,
	quoteCurrencyID int,
) domain.ClearingUseCase {
	return &clearingUseCase{
		userAssetUseCase: userAssetUseCase,
		orderUseCase:     orderUseCase,
		baseCurrencyID:   baseCurrencyID,
		quoteCurrencyID:  quoteCurrencyID,
	}
}

func (c *clearingUseCase) ClearMatchResult(matchResult *domain.MatchResult) error {
	taker := matchResult.TakerOrder
	if len(matchResult.MatchDetails) == 0 {
		return errors.New("match details length is 0")
	}
	switch matchResult.TakerOrder.Direction {
	case domain.DirectionSell:
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			matched := matchDetail.Quantity
			c.userAssetUseCase.Transfer(domain.AssetTransferFrozenToAvailable, taker.ID, maker.ID, c.baseCurrencyID, matched)
			c.userAssetUseCase.Transfer(domain.AssetTransferFrozenToAvailable, maker.ID, taker.ID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if maker.UnfilledQuantity.IsZero() {
				c.orderUseCase.RemoveOrder(maker.ID)
			}
		}
		if taker.UnfilledQuantity.IsZero() {
			c.orderUseCase.RemoveOrder(taker.ID)
		}
		return nil
	case domain.DirectionBuy:
		for _, matchDetail := range matchResult.MatchDetails {
			maker := matchDetail.MakerOrder
			matched := matchDetail.Quantity
			if taker.Price.Cmp(maker.Price) > 0 {
				// TODO: test un freeze quote
				unfreezeQuote := taker.Price.Sub(maker.Price).Mul(matched)
				if err := c.userAssetUseCase.Unfreeze(taker.ID, c.quoteCurrencyID, unfreezeQuote); err != nil {
					return errors.Wrap(err, "unfreeze taker failed")
				}
			}
			c.userAssetUseCase.Transfer(domain.AssetTransferFrozenToAvailable, taker.ID, maker.ID, c.quoteCurrencyID, maker.Price.Mul(matched))
			c.userAssetUseCase.Transfer(domain.AssetTransferFrozenToAvailable, maker.ID, taker.ID, c.baseCurrencyID, matched)
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(maker.ID); err != nil {
					return errors.Wrap(err, "remove maker order failed")
				}
			}
		}
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(taker.ID); err != nil {
				return errors.Wrap(err, "remove taker order failed")
			}
		}
		return nil
	default:
		return errors.New("unknown direction")
	}
}

func (c *clearingUseCase) ClearCancelOrder(order *domain.OrderEntity) error {
	switch order.Direction {
	case domain.DirectionSell:
		if err := c.userAssetUseCase.Unfreeze(order.UserID, c.baseCurrencyID, order.UnfilledQuantity); err != nil {
			return errors.Wrap(err, "unfreeze sell order failed")
		}
		return nil
	case domain.DirectionBuy:
		if err := c.userAssetUseCase.Unfreeze(order.UserID, c.quoteCurrencyID, order.Price.Mul(order.UnfilledQuantity)); err != nil {
			return errors.Wrap(err, "unfreeze buy order failed")
		}
		return nil
	default:
		return errors.New("unknown direction")
	}
}
