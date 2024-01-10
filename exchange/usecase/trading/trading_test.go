package trading

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"

	"github.com/superj80820/system-design/exchange/usecase/asset"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/trading/mocks"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func TestPrevious(t *testing.T) { // TODO: test

}

func TestTrading(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test order book",
			fn: func(t *testing.T) {

				userID := 2
				currencyMap := map[string]int{
					"BTC":  1,
					"USDT": 2,
				}
				tradingRepo := new(mocks.TradingRepo)
				assetRepo := assetMemoryRepo.CreateAssetRepo()
				matchingUseCase := matching.CreateMatchingUseCase()
				userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
				orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
				clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])

				uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
				assert.Nil(t, err)

				tradingUseCase := CreateTradingUseCase(context.Background(), matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo, 100)
				createTradingEvent := func(userID, previousID, sequenceID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) *domain.TradingEvent {
					return &domain.TradingEvent{
						EventType:  domain.TradingEventCreateOrderType,
						SequenceID: sequenceID,
						PreviousID: previousID,
						UniqueID:   int(uniqueIDGenerate.Generate().GetInt64()),

						OrderRequestEvent: &domain.OrderRequestEvent{
							UserID:    userID,
							Direction: direction,
							Price:     price,
							Quantity:  quantity,
						},

						CreatedAt: time.Now(),
					}
				}
				assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["USDT"], decimal.NewFromInt(1000000)))
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
				err = tradingUseCase.ProcessMessages([]*domain.TradingEvent{
					createTradingEvent(userID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)),
					createTradingEvent(userID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)),
					createTradingEvent(userID, 2, 3, domain.DirectionBuy, decimal.NewFromFloat(2087.8), decimal.NewFromInt(1)),
					createTradingEvent(userID, 3, 4, domain.DirectionBuy, decimal.NewFromFloat(2085.01), decimal.NewFromInt(5)),
					createTradingEvent(userID, 4, 5, domain.DirectionSell, decimal.NewFromFloat(2088.02), decimal.NewFromInt(3)),
					createTradingEvent(userID, 5, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)),
					createTradingEvent(userID, 6, 7, domain.DirectionBuy, decimal.NewFromFloat(2081.11), decimal.NewFromInt(7)),
					createTradingEvent(userID, 7, 8, domain.DirectionBuy, decimal.NewFromFloat(2086.0), decimal.NewFromInt(3)),
					createTradingEvent(userID, 8, 9, domain.DirectionBuy, decimal.NewFromFloat(2088.33), decimal.NewFromInt(1)),
					createTradingEvent(userID, 9, 10, domain.DirectionSell, decimal.NewFromFloat(2086.54), decimal.NewFromInt(2)),
					createTradingEvent(userID, 10, 11, domain.DirectionSell, decimal.NewFromFloat(2086.55), decimal.NewFromInt(5)),
					createTradingEvent(userID, 11, 12, domain.DirectionBuy, decimal.NewFromFloat(2086.55), decimal.NewFromInt(3)),
				})
				assert.Nil(t, err)

				// test all
				{
					orderBook := matchingUseCase.GetOrderBook(10)
					buyExpected := []struct {
						price    string
						quantity string
					}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}, {price: "2082.34", quantity: "1"}, {price: "2081.11", quantity: "7"}}
					for idx, val := range orderBook.Buy {
						assert.Equal(t, buyExpected[idx].price, val.Price.String())
						assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
					}
					sellExpected := []struct {
						price    string
						quantity string
					}{{price: "2086.55", quantity: "4"}, {price: "2087.6", quantity: "6"}, {price: "2088.02", quantity: "3"}}
					for idx, val := range orderBook.Sell {
						assert.Equal(t, sellExpected[idx].price, val.Price.String())
						assert.Equal(t, sellExpected[idx].quantity, val.Quantity.String())
					}
				}

				// test with max depth
				{
					orderBook := matchingUseCase.GetOrderBook(2)
					assert.Equal(t, 2, len(orderBook.Buy))
					assert.Equal(t, 2, len(orderBook.Sell))
					buyExpected := []struct {
						price    string
						quantity string
					}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}}
					for idx, val := range orderBook.Buy {
						assert.Equal(t, buyExpected[idx].price, val.Price.String())
						assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
					}
				}
			},
		},
		{
			scenario: "test get duplicate event",
			fn: func(t *testing.T) {

				userID := 2
				currencyMap := map[string]int{
					"BTC":  1,
					"USDT": 2,
				}
				tradingRepo := new(mocks.TradingRepo)
				assetRepo := assetMemoryRepo.CreateAssetRepo()
				matchingUseCase := matching.CreateMatchingUseCase()
				userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
				orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
				clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])

				uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
				assert.Nil(t, err)

				tradingUseCase := CreateTradingUseCase(context.Background(), matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo, 100)
				createTradingEvent := func(userID, previousID, sequenceID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) *domain.TradingEvent {
					return &domain.TradingEvent{
						EventType:  domain.TradingEventCreateOrderType,
						SequenceID: sequenceID,
						PreviousID: previousID,
						UniqueID:   int(uniqueIDGenerate.Generate().GetInt64()),

						OrderRequestEvent: &domain.OrderRequestEvent{
							UserID:    userID,
							Direction: direction,
							Price:     price,
							Quantity:  quantity,
						},

						CreatedAt: time.Now(),
					}
				}
				assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["USDT"], decimal.NewFromInt(1000000)))
				err = tradingUseCase.ProcessMessages([]*domain.TradingEvent{
					createTradingEvent(userID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)),
					createTradingEvent(userID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)),
					createTradingEvent(userID, 2, 3, domain.DirectionBuy, decimal.NewFromFloat(2087.8), decimal.NewFromInt(1)),
				})
				assert.Nil(t, err)

				// test get duplicate event
				{
					err := tradingUseCase.ProcessMessages([]*domain.TradingEvent{
						createTradingEvent(userID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)),
					})
					assert.ErrorIs(t, err, domain.ErrGetDuplicateEvent)
				}

				// test miss event
				{
					err := tradingUseCase.ProcessMessages([]*domain.TradingEvent{
						createTradingEvent(userID, 5, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)),
					})
					assert.ErrorIs(t, err, domain.ErrMissEvent)
				}

				// TODO: test think maybe no need previous
				// test previous id not correct
				{
					err := tradingUseCase.ProcessMessages([]*domain.TradingEvent{
						createTradingEvent(userID, 1, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)),
					})
					assert.ErrorIs(t, err, domain.ErrPreviousIDNotCorrect)
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
