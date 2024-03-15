package trading

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
	"golang.org/x/sync/errgroup"
)

type tradingUseCase struct {
	logger                 loggerKit.Logger
	userAssetUseCase       domain.UserAssetUseCase
	userAssetRepo          domain.UserAssetRepo
	tradingRepo            domain.TradingRepo
	sequenceTradingUseCase domain.SequenceTradingUseCase
	candleRepo             domain.CandleRepo
	matchingUseCase        domain.MatchingUseCase
	matchingOrderBookRepo  domain.MatchingOrderBookRepo
	syncTradingUseCase     domain.SyncTradingUseCase
	orderUseCase           domain.OrderUseCase
	quotationRepo          domain.QuotationRepo
	currencyUseCase        domain.CurrencyUseCase
	matchingRepo           domain.MatchingRepo
	orderRepo              domain.OrderRepo

	lastSequenceID int
	errLock        *sync.Mutex
	doneCh         chan struct{}
	err            error
}

func CreateTradingUseCase(
	ctx context.Context,
	tradingRepo domain.TradingRepo,
	matchingRepo domain.MatchingRepo,
	matchingOrderBookRepo domain.MatchingOrderBookRepo,
	quotationRepo domain.QuotationRepo,
	candleRepo domain.CandleRepo,
	orderRepo domain.OrderRepo,
	userAssetRepo domain.UserAssetRepo,
	sequenceTradingUseCase domain.SequenceTradingUseCase,
	orderUseCase domain.OrderUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	syncTradingUseCase domain.SyncTradingUseCase,
	matchingUseCase domain.MatchingUseCase,
	currencyUseCase domain.CurrencyUseCase,
	logger loggerKit.Logger,
) domain.TradingUseCase {
	return &tradingUseCase{
		logger:                 logger,
		tradingRepo:            tradingRepo,
		matchingRepo:           matchingRepo,
		matchingOrderBookRepo:  matchingOrderBookRepo,
		quotationRepo:          quotationRepo,
		candleRepo:             candleRepo,
		orderRepo:              orderRepo,
		sequenceTradingUseCase: sequenceTradingUseCase,
		userAssetRepo:          userAssetRepo,
		orderUseCase:           orderUseCase,
		syncTradingUseCase:     syncTradingUseCase,
		matchingUseCase:        matchingUseCase,
		userAssetUseCase:       userAssetUseCase,
		currencyUseCase:        currencyUseCase,

		errLock: new(sync.Mutex),
		doneCh:  make(chan struct{}),
	}
}

func (t *tradingUseCase) EnableBackupSnapshot(ctx context.Context, duration time.Duration) {
	setErrAndDone := func(err error) {
		t.errLock.Lock()
		defer t.errLock.Unlock()
		t.err = err
		close(t.doneCh)
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		snapshotSequenceID := t.sequenceTradingUseCase.GetSequenceID()

		for range ticker.C {
			if err := t.sequenceTradingUseCase.Pause(); err != nil {
				setErrAndDone(errors.Wrap(err, "pause failed"))
				return
			}
			sequenceID := t.sequenceTradingUseCase.GetSequenceID()
			if snapshotSequenceID == sequenceID {
				if err := t.sequenceTradingUseCase.Continue(); err != nil {
					setErrAndDone(errors.Wrap(err, "continue failed"))
					return
				}
				continue
			}
			snapshot, err := t.GetLatestSnapshot(ctx)

			if errors.Is(err, domain.ErrNoop) {
				continue
			} else if err != nil {
				setErrAndDone(errors.Wrap(err, "get snapshot failed"))
				return
			}
			if err := t.sequenceTradingUseCase.Continue(); err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			if err = t.SaveSnapshot(ctx, snapshot); !errors.Is(err, domain.ErrDuplicate) && err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			snapshotSequenceID = sequenceID
		}
	}()
}

func (t *tradingUseCase) ProcessTradingEvents(ctx context.Context, tes []*domain.TradingEvent) error {
	err := t.sequenceTradingUseCase.CheckEventSequence(tes[0].SequenceID, t.lastSequenceID)
	if errors.Is(err, domain.ErrMissEvent) {
		fmt.Printf("york wow miss events\nyork wow miss events\nyork wow miss events\nyork wow miss events\nyork wow miss events\nyork wow miss events\n this event id %d", tes[0].SequenceID)
		t.sequenceTradingUseCase.RecoverEvents(t.lastSequenceID, func(tradingEvents []*domain.TradingEvent) error {
			for _, te := range tradingEvents {
				if err := t.processTradingEvent(ctx, te); err != nil {
					return errors.Wrap(err, "process trading event failed")
				}
			}
			return nil
		})
		return nil
	}
	for _, te := range tes {
		if err := t.processTradingEvent(ctx, te); err != nil {
			return errors.Wrap(err, "process trading event failed")
		}
	}
	return nil
}

