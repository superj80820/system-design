package trading

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	"github.com/superj80820/system-design/kit/util"
)

type tradingAsyncUseCase struct {
	logger          loggerKit.Logger
	tradingRepo     domain.TradingRepo
	tradingUseCase  domain.TradingUseCase
	matchingUseCase domain.MatchingUseCase

	tradingUseCaseLock *sync.Mutex
	subscriber         util.GenericSyncMap[*func(tradingResult *domain.TradingResult), func(tradingResult *domain.TradingResult)]
	orderBookDepth     int
	cancel             context.CancelFunc
	doneCh             chan struct{}
	err                error
}

func CreateAsyncTradingUseCase(
	ctx context.Context,
	tradingRepo domain.TradingRepo,
	tradingUseCase domain.TradingUseCase,
	matchingUseCase domain.MatchingUseCase,
	orderBookDepth int,
	logger loggerKit.Logger,
) domain.AsyncTradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingAsyncUseCase{
		logger:          logger,
		tradingRepo:     tradingRepo,
		tradingUseCase:  tradingUseCase,
		matchingUseCase: matchingUseCase,

		tradingUseCaseLock: new(sync.Mutex),
		orderBookDepth:     orderBookDepth,
		cancel:             cancel,
		doneCh:             make(chan struct{}),
	}

	subscribeErrHandleFn := func(err error) error {
		if errors.Is(err, domain.LessAmountErr) {
			t.logger.Info(fmt.Sprintf("%+v", err))
		} else if err != nil {
			panic(fmt.Sprintf("process message get error: %+v", err))
		}
		return nil
	}
	t.tradingRepo.SubscribeTradeMessage(func(te *domain.TradingEvent) {
		switch te.EventType {
		case domain.TradingEventCreateOrderType:
			t.tradingUseCaseLock.Lock()
			matchResult, err := tradingUseCase.CreateOrder(te)
			t.tradingUseCaseLock.Unlock()
			subscribeErrHandleFn(err)

			t.subscriber.Range(func(_ *func(tradingResult *domain.TradingResult), value func(tradingResult *domain.TradingResult)) bool {
				value(&domain.TradingResult{
					TradingResultStatus: domain.TradingResultStatusCreate,
					TradingEvent:        te,
					MatchResult:         matchResult,
				})
				return true
			})
		case domain.TradingEventCancelOrderType:
			t.tradingUseCaseLock.Lock()
			err := tradingUseCase.CancelOrder(te)
			t.tradingUseCaseLock.Unlock()
			subscribeErrHandleFn(err)

			t.subscriber.Range(func(_ *func(tradingResult *domain.TradingResult), value func(tradingResult *domain.TradingResult)) bool {
				value(&domain.TradingResult{
					TradingResultStatus: domain.TradingResultStatusCancel,
					TradingEvent:        te,
				})
				return true
			})
		case domain.TradingEventTransferType:
			t.tradingUseCaseLock.Lock()
			err := tradingUseCase.Transfer(te)
			t.tradingUseCaseLock.Unlock()
			subscribeErrHandleFn(err)
		default:
			subscribeErrHandleFn(errors.New("unknown event type"))
		}
	})

	return t
}

func (t *tradingAsyncUseCase) SubscribeTradingResult(fn func(tradingResult *domain.TradingResult)) {
	t.subscriber.Store(&fn, fn)
}

func (t *tradingAsyncUseCase) GetLatestOrderBook() *domain.OrderBookEntity {
	t.tradingUseCaseLock.Lock()
	defer t.tradingUseCaseLock.Unlock()

	return t.matchingUseCase.GetOrderBook(t.orderBookDepth)
}

// func (t *tradingAsyncUseCase) AsyncEventProcess(ctx context.Context) error {

// 	<-t.tradingRepo.Done()

// 	if err := t.tradingRepo.Err(); err != nil {
// 		return errors.Wrap(err, "trading subscribe get error")
// 	}

// 	return nil
// }

// func (t *tradingAsyncUseCase) AsyncDBProcess(ctx context.Context) error {
// 	var matchResults []*domain.MatchResult
// 	lock := new(sync.Mutex)

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			lock.Lock()
// 			matchResultsClone := make([]*domain.MatchResult, len(matchResults))
// 			copy(matchResultsClone, matchResults)
// 			matchResults = nil
// 			lock.Unlock()

// 			for _, matchDetail := range matchResultsClone {
// 				t.tradingCacheUseCase.SaveOrderHistory(matchDetail)
// 			}
// 		}
// 	}()

