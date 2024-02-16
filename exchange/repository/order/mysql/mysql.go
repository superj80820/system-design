package mysql

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
	orm              *ormKit.DB
	orderMQTopic     mq.MQTopic
	orderSaveMQTopic mq.MQTopic
}

type orderEntityDB struct {
	*domain.OrderEntity
}

func (*orderEntityDB) TableName() string {
	return "orders"
}

func CreateOrderRepo(orm *ormKit.DB, orderMQTopic, orderSaveMQTopic mq.MQTopic) domain.OrderRepo {
	return &orderRepo{
		orm:              orm,
		orderMQTopic:     orderMQTopic,
		orderSaveMQTopic: orderSaveMQTopic,
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
	*domain.OrderEntity
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.OrderEntity.ID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (o *orderRepo) ProduceOrder(ctx context.Context, order *domain.OrderEntity) error {
	if err := o.orderMQTopic.Produce(ctx, &mqMessage{
		OrderEntity: order,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (o *orderRepo) ConsumeOrder(ctx context.Context, key string, notify func(orders *domain.OrderEntity) error) {
	o.orderMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderEntity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (o *orderRepo) ProduceOrderSaveMQ(ctx context.Context, order *domain.OrderEntity) error {
	if err := o.orderSaveMQTopic.Produce(ctx, &mqMessage{
		OrderEntity: order,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (o *orderRepo) ConsumeOrderSaveMQ(ctx context.Context, key string, notify func(order *domain.OrderEntity) error) {
	o.orderSaveMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderEntity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}
