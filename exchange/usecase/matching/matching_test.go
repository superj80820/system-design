package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	matchingMySQLAndMQRepo "github.com/superj80820/system-design/exchange/repository/matching/mysqlandmq"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func createOrder(t *testing.T, sequenceId int, price decimal.Decimal, direction domain.DirectionEnum, quantity decimal.Decimal) *domain.OrderEntity {
	id, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	assert.Nil(t, err)

	return &domain.OrderEntity{
		ID:               id,
		SequenceID:       sequenceId,
		Price:            price,
		Direction:        direction,
		Quantity:         quantity,
		UnfilledQuantity: quantity,
		Status:           domain.OrderStatusPending,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

type testSetup struct {
	matchingUseCase domain.MatchingUseCase
	teardownFn      func()
}

func testSetupFn(ctx context.Context, t assert.TestingT) *testSetup {
	matchingMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)
	orderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)
	// mySQLContainer, err := mysqlContainer.CreateMySQL(ctx)
	// assert.Nil(t, err)
	// ormDB, err := ormKit.CreateDB(ormKit.UseMySQL(mySQLContainer.GetURI()))
	// if err != nil {
	// 	panic(err)
	// }
	matchingRepo := matchingMySQLAndMQRepo.CreateMatchingRepo(nil, matchingMQTopic, orderBookMQTopic)
	matchingOrderBookRepo := matchingMySQLAndMQRepo.CreateOrderBookRepo()
	matchingUseCase := CreateMatchingUseCase(ctx, matchingRepo, matchingOrderBookRepo, 100)

	return &testSetup{
		matchingUseCase: matchingUseCase,
		teardownFn: func() {
			// assert.Nil(t, mySQLContainer.Terminate(ctx))
		},
	}
}

func TestMatching(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test matching use case",
			fn: func(t *testing.T) {
				// buy  2082.34 1
				// sell 2087.6  2
				// buy  2087.8  1
				// buy  2085.01 5
				// sell 2088.02 3
				// sell 2087.60 6
				// buy  2081.11 7
				// buy  2086.0  3
				// buy  2088.33 1
				// sell 2086.54 2
				// sell 2086.55 5
				// buy  2086.55 3

				// 2088.02 3
				// 2087.60 6
				// 2086.55 4
				// ---------
				// 2086.55
				// ---------
				// 2086.00 3
				// 2085.01 5
				// 2082.34 1
				// 2081.11 7
				testSetup := testSetupFn(ctx, t)
				defer testSetup.teardownFn()

				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 1, decimal.NewFromFloat32(2082.34), domain.DirectionBuy, decimal.NewFromInt(1)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 2, decimal.NewFromFloat32(2087.6), domain.DirectionSell, decimal.NewFromInt(2)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 3, decimal.NewFromFloat32(2087.8), domain.DirectionBuy, decimal.NewFromInt(1)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 4, decimal.NewFromFloat32(2085.01), domain.DirectionBuy, decimal.NewFromInt(5)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 5, decimal.NewFromFloat32(2088.02), domain.DirectionSell, decimal.NewFromInt(3)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 6, decimal.NewFromFloat32(2087.60), domain.DirectionSell, decimal.NewFromInt(6)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 7, decimal.NewFromFloat32(2081.11), domain.DirectionBuy, decimal.NewFromInt(7)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 8, decimal.NewFromFloat32(2086.0), domain.DirectionBuy, decimal.NewFromInt(3)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 9, decimal.NewFromFloat32(2088.33), domain.DirectionBuy, decimal.NewFromInt(1)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 10, decimal.NewFromFloat32(2086.54), domain.DirectionSell, decimal.NewFromInt(2)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 11, decimal.NewFromFloat32(2086.55), domain.DirectionSell, decimal.NewFromInt(5)))
				testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 12, decimal.NewFromFloat32(2086.55), domain.DirectionBuy, decimal.NewFromInt(3)))
				// testSetup.matchingUseCase.NewOrder(ctx, createOrder(t, 13, decimal.NewFromFloat32(2086.55), domain.DirectionSell, decimal.NewFromInt(3)))
				assert.Equal(t, "2086.55", testSetup.matchingUseCase.GetMarketPrice().String())
				assert.Equal(t, 12, testSetup.matchingUseCase.GetSequenceID())
				currentOrderBook := testSetup.matchingUseCase.GetOrderBook(100)
				b, _ := json.MarshalIndent(currentOrderBook, "", "\t")
				fmt.Printf("york\n%s", string(b))
				currentL3OrderBook := testSetup.matchingUseCase.GetL3OrderBook(100)
				b, _ = json.MarshalIndent(currentL3OrderBook, "", "\t")
				fmt.Printf("york2\n%s", string(b))
				{
					expectedResults := []struct {
						price            string
						unfilledQuantity string
					}{
						{
							price:            "2086.55",
							unfilledQuantity: "4",
						},
						{
							price:            "2087.6",
							unfilledQuantity: "6",
						},
						{
							price:            "2088.02",
							unfilledQuantity: "3",
						},
					}
					for idx, order := range currentOrderBook.Sell {
						assert.Equal(t, expectedResults[idx].price, order.Price.String())
						assert.Equal(t, expectedResults[idx].unfilledQuantity, order.Quantity.String())
					}
				}
				{
					expectedResults := []struct {
						price            string
						unfilledQuantity string
					}{
						{
							price:            "2086",
							unfilledQuantity: "3",
						},
						{
							price:            "2085.01",
							unfilledQuantity: "5",
						},
						{
							price:            "2082.34",
							unfilledQuantity: "1",
						},
						{
							price:            "2081.11",
							unfilledQuantity: "7",
						},
					}
					for idx, order := range currentOrderBook.Buy {
						assert.Equal(t, expectedResults[idx].price, order.Price.String())
						assert.Equal(t, expectedResults[idx].unfilledQuantity, order.Quantity.String())
					}
				}
			},
		},
		// {
		// 	scenario: "test cancel order",
		// 	fn: func(t *testing.T) {
		// 		testSetup := testSetupFn(ctx, t)
		// 		defer testSetup.teardownFn()

		// 		orderOne := createOrder(1, decimal.NewFromFloat32(2082.34), domain.DirectionBuy, decimal.NewFromInt(3))
		// 		orderTwo := createOrder(2, decimal.NewFromFloat32(2087.6), domain.DirectionSell, decimal.NewFromInt(2))
		// 		testSetup.matchingUseCase.NewOrder(ctx, orderOne)
		// 		testSetup.matchingUseCase.NewOrder(ctx, orderTwo)

		// 		orderBook := testSetup.matchingUseCase.GetOrderBook(100)
		// 		assert.Equal(t, "2082.34", orderBook.Buy[0].Price.String())
		// 		assert.Equal(t, "2087.6", orderBook.Sell[0].Price.String())
		// 		testSetup.matchingUseCase.CancelOrder(time.Now(), orderOne)
		// 		orderBook = testSetup.matchingUseCase.GetOrderBook(100)
		// 		assert.Equal(t, 0, len(orderBook.Buy))
		// 		assert.Equal(t, "2087.6", orderBook.Sell[0].Price.String())
		// 	},
		// },
	}
	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
