package ormandmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"gorm.io/gorm/clause"
)

type orderRepo struct {
	orm          *ormKit.DB
	orderMQTopic mq.MQTopic
}

type orderEntityDB struct {
	*domain.OrderEntity
}

func (*orderEntityDB) TableName() string {
	return "orders"
}

func CreateOrderRepo(orm *ormKit.DB, orderMQTopic mq.MQTopic) domain.OrderRepo {
	return &orderRepo{
		orm:          orm,
		orderMQTopic: orderMQTopic,
	}
}

func (o *orderRepo) GetHistoryOrder(userID int, orderID int) (*domain.OrderEntity, error) {
	var order orderEntityDB
	err := o.orm.Where("id = ?", orderID).First(&order).Error
	if mySQLErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mySQLErr, ormKit.ErrRecordNotFound) {
		return nil, errors.Wrap(domain.ErrNoOrder, fmt.Sprintf("error call stack: %+v", err))
	} else if err != nil {
		return nil, errors.Wrap(err, "query order failed")
	}
	if userID != order.UserID {
		return nil, errors.New("not found")
	}
	return order.OrderEntity, nil
}

func (o *orderRepo) GetHistoryOrders(userID int, maxResults int) ([]*domain.OrderEntity, error) {
	var orders []*domain.OrderEntity
	if err := o.orm.Model(&orderEntityDB{}).Where("user_id = ?", userID).Limit(maxResults).Order("id DESC").Find(&orders).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return orders, nil
}

func (o *orderRepo) SaveHistoryOrdersWithIgnore(orders []*domain.OrderEntity) error {
	ordersDB := make([]*orderEntityDB, len(orders))
	for idx, order := range orders { // TODO: need for loop to assign?
		ordersDB[idx] = &orderEntityDB{OrderEntity: order}
	}
	if err := o.orm.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(ordersDB).Error; err != nil {
		return errors.Wrap(err, "save orders failed")
	}
	return nil
}

type mqMessage struct {
	SequenceID int
	Orders     []*domain.OrderEntity
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.SequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (o *orderRepo) ProduceOrderMQByTradingResult(ctx context.Context, tradingResult *domain.TradingResult) error {
	sequenceID := tradingResult.SequenceID

	switch tradingResult.TradingResultStatus {
	case domain.TradingResultStatusCreate:
		orders := []*domain.OrderEntity{
			tradingResult.MatchResult.TakerOrder,
		}
		for _, order := range tradingResult.MatchResult.MatchDetails {
			orders = append(orders, order.MakerOrder)
		}
		if err := o.orderMQTopic.Produce(ctx, &mqMessage{
			SequenceID: sequenceID,
			Orders:     orders,
		}); err != nil {
			return errors.Wrap(err, "produce failed")
		}
	case domain.TradingResultStatusCancel:
		if err := o.orderMQTopic.Produce(ctx, &mqMessage{
			SequenceID: sequenceID,
			Orders:     []*domain.OrderEntity{tradingResult.CancelOrderResult.CancelOrder},
		}); err != nil {
			return errors.Wrap(err, "produce failed")
		}
	}

	return nil
}

func (o *orderRepo) ConsumeOrderMQBatch(ctx context.Context, key string, notify func(sequenceID int, orders []*domain.OrderEntity) error) {
	o.orderMQTopic.SubscribeBatch(key, func(messages [][]byte) error {
		for _, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}

			if err := notify(mqMessage.SequenceID, mqMessage.Orders); err != nil {
				return errors.Wrap(err, "notify failed")
			}
		}
		return nil
	})
}
