package mysqlandredis

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
	redisKit "github.com/superj80820/system-design/kit/redis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestQuotationRepo(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("docker.io/redis:7"),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	redisHost, err := redisContainer.Host(ctx)
	assert.Nil(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	assert.Nil(t, err)
	redisCache, err := redisKit.CreateCache(redisHost+":"+redisPort.Port(), "", 0)
	assert.Nil(t, err)

	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(filepath.Join(".", "schema.sql")),
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

	quotationRepo := CreateQuotationRepo(mysqlDB, redisCache)
	timestamp := time.Unix(1706558738, 0)
	assert.Nil(t, quotationRepo.SaveTickStrings(ctx, 1, []*domain.TickEntity{
		{
			ID:             1,
			SequenceID:     1,
			TakerOrderID:   1,
			MakerOrderID:   2,
			TakerDirection: domain.DirectionBuy,
			Price:          decimal.NewFromInt(2),
			Quantity:       decimal.NewFromInt(3),
			CreatedAt:      timestamp,
		},
		{
			ID:             2,
			SequenceID:     2,
			TakerOrderID:   2,
			MakerOrderID:   3,
			TakerDirection: domain.DirectionSell,
			Price:          decimal.NewFromInt(5),
			Quantity:       decimal.NewFromInt(6),
			CreatedAt:      timestamp,
		},
		{
			ID:             3,
			SequenceID:     3,
			TakerOrderID:   5,
			MakerOrderID:   6,
			TakerDirection: domain.DirectionSell,
			Price:          decimal.NewFromInt(6),
			Quantity:       decimal.NewFromInt(7),
			CreatedAt:      timestamp,
		},
		{
			ID:             5,
			SequenceID:     5,
			TakerOrderID:   6,
			MakerOrderID:   7,
			TakerDirection: domain.DirectionSell,
			Price:          decimal.NewFromInt(7),
			Quantity:       decimal.NewFromInt(8),
			CreatedAt:      timestamp,
		},
	}))

	ticks, err := quotationRepo.GetTickStrings(ctx, 1, 3)
	assert.Nil(t, err)

	expects := []string{"[1706558738000,2,5,6]", "[1706558738000,2,6,7]", "[1706558738000,2,7,8]"}
	for idx, tick := range ticks {
		assert.Equal(t, expects[idx], tick)
	}
}
