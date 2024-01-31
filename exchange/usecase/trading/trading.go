package trading

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type tradingUseCase struct {
	logger             loggerKit.Logger
	userAssetUseCase   domain.UserAssetUseCase
	tradingRepo        domain.TradingRepo
	matchingUseCase    domain.MatchingUseCase
	syncTradingUseCase domain.SyncTradingUseCase
	orderUseCase       domain.OrderUseCase

	historyMatchingDetailsLock     *sync.Mutex
	historyMatchingDetails         []*domain.MatchOrderDetail
	isHistoryMatchingDetailsFullCh chan struct{}

	tradingUseCaseLock *sync.Mutex
	orderBookDepth     int
	cancel             context.CancelFunc
	doneCh             chan struct{}
	err                error
}

func CreateTradingUseCase(
	ctx context.Context,
	tradingRepo domain.TradingRepo,
	orderUseCase domain.OrderUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	syncTradingUseCase domain.SyncTradingUseCase,
	matchingUseCase domain.MatchingUseCase,
	orderBookDepth int,
	logger loggerKit.Logger,
) domain.TradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingUseCase{
		logger:             logger,
		tradingRepo:        tradingRepo,
		orderUseCase:       orderUseCase,
		syncTradingUseCase: syncTradingUseCase,
		matchingUseCase:    matchingUseCase,
		userAssetUseCase:   userAssetUseCase,

		historyMatchingDetailsLock:     new(sync.Mutex),
		isHistoryMatchingDetailsFullCh: make(chan struct{}),

		tradingUseCaseLock: new(sync.Mutex),
		orderBookDepth:     orderBookDepth,
		cancel:             cancel,
		doneCh:             make(chan struct{}),
	}

	t.tradingRepo.SubscribeTradeEvent("global-trader", func(te *domain.TradingEvent) {
		switch te.EventType {
		case domain.TradingEventCreateOrderType:
			t.tradingUseCaseLock.Lock()
			matchResult, err := syncTradingUseCase.CreateOrder(te)
			t.tradingUseCaseLock.Unlock()
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusCreate,
				TradingEvent:        te,
				MatchResult:         matchResult,
			})
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventCancelOrderType:
			t.tradingUseCaseLock.Lock()
			err := syncTradingUseCase.CancelOrder(te)
			t.tradingUseCaseLock.Unlock()
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusCancel,
				TradingEvent:        te,
			})
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventTransferType:
			t.tradingUseCaseLock.Lock()
			err := syncTradingUseCase.Transfer(te)
			t.tradingUseCaseLock.Unlock()
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventDepositType:
			t.tradingUseCaseLock.Lock()
			err := t.Deposit(te)
			t.tradingUseCaseLock.Unlock()
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		default:
			panic(errors.New("unknown event type"))
		}
	})

	go t.collectHistoryMatchingDetailsThenSave(ctx)

	return t
}

func (t *tradingUseCase) CancelOrder(tradingEvent *domain.TradingEvent) error {
	if err := t.syncTradingUseCase.CancelOrder(tradingEvent); err != nil {
		return errors.Wrap(err, "cancel order failed")
	}
	return nil
}

func (t *tradingUseCase) CreateOrder(tradingEvent *domain.TradingEvent) (*domain.MatchResult, error) {
	matchResult, err := t.syncTradingUseCase.CreateOrder(tradingEvent)
	if err != nil {
		return nil, errors.Wrap(err, "create order failed")
	}
	return matchResult, nil
}

func (t *tradingUseCase) Transfer(tradingEvent *domain.TradingEvent) error {
	if err := t.syncTradingUseCase.Transfer(tradingEvent); err != nil {
		return errors.Wrap(err, "transfer failed")
	}
	return nil
}

func (t *tradingUseCase) Deposit(tradingEvent *domain.TradingEvent) error {
	if err := t.syncTradingUseCase.Deposit(tradingEvent); err != nil {
		return errors.Wrap(err, "deposit failed")
	}
	return nil
}

func (t *tradingUseCase) GetHistoryMatchDetails(userID, orderID int) ([]*domain.MatchOrderDetail, error) {
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

	matchOrderDetails, err := t.tradingRepo.GetMatchingDetails(orderID)
	if err != nil {
		return nil, errors.Wrap(err, "get matching details failed")
	}
	return matchOrderDetails, nil
}

