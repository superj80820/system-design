package trading

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	"golang.org/x/sync/errgroup"
)

type tradingAsyncUseCase struct {
	tradingUseCase domain.TradingUseCase
	tradingRepo    domain.TradingRepo
	logger         loggerKit.Logger

	orderBookCh chan *domain.OrderEntity
	saveOrderCh chan *domain.OrderEntity

	cancel context.CancelFunc
	doneCh chan struct{}
	err    error
}

func CreateAsyncTradingUseCase(
	ctx context.Context,
	tradingUseCase domain.TradingUseCase,
	tradingRepo domain.TradingRepo,
	logger loggerKit.Logger,
) domain.AsyncTradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingAsyncUseCase{
		tradingUseCase: tradingUseCase,
		tradingRepo:    tradingRepo,
		saveOrderCh:    make(chan *domain.OrderEntity),
		cancel:         cancel,
		doneCh:         make(chan struct{}),
		logger:         logger,
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return t.AsyncEventProcess(ctx)
	})
	eg.Go(func() error {
		return t.AsyncDBProcess(ctx)
	})
	eg.Go(func() error {
		return t.AsyncAPIResultProcess(ctx)
	})
	eg.Go(func() error {
		return t.AsyncNotifyProcess(ctx)
	})
	eg.Go(func() error {
		return t.AsyncOrderBookProcess(ctx)
	})
	eg.Go(func() error {
		return t.AsyncTickProcess(ctx)
	})
	go func() {
		t.err = eg.Wait()
		if err := t.tradingUseCase.Shutdown(); err != nil {
			t.logger.Error(fmt.Sprintf("shutdown trading use case failed, error: %+v", err))
		}
		close(t.doneCh)
	}()

	return t
}

func (t *tradingAsyncUseCase) AsyncEventProcess(ctx context.Context) error {
	t.tradingRepo.SubscribeTradeMessage(func(te *domain.TradingEvent) {
		err := t.tradingUseCase.ProcessMessages(te)
		if errors.Is(err, domain.LessAmountErr) {
			// do nothing
		} else if err != nil {
			panic(fmt.Sprintf("process message get error: %+v", err))
		}
	})
	select {
	case <-t.tradingRepo.Done():
		if err := t.tradingRepo.Err(); err != nil {
			return errors.Wrap(err, "trade subscriber get error")
		}
	case <-ctx.Done():
		t.tradingRepo.Shutdown()
	}
	return nil
}

func (t *tradingAsyncUseCase) AsyncDBProcess(ctx context.Context) error {
	orders := make([]*domain.OrderEntity, 0, 1000)   // TODO: performance?
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	go func() {
		for {
			<-ticker.C
			// TODO
		}
	}()

	for {
		select {
		case order := <-t.saveOrderCh:
			orders = append(orders, order)
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *tradingAsyncUseCase) AsyncTickProcess(ctx context.Context) error {
	return nil
}

func (t *tradingAsyncUseCase) AsyncNotifyProcess(ctx context.Context) error {
	return nil
}

func (t *tradingAsyncUseCase) AsyncOrderBookProcess(ctx context.Context) error {
	orders := make([]*domain.OrderEntity, 0, 1000)   // TODO: performance?
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	go func() {
		for {
			<-ticker.C
			// TODO
		}
	}()

	for {
		select {
		case order := <-t.orderBookCh:
			orders = append(orders, order)
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *tradingAsyncUseCase) AsyncAPIResultProcess(ctx context.Context) error { //TODO: for what?
	return nil
}

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
