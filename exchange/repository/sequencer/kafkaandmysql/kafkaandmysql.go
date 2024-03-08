package kafkaandmysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	mqKit "github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type sequenceKafkaMessage struct {
	*domain.SequencerEvent
}

func (s *sequenceKafkaMessage) GetKey() string {
	return strconv.Itoa(s.SequenceID)
}

func (s *sequenceKafkaMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

type sequencerRepo struct {
	orm        *ormKit.DB
	sequenceMQ mqKit.MQTopic
	tableName  string

	pauseCh    chan struct{}
	continueCh chan struct{}

	sequence *atomic.Uint64
}

func CreateSequencerRepo(ctx context.Context, sequenceMQ mqKit.MQTopic, orm *ormKit.DB) (domain.SequencerRepo, error) {
	var sequence atomic.Uint64
	t := &sequencerRepo{
		orm:        orm,
		sequenceMQ: sequenceMQ,
		tableName:  "events",
		pauseCh:    make(chan struct{}),
		continueCh: make(chan struct{}),
		sequence:   &sequence,
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

func (s *sequencerRepo) GetMaxSequenceID() (uint64, error) {
	var sequencerEvent domain.SequencerEvent
	if err := s.orm.Table(s.tableName).Order("sequence_id DESC").First(&sequencerEvent).Error; err != nil {
		return 0, errors.Wrap(err, "get sequence id failed")
	}
	return uint64(sequencerEvent.SequenceID), nil
}

func (s *sequencerRepo) ProduceSequenceMessages(ctx context.Context, message *domain.SequencerEvent) error {
	if err := s.sequenceMQ.Produce(ctx, &sequenceKafkaMessage{SequencerEvent: message}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (s *sequencerRepo) SaveWithFilterEvents(sequenceEvents []*domain.SequencerEvent, commitFn func() error) ([]*domain.SequencerEvent, error) {
	previousIDInt, err := utilKit.SafeUint64ToInt(s.GetSequenceID())
	if err != nil {
		return nil, errors.Wrap(err, "uint64 to int overflow")
	}

	var curSequenceID int
	for idx, sequenceEvent := range sequenceEvents {
		sequenceID := previousIDInt + 1 + idx
		sequenceEvent.SequenceID = sequenceID
		sequenceEvent.CreatedAt = time.Now()
		curSequenceID = sequenceID
	}

	err = s.SaveEvents(sequenceEvents)
	if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
		// TODO: test
		// if duplicate, filter events then retry
		filterEventsMap, err := s.GetFilterEventsMap(sequenceEvents)
		if err != nil {
			return nil, errors.Wrap(err, "get filter events map failed")
		}
		var filterSequenceEvents []*domain.SequencerEvent
		for _, val := range sequenceEvents {
			if filterEventsMap[val.ReferenceID] {
				continue
			}
			filterSequenceEvents = append(filterSequenceEvents, val)
		}

		if len(filterSequenceEvents) != 0 {
			return s.SaveWithFilterEvents(filterSequenceEvents, commitFn)
		}
	} else if err != nil {
		return nil, errors.Wrap(err, "save event failed")
	}

	if err := commitFn(); err != nil {
		return nil, errors.Wrap(err, "commit latest message failed")
	}

	s.SetSequenceID(uint64(curSequenceID))

	return sequenceEvents, nil
}

func (s *sequencerRepo) CheckEventSequence(sequenceID, lastSequenceID int) error {
	if sequenceID <= lastSequenceID {
		return errors.Wrap(domain.ErrGetDuplicateEvent, "skip duplicate, last sequence id: "+strconv.Itoa(lastSequenceID)+", message event sequence id: "+strconv.Itoa(sequenceID))
	} else if (sequenceID - lastSequenceID) > 1 {
		return errors.Wrap(domain.ErrMissEvent, "last sequence id: "+strconv.Itoa(lastSequenceID)+", message event sequence id: "+strconv.Itoa(sequenceID))
	}
	return nil
}

func (s *sequencerRepo) RecoverEvents(offsetSequenceID int, processFn func([]*domain.SequencerEvent) error) error {
	for page, isEnd := 1, false; !isEnd; page++ {
		var (
			events []*domain.SequencerEvent
			err    error
		)
		events, isEnd, err = s.GetHistoryEvents(offsetSequenceID+1, page, 1000)
		if err != nil {
			return errors.Wrap(err, "get history events failed")
		}
		if err := processFn(events); err != nil {
			return errors.Wrap(err, "process events failed")
		}
	}
	return nil
}

func (s *sequencerRepo) GetHistoryEvents(offsetSequenceID, page, limit int) (sequencerEvents []*domain.SequencerEvent, isEnd bool, err error) {
	var maxEvent domain.SequencerEvent
	if err := s.orm.Table(s.tableName).Order("sequence_id DESC").Limit(1).Select("sequence_id").First(&maxEvent).Error; err != nil {
		return nil, false, errors.Wrap(err, "get max event failed")
	}

	var events []*domain.SequencerEvent
	if err := s.orm.Table(s.tableName).Where("sequence_id >= ?", offsetSequenceID).Order("sequence_id ASC").Offset((page - 1) * limit).Limit(limit).Find(&events).Error; err != nil {
		return nil, false, errors.Wrap(err, "find events failed")
	}

	if maxEvent.SequenceID <= offsetSequenceID+(page-1)*limit {
		return events, true, nil
	}
	return events, false, nil
}

func (s *sequencerRepo) SaveEvents(sequencerEvents []*domain.SequencerEvent) error {
	if err := s.orm.Table(s.tableName).Create(&sequencerEvents).Error; err != nil {
		return errors.Wrap(err, "create events failed")
	}
	return nil
}

func (s *sequencerRepo) Pause() error {
	select {
	case s.pauseCh <- struct{}{}:
	default:
		// for no block
	}
	return nil
}

func (s *sequencerRepo) Continue() error {
	select {
	case s.continueCh <- struct{}{}:
	default:
		// for no block
	}
	return nil
}

func (s *sequencerRepo) Shutdown() {
	s.sequenceMQ.Shutdown() // TODO
}

func (s *sequencerRepo) ConsumeSequenceMessages(notify func([]*domain.SequencerEvent, func() error)) {
	s.sequenceMQ.SubscribeBatchWithManualCommit("global-sequencer", func(messages [][]byte, commitFn func() error) error {
		select {
		case <-s.pauseCh:
			<-s.continueCh
		default:
		}

		sequenceEvents := make([]*domain.SequencerEvent, len(messages))
		for idx, message := range messages {
			var sequenceEvent domain.SequencerEvent
			if err := json.Unmarshal(message, &sequenceEvent); err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			sequenceEvents[idx] = &sequenceEvent
		}

		notify(sequenceEvents, commitFn)
		return nil
	})
}

func (s *sequencerRepo) SetSequenceID(sequenceID uint64) {
	s.sequence.Store(sequenceID)
}

func (s *sequencerRepo) GetSequenceID() uint64 {
	return s.sequence.Load()
}

func (s *sequencerRepo) GetFilterEventsMap(sequencerEvents []*domain.SequencerEvent) (map[int]bool, error) {
	existsQuery := make([]string, len(sequencerEvents))
	for idx, sequencerEvent := range sequencerEvents {
		existsQuery[idx] = fmt.Sprintf("EXISTS(SELECT 1 FROM %s WHERE ReferenceID = %d) AS %d", s.tableName, sequencerEvent.ReferenceID, sequencerEvent.ReferenceID)
	}
	res := make(map[int]bool)
	if err := s.orm.Table(s.tableName).Raw("SELECT " + strings.Join(existsQuery, ",")).Scan(&res).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return res, nil
}
