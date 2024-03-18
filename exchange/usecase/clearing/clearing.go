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
	// 新增`transferResult`紀錄資產轉換結果
	transferResult := new(domain.TransferResult)

	// 更新taker在訂單模組的狀態
	taker := matchResult.TakerOrder
	if err := c.orderUseCase.UpdateOrder(ctx, taker.ID, taker.UnfilledQuantity, taker.Status, taker.UpdatedAt); err != nil {
		return nil, errors.Wrap(err, "update order failed")
	}
	switch matchResult.TakerOrder.Direction {
	// taker是賣單的情形
	case domain.DirectionSell:
		// 依照`MatchDetails`逐一拿出撮合maker的細節來處理
		for _, matchDetail := range matchResult.MatchDetails {
			// 更新maker在訂單模組的狀態
			maker := matchDetail.MakerOrder
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			matched := matchDetail.Quantity

			// 將taker賣單凍結的數量轉換給maker，數量是matched
			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			// 將maker買單凍結的資產轉換給taker，金額是(maker price)*matched
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			// 如果maker買單數量減至0，則移除訂單
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove failed")
				}
			}
		}
		// 如果taker買單數量減至0，則移除訂單
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(ctx, taker.ID); err != nil {
				return nil, errors.Wrap(err, "remove failed")
			}
		}
	// taker是買單的情形
	case domain.DirectionBuy:
		// 依照`MatchDetails`逐一拿出撮合maker的細節來處理
		for _, matchDetail := range matchResult.MatchDetails {
			// 更新maker在訂單模組的狀態
			maker := matchDetail.MakerOrder
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			matched := matchDetail.Quantity

			// taker買單的價格如果高於maker賣單，則多出來的價格須退還給taker，金額是(taker price-maker price)*matched
			// 退還給taker不需紀錄在`transferResult`，因為後續taker還會從maker拿到資產，taker資產的結果只需紀錄最後一個就可以了
			if taker.Price.Cmp(maker.Price) > 0 {
				unfreezeQuote := taker.Price.Sub(maker.Price).Mul(matched)
				_, err := c.userAssetUseCase.Unfreeze(ctx, taker.UserID, c.quoteCurrencyID, unfreezeQuote)
				if err != nil {
					return nil, errors.Wrap(err, "unfreeze taker failed")
				}
			}
			// 將taker買單凍結的資產轉換給maker，金額是(maker price)*matched
			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			// 將maker賣單凍結的資產轉換給taker，數量是matched
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			// 如果maker買單數量減至0，則移除訂單
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove maker order failed, maker order id: "+strconv.Itoa(maker.ID))
				}
			}
		}
		// 如果taker買單數量減至0，則移除訂單
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
