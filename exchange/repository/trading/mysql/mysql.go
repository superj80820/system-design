package mysql

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	mqKit "github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"gorm.io/gorm/clause"
)

type tradingRepo struct {
	orm                  *ormKit.DB
	tradingEventMQTopic  mqKit.MQTopic
	tradingResultMQTopic mqKit.MQTopic
	err                  error
	doneCh               chan struct{}
	cancel               context.CancelFunc
}

type matchOrderDetailDB struct {
	*domain.MatchOrderDetail
}

func (*matchOrderDetailDB) TableName() string {
	return "match_details"
}

type tradingResultStruct struct {
	*domain.TradingResult
}

var _ mq.Message = (*tradingResultStruct)(nil)

func (t *tradingResultStruct) GetKey() string {
	return strconv.FormatInt(t.TradingEvent.ReferenceID, 10)
}

func (t *tradingResultStruct) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(*t)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

type tradingEventStruct struct {
	*domain.TradingEvent
}

var _ mq.Message = (*tradingEventStruct)(nil)

func (t *tradingEventStruct) GetKey() string {
	return strconv.Itoa(t.TradingEvent.UniqueID) // TODO: test correct
}

func (t *tradingEventStruct) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(*t)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func CreateTradingRepo(ctx context.Context, orm *ormKit.DB, tradingEventMQTopic, tradingResultMQTopic mqKit.MQTopic) domain.TradingRepo {
	ctx, cancel := context.WithCancel(ctx)

	return &tradingRepo{
		orm:                  orm,
		tradingEventMQTopic:  tradingEventMQTopic,
		tradingResultMQTopic: tradingResultMQTopic,
		doneCh:               make(chan struct{}),
		cancel:               cancel,
	}
}

func (t *tradingRepo) SendTradingResult(ctx context.Context, tradingResult *domain.TradingResult) error {
	if err := t.tradingResultMQTopic.Produce(ctx, &tradingResultStruct{
		TradingResult: tradingResult,
	}); err != nil {
		return errors.Wrap(err, "produce trading result failed")
	}
	return nil
}

func (t *tradingRepo) SubscribeTradingResult(key string, notify func(*domain.TradingResult)) {
	t.tradingResultMQTopic.Subscribe(key, func(message []byte) error {
		var tradingResult domain.TradingResult
		if err := json.Unmarshal(message, &tradingResult); err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		notify(&tradingResult)
		return nil
	})
}

func (t *tradingRepo) GetMatchingDetails(orderID int) ([]*domain.MatchOrderDetail, error) {
	var matchOrderDetails []*domain.MatchOrderDetail
	if err := t.orm.Model(&matchOrderDetailDB{}).Where("order_id = ?", orderID).Order("id DESC").Find(&matchOrderDetails).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return matchOrderDetails, nil
}

func (t *tradingRepo) SaveMatchingDetailsWithIgnore(ctx context.Context, matchOrderDetails []*domain.MatchOrderDetail) error {
	matchOrderDetailsDB := make([]*matchOrderDetailDB, len(matchOrderDetails))
	for idx, matchOrderDetail := range matchOrderDetails { // TODO: need for loop to assign?
		matchOrderDetailsDB[idx] = &matchOrderDetailDB{
			MatchOrderDetail: matchOrderDetail,
		}
	}
	if err := t.orm.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(matchOrderDetailsDB).Error; err != nil {
		return errors.Wrap(err, "save match order details failed")
	}
	return nil
}

func (t *tradingRepo) SendTradeEvent(ctx context.Context, tradingEvents []*domain.TradingEvent) {
	for _, tradingEvent := range tradingEvents {
		t.tradingEventMQTopic.Produce(ctx, &tradingEventStruct{
			TradingEvent: tradingEvent,
		})
	}
}

func (t *tradingRepo) Shutdown() {
	t.cancel()
	<-t.doneCh
}

func (t *tradingRepo) SubscribeTradeEvent(key string, notify func(*domain.TradingEvent)) {
	t.tradingEventMQTopic.Subscribe(key, func(message []byte) error {
		var tradingEvent domain.TradingEvent
		if err := json.Unmarshal(message, &tradingEvent); err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		notify(&tradingEvent)
		return nil
	})
}

func (t *tradingRepo) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingRepo) Err() error {
	return t.err
}