func (t *tradingUseCase) ConsumeTradingResult(key string) { // TODO: implement order book in redis
	t.tradingRepo.SubscribeTradingResult(key, func(tradingResult *domain.TradingResult) {
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			return
		}

		var matchOrderDetails []*domain.MatchOrderDetail
		for _, matchDetail := range tradingResult.MatchResult.MatchDetails {
			takerOrderDetail := &domain.MatchOrderDetail{
				SequenceID:     tradingResult.TradingEvent.SequenceID, // TODO: do not use taker sequence?
				OrderID:        matchDetail.TakerOrder.ID,
				CounterOrderID: matchDetail.MakerOrder.ID,
				UserID:         matchDetail.TakerOrder.UserID,
				CounterUserID:  matchDetail.MakerOrder.UserID,
				Direction:      matchDetail.TakerOrder.Direction,
				Price:          matchDetail.Price,
				Quantity:       matchDetail.Quantity,
				Type:           domain.MatchTypeTaker,
				CreatedAt:      tradingResult.TradingEvent.CreatedAt,
			}
			makerOrderDetail := &domain.MatchOrderDetail{
				SequenceID:     tradingResult.TradingEvent.SequenceID, // TODO: do not use maker sequence?
				OrderID:        matchDetail.MakerOrder.ID,
				CounterOrderID: matchDetail.TakerOrder.ID,
				UserID:         matchDetail.MakerOrder.UserID,
				CounterUserID:  matchDetail.TakerOrder.UserID,
				Direction:      matchDetail.MakerOrder.Direction,
				Price:          matchDetail.Price,
				Quantity:       matchDetail.Quantity,
				Type:           domain.MatchTypeMaker,
				CreatedAt:      tradingResult.TradingEvent.CreatedAt,
			}

			matchOrderDetails = append(matchOrderDetails, takerOrderDetail, makerOrderDetail)
		}

		t.historyMatchingDetailsLock.Lock()
		for _, matchOrderDetail := range matchOrderDetails {
			t.historyMatchingDetails = append(t.historyMatchingDetails, matchOrderDetail)
		}
		historyMatchingDetailsLength := len(t.historyMatchingDetails)
		t.historyMatchingDetailsLock.Unlock()
		if historyMatchingDetailsLength >= 1000 {
			t.isHistoryMatchingDetailsFullCh <- struct{}{}
		}
	})
}

func (t *tradingUseCase) GetLatestOrderBook() *domain.OrderBookEntity {
	t.tradingUseCaseLock.Lock()
	defer t.tradingUseCaseLock.Unlock()

	return t.matchingUseCase.GetOrderBook(t.orderBookDepth)
}

func (t *tradingUseCase) GetSequenceID() int {
	return t.syncTradingUseCase.GetSequenceID()
}

func (t *tradingUseCase) GetLatestSnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	sequenceID := t.syncTradingUseCase.GetSequenceID()
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
	if err := t.syncTradingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
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

func (t *tradingUseCase) Shutdown() error {
	t.cancel()
	<-t.doneCh
	return t.err
}

func (t *tradingUseCase) collectHistoryMatchingDetailsThenSave(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	errContinue := errors.New("continue")
	fn := func() error {
		cloneHistoryMatchingDetails, err := func() ([]*domain.MatchOrderDetail, error) {
			t.historyMatchingDetailsLock.Lock()
			defer t.historyMatchingDetailsLock.Unlock()

			if len(t.historyMatchingDetails) == 0 {
				return nil, errContinue
			}
			cloneHistoryMatchingDetails := make([]*domain.MatchOrderDetail, len(t.historyMatchingDetails))
			copy(cloneHistoryMatchingDetails, t.historyMatchingDetails)
			t.historyMatchingDetails = nil

			return cloneHistoryMatchingDetails, nil
		}()
		if err != nil {
			return errors.Wrap(err, "clone history matching details failed")
		}

		if err := t.tradingRepo.SaveMatchingDetailsWithIgnore(ctx, cloneHistoryMatchingDetails); err != nil {
			return errors.Wrap(err, "save matching details failed")
		}

		return nil
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); errors.Is(err, errContinue) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("TODO, error: %+v", err))
			}
		case <-t.isHistoryMatchingDetailsFullCh:
			if err := fn(); errors.Is(err, errContinue) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("TODO, error: %+v", err))
			}
		case <-ctx.Done(): // TODO: test: when shutdown
			if err := fn(); errors.Is(err, errContinue) {
				continue
			} else if err != nil {
				panic(fmt.Sprintf("TODO, error: %+v", err))
			}
		}
	}
}
