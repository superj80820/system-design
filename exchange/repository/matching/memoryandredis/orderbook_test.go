package memoryandredis

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	testingRedisKit "github.com/superj80820/system-design/kit/testing/redis/container"
)

type testSetup struct {
	l3OrderBookSample *domain.OrderBookL3Entity
	orderBookRepo     domain.MatchingOrderBookRepo
	teardownFn        func()
}

func testSetupFn(ctx context.Context, t assert.TestingT) *testSetup {
	redisContainer, err := testingRedisKit.CreateRedis(ctx)
	assert.Nil(t, err)

	redisCache, err := redisKit.CreateCache(redisContainer.GetURI(), "", 0)
	assert.Nil(t, err)

	messageChannelBuffer := 1000
	messageCollectDuration := 100 * time.Millisecond
	orderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	l1OrderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	l2OrderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	l3OrderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	orderBookRepo := CreateOrderBookRepo(redisCache, orderBookMQTopic, l1OrderBookMQTopic, l2OrderBookMQTopic, l3OrderBookMQTopic)

	return &testSetup{
		l3OrderBookSample: &domain.OrderBookL3Entity{
			SequenceID: 5,
			Price:      decimal.NewFromInt(100),
			Sell: []*domain.OrderBookL3ItemEntity{
				{
					Price:    decimal.NewFromInt(101),
					Quantity: decimal.NewFromInt(150),
					Orders: []*domain.OrderL3Entity{
						{
							SequenceID: 1,
							OrderID:    1,
							Quantity:   decimal.NewFromInt(30),
						},
						{
							SequenceID: 2,
							OrderID:    2,
							Quantity:   decimal.NewFromInt(120),
						},
					},
				},
			},
			Buy: []*domain.OrderBookL3ItemEntity{
				{
					Price:    decimal.NewFromInt(99),
					Quantity: decimal.NewFromInt(10),
					Orders: []*domain.OrderL3Entity{
						{
							SequenceID: 3,
							OrderID:    3,
							Quantity:   decimal.NewFromInt(10),
						},
					},
				},
				{
					Price:    decimal.NewFromInt(90),
					Quantity: decimal.NewFromInt(20),
					Orders: []*domain.OrderL3Entity{
						{
							SequenceID: 4,
							OrderID:    4,
							Quantity:   decimal.NewFromInt(20),
						},
					},
				},
			},
		},
		orderBookRepo: orderBookRepo,
		teardownFn: func() {
			assert.Nil(t, redisContainer.Terminate(ctx))
		},
	}
}

