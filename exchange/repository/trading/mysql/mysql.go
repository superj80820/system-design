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
	orm     *ormKit.DB
	mqTopic mqKit.MQTopic
	err     error
	doneCh  chan struct{}
	cancel  context.CancelFunc
}

type matchOrderDetailDB struct {
	*domain.MatchOrderDetail
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

func CreateTradingRepo(ctx context.Context, orm *ormKit.DB, mqTopic mqKit.MQTopic) domain.TradingRepo {
	ctx, cancel := context.WithCancel(ctx)

	return &tradingRepo{
		orm:     orm,
		mqTopic: mqTopic,
		doneCh:  make(chan struct{}),
		cancel:  cancel,
	}
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

func (t *tradingRepo) SendTradeMessages(ctx context.Context, tradingEvents []*domain.TradingEvent) {
	for _, tradingEvent := range tradingEvents {
		t.mqTopic.Produce(ctx, &tradingEventStruct{
			TradingEvent: tradingEvent,
		})
	}
}

func (t *tradingRepo) Shutdown() {
	t.cancel()
	<-t.doneCh
}

func (t *tradingRepo) SubscribeTradeMessage(key string, notify func(*domain.TradingEvent)) {
	t.mqTopic.Subscribe(key, func(message []byte) error {
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
