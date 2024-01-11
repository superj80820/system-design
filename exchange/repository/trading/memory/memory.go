package memory

import (
	"context"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

type tradingRepo struct {
	tradingEventCh chan *domain.TradingEvent
	err            error
	observers      util.GenericSyncMap[*func(*domain.TradingEvent), func(*domain.TradingEvent)] // TODO: test key safe?
	cancel         context.CancelFunc
	done           chan struct{}
}

func CreateTradingRepo(ctx context.Context) domain.TradingRepo {
	ctx, cancel := context.WithCancel(ctx)

	tradingEventCh := make(chan *domain.TradingEvent)
	done := make(chan struct{})

	t := &tradingRepo{
		tradingEventCh: tradingEventCh,
		cancel:         cancel,
		done:           done,
	}

	go func() {
		for {
			select {
			case tradingEvent := <-t.tradingEventCh:
				t.observers.Range(func(key *func(*domain.TradingEvent), value func(*domain.TradingEvent)) bool {
					value(tradingEvent)
					return true
				})
			case <-ctx.Done():
				close(done)
			}
		}
	}()

	return t
}

func (t *tradingRepo) Done() <-chan struct{} {
	return t.done
}

// Err implements domain.TradingRepo.
func (t *tradingRepo) Err() error {
	return t.err
}

func (t *tradingRepo) SendTradeMessages(tradingEvent *domain.TradingEvent) {
	t.tradingEventCh <- tradingEvent
}

func (t *tradingRepo) Shutdown() {
	t.cancel()
	<-t.done
}

func (t *tradingRepo) SubscribeTradeMessage(notify func(*domain.TradingEvent)) {
	t.observers.Store(&notify, notify)
}