func TestOrderBookRepo(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test less sequence should not save",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.orderBookRepo.SaveHistoryL3OrderBook(ctx, testSetup.l3OrderBookSample))

				l3OrderBook, err := testSetup.orderBookRepo.GetHistoryL3OrderBook(ctx, -1)
				assert.Nil(t, err)
				assert.Equal(t, testSetup.l3OrderBookSample, l3OrderBook)

				// should not save because sequence id is less
				assert.Nil(t, testSetup.orderBookRepo.SaveHistoryL3OrderBook(ctx, &domain.OrderBookL3Entity{
					SequenceID: 2,
					Price:      decimal.NewFromInt(101),
					Sell:       []*domain.OrderBookL3ItemEntity{},
					Buy:        []*domain.OrderBookL3ItemEntity{},
				}))

				l3OrderBook, err = testSetup.orderBookRepo.GetHistoryL3OrderBook(ctx, -1)
				assert.Nil(t, err)
				assert.Equal(t, testSetup.l3OrderBookSample, l3OrderBook)
			},
		},
		{
			scenario: "test l1 order book",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				expected := &domain.OrderBookL1Entity{
					SequenceID: 5,
					Price:      decimal.NewFromInt(100),
					BestAsk: &domain.OrderBookL1ItemEntity{
						Price:    decimal.NewFromInt(101),
						Quantity: decimal.NewFromInt(150),
					},
					BestBid: &domain.OrderBookL1ItemEntity{
						Price:    decimal.NewFromInt(99),
						Quantity: decimal.NewFromInt(10),
					},
				}

				l1OrderBook, err := testSetup.orderBookRepo.SaveHistoryL1OrderBookByL3OrderBook(ctx, testSetup.l3OrderBookSample)
				assert.Nil(t, err)
				assert.Equal(t, expected, l1OrderBook)

				l1OrderBook, err = testSetup.orderBookRepo.GetHistoryL1OrderBook(ctx)
				assert.Nil(t, err)
				assert.Equal(t, expected, l1OrderBook)
			},
		},
		{
			scenario: "test l2 order book",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				expected := &domain.OrderBookL2Entity{
					SequenceID: 5,
					Price:      decimal.NewFromInt(100),
					Sell: []*domain.OrderBookL2ItemEntity{
						{
							Price:    decimal.NewFromInt(101),
							Quantity: decimal.NewFromInt(150),
						},
					},
					Buy: []*domain.OrderBookL2ItemEntity{
						{
							Price:    decimal.NewFromInt(99),
							Quantity: decimal.NewFromInt(10),
						},
						{
							Price:    decimal.NewFromInt(90),
							Quantity: decimal.NewFromInt(20),
						},
					},
				}

				l2OrderBook, err := testSetup.orderBookRepo.SaveHistoryL2OrderBookByL3OrderBook(ctx, testSetup.l3OrderBookSample)
				assert.Nil(t, err)
				assert.Equal(t, expected, l2OrderBook)

				l2OrderBook, err = testSetup.orderBookRepo.GetHistoryL2OrderBook(ctx, -1)
				assert.Nil(t, err)
				assert.Equal(t, expected, l2OrderBook)

				l2OrderBook, err = testSetup.orderBookRepo.GetHistoryL2OrderBook(ctx, 100)
				assert.Nil(t, err)
				assert.Equal(t, expected, l2OrderBook)

				expected = &domain.OrderBookL2Entity{
					SequenceID: 5,
					Price:      decimal.NewFromInt(100),
					Sell: []*domain.OrderBookL2ItemEntity{
						{
							Price:    decimal.NewFromInt(101),
							Quantity: decimal.NewFromInt(150),
						},
					},
					Buy: []*domain.OrderBookL2ItemEntity{
						{
							Price:    decimal.NewFromInt(99),
							Quantity: decimal.NewFromInt(10),
						},
					},
				}

				l2OrderBook, err = testSetup.orderBookRepo.GetHistoryL2OrderBook(ctx, 1)
				assert.Nil(t, err)
				assert.Equal(t, expected, l2OrderBook)
			},
		},
		{
			scenario: "test l3 order book",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.orderBookRepo.SaveHistoryL3OrderBook(ctx, testSetup.l3OrderBookSample))

				l3OrderBook, err := testSetup.orderBookRepo.GetHistoryL3OrderBook(ctx, -1)
				assert.Nil(t, err)
				assert.Equal(t, testSetup.l3OrderBookSample, l3OrderBook)

				l3OrderBook, err = testSetup.orderBookRepo.GetHistoryL3OrderBook(ctx, 100)
				assert.Nil(t, err)
				assert.Equal(t, testSetup.l3OrderBookSample, l3OrderBook)

				expected := &domain.OrderBookL3Entity{
					SequenceID: 5,
					Price:      decimal.NewFromInt(100),
					Sell: []*domain.OrderBookL3ItemEntity{
						{
							Price:    decimal.NewFromInt(101),
							Quantity: decimal.NewFromInt(150),
							Orders: []*domain.OrderL3Entity{
								{
									SequenceID: 1,
									OrderID:    1,
									Quantity:   decimal.NewFromInt(30),
								},
								{
									SequenceID: 2,
									OrderID:    2,
									Quantity:   decimal.NewFromInt(120),
								},
							},
						},
					},
					Buy: []*domain.OrderBookL3ItemEntity{
						{
							Price:    decimal.NewFromInt(99),
							Quantity: decimal.NewFromInt(10),
							Orders: []*domain.OrderL3Entity{
								{
									SequenceID: 3,
									OrderID:    3,
									Quantity:   decimal.NewFromInt(10),
								},
							},
						},
					},
				}

				l3OrderBook, err = testSetup.orderBookRepo.GetHistoryL3OrderBook(ctx, 1)
				assert.Nil(t, err)
				assert.Equal(t, expected, l3OrderBook)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
