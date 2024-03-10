package mysql

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mqMessage struct {
	*domain.TradingEvent
}

func (s *mqMessage) GetKey() string {
	return strconv.Itoa(s.SequenceID)
}

func (s *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

type tradingRepo struct {
	mongoCollection *mongo.Collection
	orm             *ormKit.DB
	tradingEventMQ  mq.MQTopic
	err             error
	doneCh          chan struct{}
	cancel          context.CancelFunc
}

type tradingResultStruct struct {
	*domain.TradingResult
}

var _ mq.Message = (*tradingResultStruct)(nil)

func (t *tradingResultStruct) GetKey() string {
	return strconv.Itoa(t.TradingEvent.ReferenceID)
}

func (t *tradingResultStruct) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(*t)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func CreateTradingRepo(
	ctx context.Context,
	mongoCollection *mongo.Collection,
	tradingEventMQ mq.MQTopic,
	orm *ormKit.DB,
) domain.TradingRepo {
	_, cancel := context.WithCancel(ctx)

	return &tradingRepo{
		orm:             orm,
		mongoCollection: mongoCollection,
		tradingEventMQ:  tradingEventMQ,
		doneCh:          make(chan struct{}),
		cancel:          cancel,
	}
}

func (t *tradingRepo) Shutdown() {
	t.cancel()
	<-t.doneCh
}

func (t *tradingRepo) GetHistorySnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	findOptions := options.Find()
	findOptions.SetLimit(1)
	findOptions.SetSort(bson.D{{Key: "sequence_id", Value: -1}})
	tradingSnapshotResult, err := t.mongoCollection.Find(ctx, bson.D{}, findOptions)
	if err != nil {
		return nil, errors.Wrap(err, "find failed")
	}
	var tradingSnapshots []*domain.TradingSnapshot
	if err := tradingSnapshotResult.All(ctx, &tradingSnapshots); err != nil {
		return nil, errors.Wrap(err, "decode failed")
	}
	if len(tradingSnapshots) == 0 {
		return nil, domain.ErrNoData
	}
	if len(tradingSnapshots) != 1 {
		return nil, errors.New("except trading snapshots length")
	}
	return tradingSnapshots[0], nil
}

func (t *tradingRepo) SaveSnapshot(ctx context.Context, sequenceID int, usersAssetsData map[int]map[int]*domain.UserAsset, ordersData []*domain.OrderEntity, matchesData *domain.MatchData) error {
	_, err := t.mongoCollection.InsertOne(ctx, domain.TradingSnapshot{
		SequenceID:  sequenceID,
		UsersAssets: usersAssetsData,
		Orders:      ordersData,
		MatchData:   matchesData,
	})
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrDuplicate
	} else if err != nil {
		return errors.Wrap(err, "save snapshot failed")
	}
	return nil
}

func (t *tradingRepo) ProduceTradingEvent(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	if err := t.tradingEventMQ.Produce(ctx, &mqMessage{TradingEvent: tradingEvent}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (t *tradingRepo) ConsumeTradingEvent(ctx context.Context, key string, notify func(events []*domain.TradingEvent, commitFn func() error)) {
	t.tradingEventMQ.SubscribeBatchWithManualCommit(key, func(messages [][]byte, commitFn func() error) error {
		tradingEvents := make([]*domain.TradingEvent, len(messages))
		for idx, message := range messages {
			var tradingEvent domain.TradingEvent
			if err := json.Unmarshal(message, &tradingEvent); err != nil {
				panic(errors.Wrap(err, "unmarshal failed")) // TODO
			}
			tradingEvents[idx] = &tradingEvent
		}
		notify(tradingEvents, commitFn)
		return nil
	})
}

func (t *tradingRepo) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingRepo) Err() error {
	return t.err
}
