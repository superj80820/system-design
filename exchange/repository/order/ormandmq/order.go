package ormandmq

import (
	"context"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
)

type orderRepo struct {
	orm          *ormKit.DB
	orderMQTopic mq.MQTopic
	tableName    string
}

func CreateOrderRepo(orm *ormKit.DB, orderMQTopic mq.MQTopic) domain.OrderRepo {
	return &orderRepo{
		orm:          orm,
		orderMQTopic: orderMQTopic,
		tableName:    "orders",
	}
}

func (o *orderRepo) GetHistoryOrder(userID int, orderID int) (*domain.OrderEntity, error) {
	var order domain.OrderEntity
	err := o.orm.Table(o.tableName).Where("id = ?", orderID).First(&order).Error
	if mySQLErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mySQLErr, ormKit.ErrRecordNotFound) {
		return nil, errors.Wrap(domain.ErrNoOrder, fmt.Sprintf("error call stack: %+v", err))
	} else if err != nil {
		return nil, errors.Wrap(err, "query order failed")
	}
	if userID != order.UserID {
		return nil, errors.New("not found")
	}
	return &order, nil
}

func (o *orderRepo) GetHistoryOrders(userID int, maxResults int) ([]*domain.OrderEntity, error) {
	var orders []*domain.OrderEntity
	if err := o.orm.Table(o.tableName).Where("user_id = ?", userID).Limit(maxResults).Order("id DESC").Find(&orders).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return orders, nil
}

func (o *orderRepo) SaveHistoryOrdersWithIgnore(sequenceID int, orders []*domain.OrderEntity) error {
	builder := sq.
		Insert(o.tableName).
		Columns("id", "user_id", "sequence_id", "direction", "price", "status", "quantity", "unfilled_quantity", "updated_at", "created_at").
		Suffix("ON DUPLICATE KEY UPDATE "+
			"`status` = CASE WHEN `sequence_id` < ? THEN VALUES(`status`) ELSE `status` END,"+
			"`unfilled_quantity` = CASE WHEN `sequence_id` < ? THEN VALUES(`unfilled_quantity`) ELSE `unfilled_quantity` END,"+
			"`updated_at` = CASE WHEN `sequence_id` < ? THEN VALUES(`updated_at`) ELSE `updated_at` END,"+
			"`sequence_id` = CASE WHEN `sequence_id` < ? THEN VALUES(`sequence_id`) ELSE `sequence_id` END",
			sequenceID,
			sequenceID,
			sequenceID,
			sequenceID,
		)
	for _, order := range orders {
		builder = builder.Values(order.ID, order.UserID, sequenceID, order.Direction, order.Price, order.Status, order.Quantity, order.UnfilledQuantity, order.UpdatedAt, order.CreatedAt)
	}
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	if err := o.orm.Table(o.tableName).Exec(sql, args...).Error; err != nil {
		return errors.Wrap(err, "save orders failed")
	}
	return nil
}

func (o *orderRepo) ProduceOrderMQByTradingResults(ctx context.Context, tradingResults []*domain.TradingResult) error {
	var mqMessages []mq.Message
	for _, tradingResult := range tradingResults {
		mqMessage := mqMessage{
			SequenceID: tradingResult.SequenceID,
		}

		switch tradingResult.TradingResultStatus {
		case domain.TradingResultStatusCreate:
			orders := []*domain.OrderEntity{
				tradingResult.MatchResult.TakerOrder,
			}
			for _, order := range tradingResult.MatchResult.MatchDetails {
				orders = append(orders, order.MakerOrder)
			}
			mqMessage.Orders = orders
		case domain.TradingResultStatusCancel:
			mqMessage.Orders = []*domain.OrderEntity{tradingResult.CancelOrderResult.CancelOrder}
		}

		mqMessages = append(mqMessages, &mqMessage)
	}

	if err := o.orderMQTopic.ProduceBatch(ctx, mqMessages); err != nil {
		return errors.Wrap(err, "produce failed")
	}

	return nil
}

func (o *orderRepo) ConsumeOrderMQ(ctx context.Context, key string, notify func(sequenceID int, orders []*domain.OrderEntity) error) {
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

func (o *orderRepo) ConsumeOrderMQWithCommit(ctx context.Context, key string, notify func(sequenceID int, orders []*domain.OrderEntity, commitFn func() error) error) {
	o.orderMQTopic.SubscribeBatchWithManualCommit(key, func(messages [][]byte, commitFn func() error) error {
		for _, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}

			if err := notify(mqMessage.SequenceID, mqMessage.Orders, commitFn); err != nil {
				return errors.Wrap(err, "notify failed")
			}
		}
		return nil
	})
}
