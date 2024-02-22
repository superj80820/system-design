package matching

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	treemapKit "github.com/superj80820/system-design/kit/util/treemap"
)

func TestMatching(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test direction buy",
			fn: func(t *testing.T) {
				book := treemapKit.NewWith[*orderKey, string](directionEnum(domain.DirectionBuy).compare)
				book.Put(&orderKey{
					sequenceId: 1,
					price:      decimal.NewFromFloat32(0.1),
				}, "order1")
				book.Put(&orderKey{
					sequenceId: 2,
					price:      decimal.NewFromFloat32(0.1),
				}, "order2")
				book.Put(&orderKey{
					sequenceId: 3,
					price:      decimal.NewFromFloat32(0.3),
				}, "order3")
				expectedResults := []string{"order3", "order1", "order2"}
				var count int
				book.Each(func(key *orderKey, value string) {
					assert.Equal(t, expectedResults[count], value)
					count++
				})
				_, value := book.Min()
				assert.Equal(t, "order3", value)
				_, value = book.Max()
				assert.Equal(t, "order2", value)
			},
		},
		{
			scenario: "test direction sell",
			fn: func(t *testing.T) {
				book := treemapKit.NewWith[*orderKey, string](directionEnum(domain.DirectionSell).compare)
				book.Put(&orderKey{
					sequenceId: 1,
					price:      decimal.NewFromFloat32(0.1),
				}, "order1")
				book.Put(&orderKey{
					sequenceId: 2,
					price:      decimal.NewFromFloat32(0.1),
				}, "order2")
				book.Put(&orderKey{
					sequenceId: 3,
					price:      decimal.NewFromFloat32(0.3),
				}, "order3")
				expectedResults := []string{"order1", "order2", "order3"}
				var count int
				book.Each(func(key *orderKey, value string) {
					assert.Equal(t, expectedResults[count], value)
					count++
				})
				_, value := book.Min()
				assert.Equal(t, "order1", value)
				_, value = book.Max()
				assert.Equal(t, "order3", value)
			},
		},
		{
			scenario: "test order book",
			fn: func(t *testing.T) {
				orderBook := createOrderBook(domain.DirectionBuy)
				orderBook.add(&order{&domain.OrderEntity{
					SequenceID: 1,
					Price:      decimal.NewFromInt(100),
				}})
				firstOrder, err := orderBook.getFirst()
				assert.Nil(t, err)
				assert.True(t, firstOrder.Price.Equal(decimal.NewFromInt(100)))
				assert.Equal(t, 1, firstOrder.SequenceID)
				assert.True(t, orderBook.remove(&order{&domain.OrderEntity{
					SequenceID: 1,
					Price:      decimal.NewFromInt(100),
				}}))
				firstOrder, err = orderBook.getFirst()
				assert.ErrorIs(t, err, domain.ErrEmptyOrderBook)
				assert.Nil(t, firstOrder)
			},
		},
		{
			scenario: "test matching engine",
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
				matchEngine := &matchingUseCase{
					buyBook:     createOrderBook(domain.DirectionBuy),
					sellBook:    createOrderBook(domain.DirectionSell),
					marketPrice: decimal.Zero, // TODO: check correct?
				}
				matchEngine.NewOrder(ctx, createOrder(1, decimal.NewFromFloat32(2082.34), domain.DirectionBuy, decimal.NewFromInt(1)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(2, decimal.NewFromFloat32(2087.6), domain.DirectionSell, decimal.NewFromInt(2)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(3, decimal.NewFromFloat32(2087.8), domain.DirectionBuy, decimal.NewFromInt(1)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(4, decimal.NewFromFloat32(2085.01), domain.DirectionBuy, decimal.NewFromInt(5)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(5, decimal.NewFromFloat32(2088.02), domain.DirectionSell, decimal.NewFromInt(3)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(6, decimal.NewFromFloat32(2087.60), domain.DirectionSell, decimal.NewFromInt(6)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(7, decimal.NewFromFloat32(2081.11), domain.DirectionBuy, decimal.NewFromInt(7)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(8, decimal.NewFromFloat32(2086.0), domain.DirectionBuy, decimal.NewFromInt(3)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(9, decimal.NewFromFloat32(2088.33), domain.DirectionBuy, decimal.NewFromInt(1)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(10, decimal.NewFromFloat32(2086.54), domain.DirectionSell, decimal.NewFromInt(2)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(11, decimal.NewFromFloat32(2086.55), domain.DirectionSell, decimal.NewFromInt(5)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(12, decimal.NewFromFloat32(2086.55), domain.DirectionBuy, decimal.NewFromInt(3)).OrderEntity)
				assert.Equal(t, "2086.55", matchEngine.marketPrice.String())
				assert.Equal(t, 12, matchEngine.sequenceID)
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
					var count int
					matchEngine.sellBook.book.Each(func(key *orderKey, value *order) {
						assert.Equal(t, expectedResults[count].price, value.Price.String())
						assert.Equal(t, expectedResults[count].unfilledQuantity, value.UnfilledQuantity.String())
						count++
					})
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
					var count int
					matchEngine.buyBook.book.Each(func(key *orderKey, value *order) {
						assert.Equal(t, expectedResults[count].price, value.Price.String())
						assert.Equal(t, expectedResults[count].unfilledQuantity, value.UnfilledQuantity.String())
						count++
					})
				}
			},
		},
		{
			scenario: "test cancel order",
			fn: func(t *testing.T) {
				matchEngine := &matchingUseCase{
					buyBook:     createOrderBook(domain.DirectionBuy),
					sellBook:    createOrderBook(domain.DirectionSell),
					marketPrice: decimal.Zero, // TODO: check correct?
				}
				matchEngine.NewOrder(ctx, createOrder(1, decimal.NewFromFloat32(2082.34), domain.DirectionBuy, decimal.NewFromInt(3)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(2, decimal.NewFromFloat32(2087.6), domain.DirectionSell, decimal.NewFromInt(2)).OrderEntity)
				matchEngine.NewOrder(ctx, createOrder(3, decimal.NewFromFloat32(2087.8), domain.DirectionBuy, decimal.NewFromInt(1)).OrderEntity)

				// test fully canceled
				buyOrder, found := matchEngine.buyBook.book.Get(&orderKey{sequenceId: 1, price: decimal.NewFromFloat32(2082.34)})
				assert.True(t, found)
				assert.Equal(t, "3", buyOrder.UnfilledQuantity.String())
				assert.Equal(t, domain.OrderStatusPending, buyOrder.Status)
				matchEngine.CancelOrder(time.Now(), buyOrder.OrderEntity)
				assert.Equal(t, "3", buyOrder.UnfilledQuantity.String())
				assert.Equal(t, domain.OrderStatusFullyCanceled, buyOrder.Status)

				// test partial canceled
				sellOrder, found := matchEngine.sellBook.book.Get(&orderKey{sequenceId: 2, price: decimal.NewFromFloat32(2087.6)})
				assert.True(t, found)
				assert.Equal(t, "1", sellOrder.UnfilledQuantity.String())
				assert.Equal(t, domain.OrderStatusPartialFilled, sellOrder.Status)
				matchEngine.CancelOrder(time.Now(), sellOrder.OrderEntity)
				assert.Equal(t, "1", sellOrder.UnfilledQuantity.String())
				assert.Equal(t, domain.OrderStatusPartialCanceled, sellOrder.Status)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
