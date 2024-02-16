package sequencer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type tradingSequencerUseCase struct {
	lock                *sync.Mutex
	doneCh              chan struct{}
	logger              loggerKit.Logger
	err                 error
	sequencerRepo       domain.SequencerRepo[domain.TradingEvent]
	tradingRepo         domain.TradingRepo
	tradingUseCase      domain.TradingUseCase
	lastTimestamp       time.Time
	batchEventsSize     int
	batchEventsDuration time.Duration
}

type batchEventsStruct struct {
	tradingEvent   []*domain.TradingEvent
	sequencerEvent []*domain.SequencerEvent
	commitFns      []func() error
}

func CreateTradingSequencerUseCase(
	logger loggerKit.Logger,
	sequencerRepo domain.SequencerRepo[domain.TradingEvent],
	tradingRepo domain.TradingRepo,
	tradingUseCase domain.TradingUseCase,
	batchEventsSize int,
	batchEventsDuration time.Duration,
) domain.TradingSequencerUseCase {
	return &tradingSequencerUseCase{
		lock:                new(sync.Mutex),
		doneCh:              make(chan struct{}),
		sequencerRepo:       sequencerRepo,
		tradingRepo:         tradingRepo,
		tradingUseCase:      tradingUseCase,
		logger:              logger,
		batchEventsSize:     batchEventsSize,
		batchEventsDuration: batchEventsDuration,
	}
}

