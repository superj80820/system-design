package ormandmq

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
)

func TestOrderRepo(t *testing.T) {
	ctx := context.Background()

	mysqlContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(filepath.Join(".", "schema.sql")))
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlContainer.GetURI()))
	assert.Nil(t, err)
	orderMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)

	orderRepo := CreateOrderRepo(mysqlDB, orderMQTopic)
	assert.Nil(t, orderRepo.SaveHistoryOrdersWithIgnore(1, []*domain.OrderEntity{
		{
			ID:               1,
			SequenceID:       1,
			UserID:           2,
			Price:            decimal.NewFromInt(10),
			Direction:        domain.DirectionBuy,
			Status:           domain.OrderStatusPending,
			Quantity:         decimal.NewFromInt(1),
			UnfilledQuantity: decimal.NewFromInt(1),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}))

	order, err := orderRepo.GetHistoryOrder(2, 1)
	assert.Nil(t, err)
	assert.Equal(t, "10", order.Price.String())

	orders, err := orderRepo.GetHistoryOrders(2, 100)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(orders))
	assert.Equal(t, "10", orders[0].Price.String())
}