func (t *tradingUseCase) processTradingEvent(ctx context.Context, te *domain.TradingEvent) error {
	var tradingResult domain.TradingResult

	t.lastSequenceID = te.SequenceID

	switch te.EventType {
	case domain.TradingEventCreateOrderType:
		matchResult, transferResult, err := t.syncTradingUseCase.CreateOrder(ctx, te)
		if errors.Is(err, domain.LessAmountErr) || errors.Is(err, domain.InvalidAmountErr) {
			t.logger.Info(fmt.Sprintf("%+v", err))
			return nil
		} else if err != nil {
			return errors.Wrap(err, "process message get failed")
		}

		tradingResult = domain.TradingResult{
			SequenceID:          te.SequenceID,
			TradingResultStatus: domain.TradingResultStatusCreate,
			TradingEvent:        te,
			MatchResult:         matchResult,
			TransferResult:      transferResult,
		}
	case domain.TradingEventCancelOrderType:
		cancelOrderResult, transferResult, err := t.syncTradingUseCase.CancelOrder(ctx, te)
		if errors.Is(err, domain.LessAmountErr) || errors.Is(err, domain.ErrNoOrder) {
			t.logger.Info(fmt.Sprintf("%+v", err))
			return nil
		} else if err != nil {
			return errors.Wrap(err, "process message get failed")
		}

		tradingResult = domain.TradingResult{
			SequenceID:          te.SequenceID,
			TradingResultStatus: domain.TradingResultStatusCancel,
			CancelOrderResult:   cancelOrderResult,
			TradingEvent:        te,
			TransferResult:      transferResult,
		}
	case domain.TradingEventTransferType:
		transferResult, err := t.syncTradingUseCase.Transfer(ctx, te)
		if errors.Is(err, domain.LessAmountErr) {
			t.logger.Info(fmt.Sprintf("%+v", err))
			return nil
		} else if err != nil {
			return errors.Wrap(err, "process message get failed")
		}

		tradingResult = domain.TradingResult{
			SequenceID:          te.SequenceID,
			TradingResultStatus: domain.TradingResultStatusTransfer,
			TradingEvent:        te,
			TransferResult:      transferResult,
		}
	case domain.TradingEventDepositType:
		transferResult, err := t.syncTradingUseCase.Deposit(ctx, te)
		if err != nil {
			return errors.Wrap(err, "process message get failed")
		}

		tradingResult = domain.TradingResult{
			SequenceID:          te.SequenceID,
			TradingResultStatus: domain.TradingResultStatusDeposit,
			TradingEvent:        te,
			TransferResult:      transferResult,
		}
	default:
		return errors.New("unknown event type")
	}

	fmt.Println("york sequence:", tradingResult.SequenceID)

	if err := t.tradingRepo.ProduceTradingResult(ctx, &tradingResult); err != nil {
		panic(errors.Wrap(err, "produce trading result failed"))
	}

	return nil
}

func (t *tradingUseCase) ConsumeGlobalSequencer(ctx context.Context) {
	setErrAndDone := func(err error) {
		t.errLock.Lock()
		defer t.errLock.Unlock()
		t.err = err
		close(t.doneCh)
	}

	t.sequenceTradingUseCase.ConsumeSequenceMessages(func(events []*domain.TradingEvent, commitFn func() error) {
		events, err := t.sequenceTradingUseCase.SaveWithFilterEvents(events, commitFn)
		if err != nil {
			setErrAndDone(errors.Wrap(err, fmt.Sprintf("save with filter events failed, events length: %d", len(events))))
			return
		}
		if err := t.tradingRepo.ProduceTradingEvents(ctx, events); err != nil {
			setErrAndDone(errors.Wrap(err, "produce trading event failed"))
			return
		}
	})
}

func (t *tradingUseCase) ConsumeTradingResult(ctx context.Context, key string) {
	t.tradingRepo.ConsumeTradingResult(ctx, key, func(tradingResults []*domain.TradingResult) error {
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			if err := t.userAssetRepo.ProduceUserAssetByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce order failed")
			}
			return nil
		})

		eg.Go(func() error {
			if err := t.orderRepo.ProduceOrderMQByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce order failed")
			}
			return nil
		})

		eg.Go(func() error {
			if err := t.matchingRepo.ProduceMatchOrderMQByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce match order failed")
			}
			return nil
		})

		eg.Go(func() error {
			if err := t.candleRepo.ProduceCandleMQByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce candle failed")
			}
			return nil
		})

		eg.Go(func() error {
			if err := t.quotationRepo.ProduceTicksMQByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce ticks failed")
			}
			return nil
		})

		if err := eg.Wait(); err != nil {
			panic(errors.Wrap(err, "produce failed"))
		}

		return nil
	})
}