// 	for {
// 		select {
// 		case matchResult := <-t.saveOrderCh:
// 			lock.Lock()
// 			matchResults = append(matchResults, matchResult)
// 			lock.Unlock()
// 		case <-ctx.Done():
// 			return nil
// 		}
// 	}
// }

// func (t *tradingAsyncUseCase) AsyncTickProcess(ctx context.Context) error {
// 	var matchResults []*domain.MatchResult
// 	lock := new(sync.Mutex)

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			lock.Lock()
// 			matchResultsClone := make([]*domain.MatchResult, len(matchResults))
// 			copy(matchResultsClone, matchResults)
// 			matchResults = nil
// 			lock.Unlock()

// 			for _, matchDetail := range matchResultsClone {
// 				t.tradingCacheUseCase.SendTick(matchDetail)
// 			}
// 		}
// 	}()

// 	for {
// 		select {
// 		case matchResult := <-t.tickCh:
// 			lock.Lock()
// 			matchResults = append(matchResults, matchResult)
// 			lock.Unlock()
// 		case <-ctx.Done():
// 			return nil
// 		}
// 	}
// }

// func (t *tradingAsyncUseCase) AsyncNotifyProcess(ctx context.Context) error {
// 	var matchResults []*domain.MatchResult
// 	lock := new(sync.Mutex)

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			lock.Lock()
// 			matchResultsClone := make([]*domain.MatchResult, len(matchResults))
// 			copy(matchResultsClone, matchResults)
// 			matchResults = nil
// 			lock.Unlock()

// 			for _, matchDetail := range matchResultsClone {
// 				t.tradingCacheUseCase.SendNotification(matchDetail)
// 			}
// 		}
// 	}()

// 	for {
// 		select {
// 		case matchResult := <-t.notifyCh:
// 			lock.Lock()
// 			matchResults = append(matchResults, matchResult)
// 			lock.Unlock()
// 		case <-ctx.Done():
// 			return nil
// 		}
// 	}
// }

// func (t *tradingAsyncUseCase) AsyncOrderBookProcess(ctx context.Context) error {
// 	var isOrderBookChange bool
// 	lock := new(sync.Mutex)

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			lock.Lock()
// 			isOrderBookChangeClone := isOrderBookChange
// 			isOrderBookChange = false
// 			lock.Unlock()

// 			if !isOrderBookChangeClone {
// 				continue
// 			}

// 			t.orderBookProcessGetLatestOrderBookCh <- true
// 			orderBook := <-t.orderBookProcessReceiveLatestOrderBook

// 			t.tradingCacheUseCase.StoreOrderBookCache(orderBook)
// 		}
// 	}()

// 	for {
// 		select {
// 		case isChange := <-t.isOrderBookChange:
// 			lock.Lock()
// 			isOrderBookChange = isChange
// 			lock.Unlock()
// 		case <-ctx.Done():
// 			return nil
// 		}
// 	}
// }

// func (t *tradingAsyncUseCase) AsyncTradingLogResultProcess(ctx context.Context) error {
// 	var tradingLogResults []*domain.TradingLogResult // TODO: performance?
// 	lock := new(sync.Mutex)

// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
// 		defer ticker.Stop()

// 		for range ticker.C {
// 			lock.Lock()
// 			tradingLogResultsClone := make([]*domain.TradingLogResult, len(tradingLogResults))
// 			copy(tradingLogResultsClone, tradingLogResults)
// 			tradingLogResults = nil
// 			lock.Unlock()

// 			t.tradingCacheUseCase.StoreTradingLogCache(tradingLogResultsClone)
// 		}
// 	}()

// 	for {
// 		select {
// 		case tradingLogResult := <-t.tradingLogResultCh:
// 			lock.Lock()
// 			tradingLogResults = append(tradingLogResults, tradingLogResult)
// 			lock.Unlock()
// 		case <-ctx.Done():
// 			return nil
// 		}
// 	}
// }

func (t *tradingAsyncUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingAsyncUseCase) Shutdown() error {
	t.cancel()
	<-t.doneCh
	return t.err
}

// if len(matchResult.MatchDetails) != 0 {
//     var closedOrders []*domain.OrderEntity
//     if matchResult.TakerOrder.Status.IsFinalStatus() {
//         closedOrders = append(closedOrders, matchResult.TakerOrder)
//     }
//     for _, matchDetail := range matchResult.MatchDetails {
//         maker := matchDetail.MakerOrder
//         if maker.Status.IsFinalStatus() {
//             closedOrders = append(closedOrders, maker)
//         }
//     }
//     for _, closedOrder := range closedOrders {
//         t.orderBookCh <- closedOrder
//     }
// }