func (t *tradingSequencerUseCase) ConsumeTradingEventThenProduce(ctx context.Context) {
	var batchEvents batchEventsStruct
	ticker := time.NewTicker(t.batchEventsDuration)
	lock := new(sync.Mutex)
	setErrAndDone := func(err error) {
		t.lock.Lock()
		defer t.lock.Unlock()
		fmt.Println(fmt.Sprintf("%+v", err))
		ticker.Stop()
		t.err = err
		close(t.doneCh)
	}
	eventsFullCh := make(chan struct{})
	sequenceMessageFn := func(tradingEvent *domain.TradingEvent, commitFn func() error) {
		timeNow := time.Now()
		if timeNow.Before(t.lastTimestamp) {
			setErrAndDone(errors.New("now time is before last timestamp"))
			return
		}
		t.lastTimestamp = timeNow

		// sequence event
		previousID := t.sequencerRepo.GetCurrentSequenceID()
		previousIDInt, err := utilKit.SafeUint64ToInt(previousID)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "uint64 to int overflow"))
			return
		}
		sequenceID := t.sequencerRepo.GenerateNextSequenceID()
		sequenceIDInt, err := utilKit.SafeUint64ToInt(sequenceID)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "uint64 to int overflow"))
			return
		}
		tradingEvent.PreviousID = previousIDInt
		tradingEvent.SequenceID = sequenceIDInt
		tradingEvent.CreatedAt = t.lastTimestamp

		marshalData, err := json.Marshal(*tradingEvent)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "marshal failed"))
			return
		}

		lock.Lock()
		batchEvents.sequencerEvent = append(batchEvents.sequencerEvent, &domain.SequencerEvent{
			ReferenceID: int64(tradingEvent.ReferenceID),
			SequenceID:  int64(tradingEvent.SequenceID),
			PreviousID:  int64(tradingEvent.PreviousID),
			Data:        string(marshalData),
			CreatedAt:   time.Now(),
		})
		batchEvents.tradingEvent = append(batchEvents.tradingEvent, tradingEvent)
		batchEvents.commitFns = append(batchEvents.commitFns, commitFn)
		batchEventsLength := len(batchEvents.sequencerEvent)
		lock.Unlock()

		if batchEventsLength >= t.batchEventsSize {
			eventsFullCh <- struct{}{}
		}
	}

	t.sequencerRepo.SubscribeTradeSequenceMessage(sequenceMessageFn)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		snapshotSequenceID := t.tradingUseCase.GetSequenceID()

		for range ticker.C {
			if err := t.sequencerRepo.Pause(); err != nil {
				setErrAndDone(errors.Wrap(err, "pause failed"))
				return
			}
			sequenceID := t.tradingUseCase.GetSequenceID()
			if snapshotSequenceID == sequenceID {
				if err := t.sequencerRepo.Continue(); err != nil {
					setErrAndDone(errors.Wrap(err, "continue failed"))
					return
				}
				continue
			}
			snapshot, err := t.tradingUseCase.GetLatestSnapshot(ctx)

			if errors.Is(err, domain.ErrNoop) {
				continue
			} else if err != nil {
				setErrAndDone(errors.Wrap(err, "get snapshot failed"))
				return
			}
			if err := t.sequencerRepo.Continue(); err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			if err = t.tradingUseCase.SaveSnapshot(ctx, snapshot); !errors.Is(err, domain.ErrDuplicate) && err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			snapshotSequenceID = sequenceID
		}
	}()

	go func() {
		errContinue := errors.New("continue")
		fn := func() error {
			sequencerEventClone, tradingEventClone, latestCommitFn, err :=
				func() ([]*domain.SequencerEvent, []*domain.TradingEvent, func() error, error) {
					lock.Lock()
					defer lock.Unlock()

					if len(batchEvents.sequencerEvent) == 0 || len(batchEvents.tradingEvent) == 0 {
						return nil, nil, nil, errors.Wrap(errContinue, "event length is zero")
					}

					if len(batchEvents.sequencerEvent) != len(batchEvents.tradingEvent) {
						panic("except trading event and sequencer event length")
					}
					sequencerEventClone := make([]*domain.SequencerEvent, len(batchEvents.sequencerEvent))
					copy(sequencerEventClone, batchEvents.sequencerEvent)
					tradingEventClone := make([]*domain.TradingEvent, len(batchEvents.tradingEvent))
					copy(tradingEventClone, batchEvents.tradingEvent)
					latestCommitFn := batchEvents.commitFns[len(batchEvents.commitFns)-1]
					batchEvents.sequencerEvent = nil // reset
					batchEvents.tradingEvent = nil   // reset
					batchEvents.commitFns = nil

					return sequencerEventClone, tradingEventClone, latestCommitFn, nil
				}()
			if err != nil {
				return errors.Wrap(err, "clone events failed")
			}

			err = t.sequencerRepo.SaveEvents(sequencerEventClone)
			if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
				// TODO: test
				// if duplicate, filter events then retry
				filterEventsMap, err := t.sequencerRepo.GetFilterEventsMap(sequencerEventClone)
				if err != nil {
					return errors.Wrap(err, "get filter events map failed")
				}
				var filterSequencerEventClone []*domain.SequencerEvent
				for _, val := range sequencerEventClone {
					if filterEventsMap[val.ReferenceID] {
						continue
					}
					filterSequencerEventClone = append(filterSequencerEventClone, val)
				}
				var filterTradingEventClone []*domain.TradingEvent
				for _, val := range tradingEventClone {
					if filterEventsMap[val.ReferenceID] {
						continue
					}
					filterTradingEventClone = append(filterTradingEventClone, val)
				}

				if len(filterSequencerEventClone) == 0 || len(filterTradingEventClone) == 0 {
					return nil
				}

				if len(batchEvents.sequencerEvent) != len(batchEvents.tradingEvent) {
					panic("except trading event and sequencer event length")
				}

				if err := t.sequencerRepo.SaveEvents(sequencerEventClone); err != nil {
					panic(errors.Wrap(err, "save event failed")) // TODO: use panic?
				}

				if err := latestCommitFn(); err != nil {
					return errors.Wrap(err, "commit latest message failed")
				}

				t.tradingRepo.SendTradeEvent(ctx, tradingEventClone)

				return nil
			} else if err != nil {
				panic(errors.Wrap(err, "save event failed")) // TODO: use panic?
			}

			if err := latestCommitFn(); err != nil {
				return errors.Wrap(err, "commit latest message failed")
			}

			t.tradingRepo.SendTradeEvent(ctx, tradingEventClone)

			return nil
		}
		for {
			select {
			case <-ticker.C:
				if err := fn(); errors.Is(err, errContinue) {
					continue
				} else if err != nil {
					setErrAndDone(errors.Wrap(err, "clone event failed"))
					return
				}
			case <-eventsFullCh:
				if err := fn(); errors.Is(err, errContinue) {
					continue
				} else if err != nil {
					setErrAndDone(errors.Wrap(err, "clone event failed"))
					return
				}
			}
		}
	}()
}

func (t *tradingSequencerUseCase) ProduceTradingEvent(ctx context.Context, tradingEvent *domain.TradingEvent) (int64, error) {
	tradingEvent.ReferenceID = utilKit.GetSnowflakeIDInt64()
	if err := t.sequencerRepo.SendTradeSequenceMessages(ctx, tradingEvent); err != nil {
		t.lock.Lock()
		defer t.lock.Unlock()
		t.err = err
		close(t.doneCh)
	}
	return tradingEvent.ReferenceID, nil
}

func (t *tradingSequencerUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingSequencerUseCase) Err() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.Err()
}
