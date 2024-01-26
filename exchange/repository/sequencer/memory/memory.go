package memory

import (
	"context"
	"sync/atomic"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/util"
)

type tradingSequencerRepo struct {
	sequence       *atomic.Uint64
	tradingEventCh chan *domain.TradingEvent
	observers      util.GenericSyncMap[*func(*domain.TradingEvent), func(*domain.TradingEvent)] // TODO: test key safe?
	cancel         context.CancelFunc
	done           chan struct{}
}

func CreateTradingSequencerRepo(ctx context.Context) domain.SequencerRepo[domain.TradingEvent] {
	ctx, cancel := context.WithCancel(ctx)

	var sequence atomic.Uint64
	tradingEventCh := make(chan *domain.TradingEvent)
	done := make(chan struct{})
	t := &tradingSequencerRepo{
		sequence:       &sequence,
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

func (t *tradingSequencerRepo) GenerateNextSequenceID() uint64 {
	return t.sequence.Add(1)
}

func (t *tradingSequencerRepo) GetCurrentSequenceID() uint64 {
	return t.sequence.Load()
}

func (t *tradingSequencerRepo) GetMaxSequenceID() (uint64, error) {
	return t.sequence.Load(), nil
}

func (t *tradingSequencerRepo) Shutdown() {
	t.cancel()
	<-t.done
}

// noop
func (*tradingSequencerRepo) SaveEvent(sequencerEvent *domain.SequencerEvent) error {
	return nil
}

// noop
func (*tradingSequencerRepo) SaveEvents(sequencerEvents []*domain.SequencerEvent) error {
	return nil
}

func (t *tradingSequencerRepo) SendTradeSequenceMessages(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	t.tradingEventCh <- tradingEvent
	return nil
}

func (t *tradingSequencerRepo) SubscribeTradeSequenceMessage(notify func(*domain.TradingEvent)) {
	t.observers.Store(&notify, notify)
}

func (t *tradingSequencerRepo) GetFilterEventsMap([]*domain.SequencerEvent) (map[int64]bool, error) {
	panic("need implement")
}

func (t *tradingSequencerRepo) ResetSequence() error {
	panic("need implement")
}