func (t *tradingUseCase) ConsumeTradingEvent(ctx context.Context, key string) {
	setErrAndDone := func(err error) {
		t.errLock.Lock()
		defer t.errLock.Unlock()
		t.err = err
		close(t.doneCh)
	}

	t.tradingRepo.ConsumeTradingEvent(ctx, key, func(events []*domain.TradingEvent, commitFn func() error) {
		if err := t.ProcessTradingEvents(ctx, events); err != nil {
			setErrAndDone(errors.Wrap(err, "process trading events failed"))
			return
		}
		if err := commitFn(); err != nil {
			setErrAndDone(errors.Wrap(err, "commit failed"))
			return
		}
	})

}

func (t *tradingUseCase) ProduceCancelOrderTradingEvent(ctx context.Context, userID, orderID int) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventCancelOrderType,
		OrderCancelEvent: &domain.OrderCancelEvent{
			UserID:  userID,
			OrderId: orderID,
		},
		CreatedAt: time.Now(),
	}

	if err := t.sequenceTradingUseCase.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingUseCase) ProduceCreateOrderTradingEvent(ctx context.Context, userID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}
	orderID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}
	if price.LessThanOrEqual(decimal.Zero) {
		return nil, errors.Wrap(err, "amount is less then or equal zero failed")
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, errors.Wrap(err, "quantity is less then or equal zero failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventCreateOrderType,
		OrderRequestEvent: &domain.OrderRequestEvent{
			UserID:    userID,
			OrderID:   orderID,
			Direction: direction,
			Price:     price,
			Quantity:  quantity,
		},
		CreatedAt: time.Now(),
	}

	if err := t.sequenceTradingUseCase.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingUseCase) ProduceDepositOrderTradingEvent(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventDepositType,
		DepositEvent: &domain.DepositEvent{
			ToUserID: userID,
			AssetID:  assetID,
			Amount:   amount,
		},
		CreatedAt: time.Now(),
	}

	if err := t.sequenceTradingUseCase.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingUseCase) GetHistoryMatchDetails(maxResults int) ([]*domain.MatchOrderDetail, error) {
	details, err := t.matchingRepo.GetMatchingHistory(maxResults)
	if err != nil {
		return nil, errors.Wrap(err, "get matching history failed")
	}
	return details, nil
}

func (t *tradingUseCase) GetUserHistoryMatchDetails(userID, orderID int) ([]*domain.MatchOrderDetail, error) {
	order, err := t.orderUseCase.GetOrder(orderID)
	if errors.Is(err, domain.ErrNoOrder) {
		_, err := t.orderUseCase.GetHistoryOrder(userID, orderID)
		if errors.Is(err, domain.ErrNoOrder) {
			return nil, errors.New("order not found")
		} else if err != nil {
			return nil, errors.Wrap(err, "get order failed")
		}
	} else if err != nil {
		return nil, errors.Wrap(err, "get order failed")
	} else {
		if userID != order.UserID {
			return nil, errors.New("order not found")
		}
	}

	matchOrderDetails, err := t.matchingRepo.GetMatchingDetails(orderID)
	if err != nil {
		return nil, errors.Wrap(err, "get matching details failed")
	}
	return matchOrderDetails, nil
}

func (t *tradingUseCase) GetLatestSnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	sequenceID := t.lastSequenceID
	usersAssetsData, err := t.userAssetUseCase.GetUsersAssetsData()
	if err != nil {
		return nil, errors.Wrap(err, "get all users assets failed")
	}
	ordersData, err := t.orderUseCase.GetOrdersData()
	if err != nil {
		return nil, errors.Wrap(err, "get all orders failed")
	}
	matchesData, err := t.matchingUseCase.GetMatchesData()
	if err != nil {
		return nil, errors.Wrap(err, "get all matches failed")
	}
	return &domain.TradingSnapshot{
		SequenceID:  sequenceID,
		UsersAssets: usersAssetsData,
		Orders:      ordersData,
		MatchData:   matchesData,
	}, nil
}

func (t *tradingUseCase) GetHistorySnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	historySnapshot, err := t.tradingRepo.GetHistorySnapshot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get history snapshot failed")
	}
	return historySnapshot, nil
}

func (t *tradingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	if err := t.userAssetUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	if err := t.orderUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	if err := t.matchingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	t.lastSequenceID = tradingSnapshot.SequenceID
	return nil
}

func (t *tradingUseCase) SaveSnapshot(ctx context.Context, tradingSnapshot *domain.TradingSnapshot) error {
	if err := t.tradingRepo.SaveSnapshot(
		ctx,
		tradingSnapshot.SequenceID,
		tradingSnapshot.UsersAssets,
		tradingSnapshot.Orders,
		tradingSnapshot.MatchData,
	); err != nil {
		return errors.Wrap(err, "save snapshot failed")
	}
	return nil
}

func (t *tradingUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingUseCase) Err() error {
	t.errLock.Lock()
	defer t.errLock.Unlock()
	return t.err
}

func (t *tradingUseCase) Shutdown() error {
	panic("need ctx implement")
	<-t.doneCh
	return t.err
}
