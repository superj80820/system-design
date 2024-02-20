package mysqlandmq

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"gorm.io/gorm/clause"
)

type matchOrderDetailDB struct {
	*domain.MatchOrderDetail
}

func (*matchOrderDetailDB) TableName() string {
	return "match_details"
}

type matchingRepo struct {
	orm              *ormKit.DB
	matchingMQTopic  mq.MQTopic
	orderBookMQTopic mq.MQTopic
}

func CreateMatchingRepo(orm *ormKit.DB, matchingMQTopic, orderBookMQTopic mq.MQTopic) domain.MatchingRepo {
	return &matchingRepo{
		orm:              orm,
		matchingMQTopic:  matchingMQTopic,
		orderBookMQTopic: orderBookMQTopic,
	}
}

func (m *matchingRepo) GetMatchingDetails(orderID int) ([]*domain.MatchOrderDetail, error) {
	var matchOrderDetails []*domain.MatchOrderDetail
	if err := m.orm.Model(&matchOrderDetailDB{}).Where("order_id = ?", orderID).Order("id DESC").Find(&matchOrderDetails).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return matchOrderDetails, nil
}

func (m *matchingRepo) SaveMatchingDetailsWithIgnore(ctx context.Context, matchOrderDetails []*domain.MatchOrderDetail) error {
	matchOrderDetailsDB := make([]*matchOrderDetailDB, len(matchOrderDetails))
	for idx, matchOrderDetail := range matchOrderDetails { // TODO: need for loop to assign?
		matchOrderDetailsDB[idx] = &matchOrderDetailDB{
			MatchOrderDetail: matchOrderDetail,
		}
	}
	if err := m.orm.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(matchOrderDetailsDB).Error; err != nil {
		return errors.Wrap(err, "save match order details failed")
	}
	return nil
}

type mqMessage struct {
	*domain.MatchOrderDetail
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.MatchOrderDetail.SequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (m *matchingRepo) ProduceMatchOrderMQByMatchResult(ctx context.Context, matchResult *domain.MatchResult) error {
	for _, matchDetail := range matchResult.MatchDetails {
		takerOrderDetail := &domain.MatchOrderDetail{
			SequenceID:     matchResult.SequenceID, // TODO: do not use taker sequence?
			OrderID:        matchDetail.TakerOrder.ID,
			CounterOrderID: matchDetail.MakerOrder.ID,
			UserID:         matchDetail.TakerOrder.UserID,
			CounterUserID:  matchDetail.MakerOrder.UserID,
			Direction:      matchDetail.TakerOrder.Direction,
			Price:          matchDetail.Price,
			Quantity:       matchDetail.Quantity,
			Type:           domain.MatchTypeTaker,
			CreatedAt:      matchResult.CreatedAt,
		}
		makerOrderDetail := &domain.MatchOrderDetail{
			SequenceID:     matchResult.SequenceID, // TODO: do not use maker sequence?
			OrderID:        matchDetail.MakerOrder.ID,
			CounterOrderID: matchDetail.TakerOrder.ID,
			UserID:         matchDetail.MakerOrder.UserID,
			CounterUserID:  matchDetail.TakerOrder.UserID,
			Direction:      matchDetail.MakerOrder.Direction,
			Price:          matchDetail.Price,
			Quantity:       matchDetail.Quantity,
			Type:           domain.MatchTypeMaker,
			CreatedAt:      matchResult.CreatedAt,
		}

		if err := m.matchingMQTopic.Produce(ctx, &mqMessage{
			MatchOrderDetail: takerOrderDetail,
		}); err != nil {
			return errors.Wrap(err, "produce failed")
		}

		if err := m.matchingMQTopic.Produce(ctx, &mqMessage{
			MatchOrderDetail: makerOrderDetail,
		}); err != nil {
			return errors.Wrap(err, "produce failed")
		}
	}

	return nil
}

func (m *matchingRepo) ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*domain.MatchOrderDetail) error) {
	m.matchingMQTopic.SubscribeBatch(key, func(messages [][]byte) error {
		details := make([]*domain.MatchOrderDetail, len(messages))
		for idx, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			details[idx] = mqMessage.MatchOrderDetail
		}

		if err := notify(details); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (m *matchingRepo) GetMatchingHistory(maxResults int) ([]*domain.MatchOrderDetail, error) {
	var matchOrderDetails []*domain.MatchOrderDetail
	if err := m.orm.Model(&matchOrderDetailDB{}).Order("id DESC").Limit(maxResults).Find(&matchOrderDetails).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return matchOrderDetails, nil
}

type mqOrderBookMessage struct {
	*domain.OrderBookEntity
}

var _ mq.Message = (*mqOrderBookMessage)(nil)

func (m *mqOrderBookMessage) GetKey() string {
	return strconv.Itoa(m.OrderBookEntity.SequenceID)
}

func (m *mqOrderBookMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (m *matchingRepo) ProduceOrderBook(ctx context.Context, orderBook *domain.OrderBookEntity) error {
	if err := m.orderBookMQTopic.Produce(ctx, &mqOrderBookMessage{
		OrderBookEntity: orderBook,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (m *matchingRepo) ConsumeOrderBook(ctx context.Context, key string, notify func(*domain.OrderBookEntity) error) {
	m.orderBookMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqOrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookEntity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}
