package kafkaandmysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	mqKit "github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
)

type sequencerEventDBEntity struct {
	*domain.SequencerEvent
}

type tradingEventKafkaEntity struct {
	*domain.TradingEvent
}

var _ mqKit.Message = (*tradingEventKafkaEntity)(nil)

func (t *tradingEventKafkaEntity) GetKey() string {
	return strconv.Itoa(t.SequenceID)
}

func (t *tradingEventKafkaEntity) Marshal() ([]byte, error) {
	marshalMessage, err := json.Marshal(t)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalMessage, nil
}

func (sequencerEventDBEntity) TableName() string {
	return "events"
}

type tradingSequencerRepo struct {
	orm        *ormKit.DB
	sequenceMQ mqKit.MQTopic

	pauseCh    chan struct{}
	continueCh chan struct{}

	sequence    *atomic.Uint64
	isSubscribe *atomic.Bool
}

func CreateTradingSequencerRepo(ctx context.Context, sequenceMQ mqKit.MQTopic, orm *ormKit.DB) (domain.SequencerRepo[domain.TradingEvent], error) {
	var sequence atomic.Uint64
	t := &tradingSequencerRepo{
		orm:         orm,
		sequenceMQ:  sequenceMQ,
		pauseCh:     make(chan struct{}),
		continueCh:  make(chan struct{}),
		sequence:    &sequence,
		isSubscribe: new(atomic.Bool),
	}

	maxSequenceID, err := t.GetMaxSequenceID()
	if errors.Is(err, ormKit.ErrRecordNotFound) {
		// do nothing
	} else if err != nil {
		return nil, errors.Wrap(err, "get max sequence id failed")
	}

	sequence.Add(maxSequenceID)

	return t, nil
}

func (t *tradingSequencerRepo) GetMaxSequenceID() (uint64, error) {
	var sequencerEvent sequencerEventDBEntity
	if err := t.orm.Order("sequence_id DESC").First(&sequencerEvent).Error; err != nil {
		return 0, errors.Wrap(err, "get sequence id failed")
	}
	return uint64(sequencerEvent.SequenceID), nil
}

func (t *tradingSequencerRepo) SendTradeSequenceMessages(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	if err := t.sequenceMQ.Produce(ctx, &tradingEventKafkaEntity{
		TradingEvent: tradingEvent,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (t *tradingSequencerRepo) SaveEvent(sequencerEvent *domain.SequencerEvent) error {
	if err := t.orm.Create(&sequencerEventDBEntity{
		SequencerEvent: sequencerEvent,
	}).Error; err != nil {
		return errors.Wrap(err, "create trading event failed")
	}
	return nil
}

func (t *tradingSequencerRepo) SaveEvents(sequencerEvents []*domain.SequencerEvent) error {
	sequencerEventsDBEntity := make([]*sequencerEventDBEntity, len(sequencerEvents)) // TODO: optimize(no for loop)
	for idx, sequencerEvent := range sequencerEvents {
		sequencerEventsDBEntity[idx] = &sequencerEventDBEntity{
			SequencerEvent: sequencerEvent,
		}
	}
	if err := t.orm.Create(&sequencerEventsDBEntity).Error; err != nil {
		return errors.Wrap(err, "create trading events failed")
	}
	return nil
}

func (t *tradingSequencerRepo) Pause() error {
	select {
	case t.pauseCh <- struct{}{}:
	default:
		// for no block
	}
	return nil
}

func (t *tradingSequencerRepo) Continue() error {
	select {
	case t.continueCh <- struct{}{}:
	default:
		// for no block
	}
	return nil
}

func (t *tradingSequencerRepo) Shutdown() {
	t.sequenceMQ.Shutdown() // TODO
}

func (t *tradingSequencerRepo) SubscribeGlobalTradeSequenceMessages(notify func([]*domain.TradingEvent, func() error)) {
	if !t.isSubscribe.CompareAndSwap(false, true) {
		return
	}
	t.sequenceMQ.SubscribeBatchWithManualCommit("global-sequencer", func(messages [][]byte, commitFn func() error) error {
		select {
		case <-t.pauseCh:
			<-t.continueCh
		default:
		}

		tradingEvents := make([]*domain.TradingEvent, len(messages))
		for idx := range messages {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal(messages[idx], &tradingEvent); err != nil {
				return errors.Wrap(err, "json unmarshal failed")
			}
			tradingEvents[idx] = &tradingEvent
		}
		notify(tradingEvents, commitFn)
		return nil
	})
}

func (t *tradingSequencerRepo) GenerateNextSequenceID() uint64 {
	return t.sequence.Add(1)
}

func (t *tradingSequencerRepo) GetCurrentSequenceID() uint64 {
	return t.sequence.Load()
}

func (t *tradingSequencerRepo) GetFilterEventsMap(sequencerEvents []*domain.SequencerEvent) (map[int64]bool, error) {
	existsQuery := make([]string, len(sequencerEvents))
	var sequencerEventDB sequencerEventDBEntity
	for idx, sequencerEvent := range sequencerEvents {
		existsQuery[idx] = fmt.Sprintf("EXISTS(SELECT 1 FROM %s WHERE ReferenceID = %d) AS %d", sequencerEventDB.TableName(), sequencerEvent.ReferenceID, sequencerEvent.ReferenceID)
	}
	res := make(map[int64]bool)
	if err := t.orm.Raw("SELECT " + strings.Join(existsQuery, ",")).Scan(&res).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return res, nil
}

func (t *tradingSequencerRepo) ResetSequence() error {
	maxSequenceID, err := t.GetMaxSequenceID()
	if err != nil {
		return errors.Wrap(err, "get max sequence id failed")
	}
	t.sequence.Store(maxSequenceID)
	return nil
}
