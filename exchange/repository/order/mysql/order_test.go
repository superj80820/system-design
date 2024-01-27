package mysql

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestOrderRepo(t *testing.T) {
	ctx := context.Background()

	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(
			filepath.Join(".", "schema.sql"), //TODO: workaround
		),
	)
	assert.Nil(t, err)
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	assert.Nil(t, err)
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	assert.Nil(t, err)
	mysqlDB, err := ormKit.CreateDB(
		ormKit.UseMySQL(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			)))
	assert.Nil(t, err)

	orderRepo := CreateOrderRepo(mysqlDB)
	assert.Nil(t, orderRepo.SaveHistoryOrdersWithIgnore([]*domain.OrderEntity{
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
