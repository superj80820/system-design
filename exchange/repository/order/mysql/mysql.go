package mysql

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"gorm.io/gorm/clause"
)

type orderRepo struct {
	orm *ormKit.DB
}

type orderEntityDB struct {
	*domain.OrderEntity
}

func (*orderEntityDB) TableName() string {
	return "orders"
}

func CreateOrderRepo(orm *ormKit.DB) domain.OrderRepo {
	return &orderRepo{
		orm: orm,
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
