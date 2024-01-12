package main

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"
	"strings"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	sequencerMemoryRepo "github.com/superj80820/system-design/exchange/repository/sequencer/memory"
	tradingMemoryRepo "github.com/superj80820/system-design/exchange/repository/trading/memory"
	"github.com/superj80820/system-design/exchange/usecase/asset"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/sequencer"
	"github.com/superj80820/system-design/exchange/usecase/trading"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type testSetup struct {
	gerUserOrders *httptransport.Server
	getOrderBook  *httptransport.Server
	cancelOrder   *httptransport.Server
	createOrder   *httptransport.Server
	getUserAssets *httptransport.Server
	getUserOrder  *httptransport.Server
}

func TestServer(t *testing.T) {
	userAID := 2
	userAIDString := strconv.Itoa(userAID)
	currencyMap := map[string]int{
		"BTC":  1,
		"USDT": 2,
	}
	testSetupFn := func() *testSetup {
		ctx := context.Background()
		tradingRepo := tradingMemoryRepo.CreateTradingRepo(ctx)
		assetRepo := assetMemoryRepo.CreateAssetRepo()
		sequencerRepo := sequencerMemoryRepo.CreateTradingSequencerRepo(ctx)

		logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
		if err != nil {
			panic(err)
		}

		matchingUseCase := matching.CreateMatchingUseCase()
		userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
		orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
		clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
		tradingUseCase := trading.CreateTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo) // TODO: orderBookDepth use function?
		tradingAsyncUseCase := trading.CreateAsyncTradingUseCase(ctx, tradingUseCase, tradingRepo, matchingUseCase, 100, logger)            //TODO:100?
		tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(sequencerRepo, tradingRepo)
		asyncTradingSequencerUseCase := sequencer.CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase, tradingRepo, sequencerRepo)

		go background.RunAsyncTradingSequencer(ctx, asyncTradingSequencerUseCase)
		go background.RunAsyncTrading(ctx, tradingAsyncUseCase)

		// TODO: workaround
		serverBeforeAddUserID := httptransport.ServerBefore(func(ctx context.Context, r *http.Request) context.Context {
			var userID int
			userIDString := r.Header.Get("user-id")
			if userIDString != "" {
				userID, _ = strconv.Atoi(userIDString)
			}
			ctx = httpKit.AddUserID(ctx, userID)
			return ctx
		})

		// TODO: workaround
		for userID := 2; userID <= 1000; userID++ {
			userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["BTC"], decimal.NewFromInt(10000000000))
			userAssetUseCase.LiabilityUserTransfer(userID, currencyMap["USDT"], decimal.NewFromInt(10000000000))
		}

		gerUserOrders := httptransport.NewServer(
			httpDelivery.MakeGetUserOrdersEndpoint(orderUserCase),
			httpDelivery.DecodeGetUserOrdersRequest,
			httpDelivery.EncodeGetUserOrdersResponse,
			serverBeforeAddUserID,
		)
		getOrderBook := httptransport.NewServer(
			httpDelivery.MakeGetOrderBookEndpoint(matchingUseCase),
			httpDelivery.DecodeGetOrderBookRequest,
			httpDelivery.EncodeGetOrderBookResponse,
			serverBeforeAddUserID,
		)
		cancelOrder := httptransport.NewServer(
			httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase),
			httpDelivery.DecodeCancelOrderRequest,
			httpDelivery.EncodeCancelOrderResponse,
			serverBeforeAddUserID,
		)
		createOrder := httptransport.NewServer(
			httpDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase),
			httpDelivery.DecodeCreateOrderRequest,
			httpDelivery.EncodeCreateOrderResponse,
			serverBeforeAddUserID,
		)
		getUserAssets := httptransport.NewServer(
			httpDelivery.MakeGetUserAssetsEndpoint(userAssetUseCase),
			httpDelivery.DecodeGetUserAssetsRequests,
			httpDelivery.EncodeGetUserAssetsResponse,
			serverBeforeAddUserID,
		)
		getUserOrder := httptransport.NewServer(
			httpDelivery.MakeGetUserOrderEndpoint(orderUserCase),
			httpDelivery.DecodeGetUserOrderRequest,
			httpDelivery.EncodeGetUserOrderResponse,
			serverBeforeAddUserID,
		)

		return &testSetup{
			gerUserOrders: gerUserOrders,
			getOrderBook:  getOrderBook,
			cancelOrder:   cancelOrder,
			createOrder:   createOrder,
			getUserAssets: getUserAssets,
			getUserOrder:  getUserOrder,
		}
	}

	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test order book",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()

				reqFn := func(payloadRawData string) *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(payloadRawData))
					r.Header.Add("user-id", userAIDString)
					return r
				}
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 2, "price": 2087.6, "quantity": 2 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2087.8, "quantity": 1 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2085.01, "quantity": 5 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 2, "price": 2088.02, "quantity": 3 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 2, "price": 2087.6, "quantity": 6 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2081.11, "quantity": 7 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2086, "quantity": 3 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2088.33, "quantity": 1 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 2, "price": 2086.54, "quantity": 2 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 2, "price": 2086.55, "quantity": 5 }`))
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": 1, "price": 2086.55, "quantity": 3 }`))

				time.Sleep(100 * time.Millisecond)

				// test all
				{
					w := httptest.NewRecorder()
					testSetup.getOrderBook.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?max_depth=10", nil))
					var orderBook domain.OrderBookEntity
					body, err := io.ReadAll(w.Body)
					assert.Nil(t, err)
					assert.Nil(t, json.Unmarshal(body, &orderBook))

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
					w := httptest.NewRecorder()
					testSetup.getOrderBook.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?max_depth=2", nil))
					var orderBook domain.OrderBookEntity
					body, err := io.ReadAll(w.Body)
					assert.Nil(t, err)
					assert.Nil(t, json.Unmarshal(body, &orderBook))

					assert.Nil(t, err)
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
			scenario: "test get user orders",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()

				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(w, r)

				time.Sleep(100 * time.Millisecond)

				w = httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				testSetup.gerUserOrders.ServeHTTP(w, r)

				var orders []*domain.OrderEntity
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &orders))
				assert.Len(t, orders, 1)
				assert.Equal(t, "2082.34", orders[0].Price.String())
			},
		},
		{
			scenario: "test cancel order",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()

				r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), r)

				time.Sleep(100 * time.Millisecond)

				w := httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				testSetup.gerUserOrders.ServeHTTP(w, r)

				var userOrders []*domain.OrderEntity
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &userOrders))
				assert.Len(t, userOrders, 1)
				assert.Equal(t, "2082.34", userOrders[0].Price.String())

				w = httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodPost, "/", nil)
				r.Header.Add("user-id", userAIDString)
				r = mux.SetURLVars(r, map[string]string{"orderID": strconv.Itoa(userOrders[0].ID)})
				testSetup.cancelOrder.ServeHTTP(w, r)

				time.Sleep(100 * time.Millisecond)

				w = httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				testSetup.gerUserOrders.ServeHTTP(w, r)

				userOrders = make([]*domain.OrderEntity, 0)
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &userOrders))
				assert.Len(t, userOrders, 0)
			},
		},
		{
			scenario: "test get user assets",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()

				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				testSetup.getUserAssets.ServeHTTP(w, r)

				userAssets := make(map[int]*domain.UserAsset)
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &userAssets))
				assert.Equal(t, "10000000000", userAssets[currencyMap["USDT"]].Available.String())
				assert.Equal(t, "10000000000", userAssets[currencyMap["BTC"]].Available.String())
			},
		},
		{
			scenario: "test get user order",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()

				r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), r)

				time.Sleep(100 * time.Millisecond)

				w := httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				testSetup.gerUserOrders.ServeHTTP(w, r)

				var userOrders []*domain.OrderEntity
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &userOrders))
				assert.Len(t, userOrders, 1)

				w = httptest.NewRecorder()
				r = httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Add("user-id", userAIDString)
				r = mux.SetURLVars(r, map[string]string{"orderID": strconv.Itoa(userOrders[0].ID)})
				testSetup.getUserOrder.ServeHTTP(w, r)

				var userOrder *domain.OrderEntity
				assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &userOrder))
				assert.Equal(t, "2082.34", userOrder.Price.String())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
