package sequencer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type tradingSequencerUseCase struct {
	sequencerRepo domain.SequencerRepo
	tradingRepo   domain.TradingRepo
}

func CreateTradingSequencerUseCase(sequencerRepo domain.SequencerRepo, tradingRepo domain.TradingRepo) domain.SequenceTradingUseCase {
	return &tradingSequencerUseCase{
		sequencerRepo: sequencerRepo,
		tradingRepo:   tradingRepo,
	}
}

func (t *tradingSequencerUseCase) CheckEventSequence(sequenceID int, lastSequenceID int) error {
	if err := t.sequencerRepo.CheckEventSequence(sequenceID, lastSequenceID); err != nil {
		return errors.Wrap(err, "check event sequence failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) Continue() error {
	if err := t.sequencerRepo.Continue(); err != nil {
		return errors.Wrap(err, "continue failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) GetSequenceID() uint64 {
	return t.sequencerRepo.GetSequenceID()
}

func (t *tradingSequencerUseCase) Pause() error {
	if err := t.sequencerRepo.Pause(); err != nil {
		return errors.Wrap(err, "pause failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) ProduceSequenceMessages(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	marshalData, err := json.Marshal(*tradingEvent)
	if err != nil {
		return errors.Wrap(err, "marshal failed")
	}
	if err := t.sequencerRepo.ProduceSequenceMessages(ctx, &domain.SequencerEvent{
		ReferenceID: tradingEvent.ReferenceID,
		Data:        string(marshalData),
		CreatedAt:   tradingEvent.CreatedAt,
	}); err != nil {
		return errors.Wrap(err, "produce sequence messages failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) RecoverEvents(offsetSequenceID int, processFn func([]*domain.TradingEvent) error) error {
	if err := t.sequencerRepo.RecoverEvents(offsetSequenceID, func(sequencerEvents []*domain.SequencerEvent) error {
		tradingEvents := make([]*domain.TradingEvent, len(sequencerEvents))
		for idx, sequencerEvent := range sequencerEvents {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal([]byte(sequencerEvent.Data), &tradingEvent); err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			tradingEvent.ReferenceID = sequencerEvent.ReferenceID
			tradingEvent.SequenceID = sequencerEvent.SequenceID
			tradingEvents[idx] = &tradingEvent
		}
		if err := processFn(tradingEvents); err != nil {
			return errors.Wrap(err, "process failed")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "recover events failed")
	}
	return nil
}

func (t *tradingSequencerUseCase) SaveWithFilterEvents(tradingEvents []*domain.TradingEvent, commitFn func() error) ([]*domain.TradingEvent, error) {
	sequenceEvents := make([]*domain.SequencerEvent, len(tradingEvents))
	for idx, tradingEvent := range tradingEvents {
		marshalData, err := json.Marshal(tradingEvent)
		if err != nil {
			return nil, errors.Wrap(err, "marshal failed")
		}
		sequenceEvents[idx] = &domain.SequencerEvent{
			ReferenceID: tradingEvent.ReferenceID,
			SequenceID:  tradingEvent.SequenceID,
			Data:        string(marshalData),
			CreatedAt:   tradingEvent.CreatedAt,
		}
	}
	sequenceEvents, err := t.sequencerRepo.SaveWithFilterEvents(sequenceEvents, commitFn)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("save with filter events failed, events length: %d", len(tradingEvents)))
	}
	tradingEvents = make([]*domain.TradingEvent, len(sequenceEvents))
	for idx, sequenceEvent := range sequenceEvents {
		var tradingEvent domain.TradingEvent
		if err := json.Unmarshal([]byte(sequenceEvent.Data), &tradingEvent); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		tradingEvent.ReferenceID = sequenceEvent.ReferenceID
		tradingEvent.SequenceID = sequenceEvent.SequenceID
		tradingEvents[idx] = &tradingEvent
	}
	return tradingEvents, nil
}

func (t *tradingSequencerUseCase) Shutdown() {
	t.sequencerRepo.Shutdown()
}

func (t *tradingSequencerUseCase) ConsumeSequenceMessages(ctx context.Context) {
	t.sequencerRepo.ConsumeSequenceMessages(func(sequencerEvents []*domain.SequencerEvent, commitFn func() error) {
		tradingEvents := make([]*domain.TradingEvent, len(sequencerEvents))
		for idx, sequencerEvent := range sequencerEvents {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal([]byte(sequencerEvent.Data), &tradingEvent); err != nil {
				panic(errors.Wrap(err, "unmarshal failed")) // TODO
			}
			tradingEvent.ReferenceID = sequencerEvent.ReferenceID
			tradingEvent.SequenceID = sequencerEvent.SequenceID
			tradingEvents[idx] = &tradingEvent
		}
		events, err := t.SaveWithFilterEvents(tradingEvents, commitFn)
		if err != nil {
			panic(errors.Wrap(err, fmt.Sprintf("save with filter events failed, events length: %d", len(events)))) // TODO: error handle
		}
		if err := t.tradingRepo.ProduceTradingEvents(ctx, events); err != nil {
			panic(errors.Wrap(err, "produce trading event failed")) // TODO: error handle
		}
	})
}

func (t *tradingSequencerUseCase) ProduceCancelOrderTradingEvent(ctx context.Context, userID, orderID int) (*domain.TradingEvent, error) {
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

	if err := t.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingSequencerUseCase) ProduceCreateOrderTradingEvent(ctx context.Context, userID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) (*domain.TradingEvent, error) {
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

	if err := t.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingSequencerUseCase) ProduceDepositOrderTradingEvent(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TradingEvent, error) {
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

	if err := t.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingSequencerUseCase) Done() <-chan struct{} {
	panic("TODO unimplemented")
}

func (t *tradingSequencerUseCase) Err() error {
	panic("TODO unimplemented")
}
