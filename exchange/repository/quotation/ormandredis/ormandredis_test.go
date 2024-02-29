package ormandredis

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	redisKit "github.com/superj80820/system-design/kit/redis"
	testingPostgresKit "github.com/superj80820/system-design/kit/testing/postgres/container"
	testingRedisKit "github.com/superj80820/system-design/kit/testing/redis/container"
)

func TestQuotationRepo(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := testingRedisKit.CreateRedis(ctx)
	assert.Nil(t, err)
	postgres, err := testingPostgresKit.CreatePostgres(ctx, "postgres.schema.sql")
	assert.Nil(t, err)

	redisCache, err := redisKit.CreateCache(redisContainer.GetURI(), "", 0)
	assert.Nil(t, err)
	postgresDB, err := ormKit.CreateDB(ormKit.UsePostgres(postgres.GetURI()))
	assert.Nil(t, err)
	tickMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)

	quotationRepo := CreateQuotationRepo(postgresDB, redisCache, tickMQTopic)
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
