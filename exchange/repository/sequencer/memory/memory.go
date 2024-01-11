package memory

import (
	"context"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

type tradingSequencerRepo struct {
	tradingEventCh chan *domain.TradingEvent
	observers      util.GenericSyncMap[*func(*domain.TradingEvent), func(*domain.TradingEvent)] // TODO: test key safe?
	cancel         context.CancelFunc
	done           chan struct{}
}

func CreateTradingSequencerRepo(ctx context.Context) domain.TradingSequencerRepo {
	ctx, cancel := context.WithCancel(ctx)

	tradingEventCh := make(chan *domain.TradingEvent)
	done := make(chan struct{})

	t := &tradingSequencerRepo{
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

func (*tradingSequencerRepo) GetMaxSequenceID() uint64 {
	return 0
}

func (t *tradingSequencerRepo) Shutdown() {
	t.cancel()
	<-t.done
}

func (*tradingSequencerRepo) SaveEvent(tradingEvent *domain.TradingEvent) {
	// noop
}

func (t *tradingSequencerRepo) SendTradeSequenceMessages(tradingEvent *domain.TradingEvent) {
	t.tradingEventCh <- tradingEvent
}

func (t *tradingSequencerRepo) SubscribeTradeSequenceMessage(notify func(*domain.TradingEvent)) {
	t.observers.Store(&notify, notify)
}
