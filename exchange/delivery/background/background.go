package background

import (
	"context"
	"time"

	"math/rand"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func AsyncBackupSnapshot(
	ctx context.Context,
	tradingUseCase domain.TradingUseCase,
	backupSnapshotDuration time.Duration,
) (readyCh chan struct{}, errCh chan error) {
	readyCh = make(chan struct{})
	errCh = make(chan error)

	go func() {
		tradingSnapshot, err := tradingUseCase.GetHistorySnapshot(ctx)
		if !errors.Is(err, domain.ErrNoData) && err != nil {
			errCh <- errors.Wrap(err, "get history snapshot failed")
			return
		}
		if tradingSnapshot != nil {
			if err := tradingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
				errCh <- errors.Wrap(err, "recover by snapshot failed")
				return
			}
		}
		tradingUseCase.EnableBackupSnapshot(ctx, backupSnapshotDuration)

		close(readyCh)

		<-tradingUseCase.Done()
		if err := tradingUseCase.Err(); err != nil {
			errCh <- errors.Wrap(err, "trading use case get error")
		}
	}()

	return readyCh, errCh
}

func AsyncTradingConsume(
	ctx context.Context,
	quotationUseCase domain.QuotationUseCase,
	candleUseCase domain.CandleUseCase,
	orderUseCase domain.OrderUseCase,
	tradingUseCase domain.TradingUseCase,
	matchingUseCase domain.MatchingUseCase,
) error {
	tradingUseCase.ConsumeGlobalSequencer(ctx)
	orderUseCase.ConsumeOrderResultToSave(ctx, "global-save-order") // TODO: error handle
	quotationUseCase.ConsumeTicksToSave(ctx, "global-save-quotation")
	candleUseCase.ConsumeTradingResultToSave(ctx, "global-save-candle")
	matchingUseCase.ConsumeMatchResultToSave(ctx, "global-save-matching") // TODO: error handle

	select {
	case <-tradingUseCase.Done():
		if err := tradingUseCase.Err(); err != nil {
			return errors.Wrap(err, "trading use case get error")
		}
	case <-candleUseCase.Done():
		if err := candleUseCase.Err(); err != nil {
			return errors.Wrap(err, "candle use case get error")
		}
	case <-quotationUseCase.Done():
		if err := quotationUseCase.Err(); err != nil {
			return errors.Wrap(err, "quotation use case get error")
		}
	}
	return nil
}

func AsyncAutoPreviewTrading(ctx context.Context, email, password string, duration time.Duration, minOrderPrice, maxOrderPrice, minQuantity, maxQuantity float64, accountUseCase domain.AccountUseCase, tradingUseCase domain.TradingUseCase, currencyUseCase domain.CurrencyUseCase) error {
	account, err := accountUseCase.Register(email, password)
	if err != nil {
		return errors.Wrap(err, "register failed")
	}
	userID, err := utilKit.SafeInt64ToInt(account.ID)
	if err != nil {
		return errors.Wrap(err, "safe int64 to int failed")
	}
	if _, err := tradingUseCase.ProduceDepositOrderTradingEvent(ctx, userID, currencyUseCase.GetBaseCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
		return errors.Wrap(err, "produce trading event failed")
	}
	if _, err := tradingUseCase.ProduceDepositOrderTradingEvent(ctx, userID, currencyUseCase.GetQuoteCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
		return errors.Wrap(err, "produce trading event failed")
	}

	errCh := make(chan error)
	doneCh := make(chan struct{})
	ticker := time.NewTicker(duration)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				randomDirection := []domain.DirectionEnum{domain.DirectionBuy, domain.DirectionSell}
				randomPrice := decimal.NewFromFloat((rand.Float64() * minOrderPrice) + maxOrderPrice)
				randomQuantity := decimal.NewFromFloat((rand.Float64() * minQuantity) + maxQuantity)

				if _, err = tradingUseCase.ProduceCreateOrderTradingEvent(ctx, userID, randomDirection[rand.Intn(2)], randomPrice, randomQuantity); err != nil {
					errCh <- errors.Wrap(err, "produce create order trading event failed")
					return
				}
			case <-tradingUseCase.Done():
				close(doneCh)
			}

		}
	}()

	select {
	case err := <-errCh:
		return errors.Wrap(err, "get async error")
	case <-doneCh:
		return nil
	}
}
