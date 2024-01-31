package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
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
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	orderMysqlReop "github.com/superj80820/system-design/exchange/repository/order/mysql"
	quotationRepoMySQLAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/mysqlandredis"
	sequencerKafkaAndMySQLRepo "github.com/superj80820/system-design/exchange/repository/sequencer/kafkaandmysql"
	tradingMySQLAndMongoRepo "github.com/superj80820/system-design/exchange/repository/trading/mysqlandmongo"
	"github.com/superj80820/system-design/exchange/usecase/asset"
	candleUseCaseLib "github.com/superj80820/system-design/exchange/usecase/candle"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/quotation"
	"github.com/superj80820/system-design/exchange/usecase/sequencer"
	"github.com/superj80820/system-design/exchange/usecase/trading"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	redisKit "github.com/superj80820/system-design/kit/redis"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type testSetup struct {
	gerUserOrders *httptransport.Server
	getOrderBook  *httptransport.Server
	cancelOrder   *httptransport.Server
	createOrder   *httptransport.Server
	getUserAssets *httptransport.Server
	getUserOrder  *httptransport.Server
	teardownFn    func()
}

var (
	userAID       = 2
	userAIDString = strconv.Itoa(userAID)
	currencyMap   = map[string]int{
		"BTC":  1,
		"USDT": 2,
	}
)

func testSetupFn() *testSetup {
	ctx := context.Background()

	quotationSchemaSQL, err := os.ReadFile("../../exchange/repository/quotation/mysqlandredis/schema.sql")
	if err != nil {
		panic(err)
	}
	tradingSchemaSQL, err := os.ReadFile("../../exchange/repository/trading/mysqlandmongo/schema.sql")
	if err != nil {
		panic(err)
	}
	orderSchemaSQL, err := os.ReadFile("../../exchange/repository/order/mysql/schema.sql")
	if err != nil {
		panic(err)
	}
	candleSchemaSQL, err := os.ReadFile("../../exchange/repository/candle/schema.sql")
	if err != nil {
		panic(err)
	}
	sequencerSchemaSQL, err := os.ReadFile("../../exchange/repository/sequencer/kafkaandmysql/schema.sql")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./schema.sql", []byte(string(quotationSchemaSQL)+"\n"+string(tradingSchemaSQL)+"\n"+string(candleSchemaSQL)+"\n"+string(orderSchemaSQL)+"\n"+string(sequencerSchemaSQL)), 0644)
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
	err = os.Remove("./schema.sql")
	if err != nil {
		panic(err)
	}
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("docker.io/redis:7"),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		panic(err)
	}
	redisCache, err := redisKit.CreateCache(redisHost+":"+redisPort.Port(), "", 0)
	if err != nil {
		panic(err)
	}

	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		panic(err)
	}
	mongoHost, err := mongodbContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	mongoPort, err := mongodbContainer.MappedPort(ctx, "27017")
	if err != nil {
		panic(err)
	}
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
	if err != nil {
		panic(err)
	}

	eventsCollection := mongoDB.Database("exchange").Collection("events")
	eventsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"sequence_id": -1,
		},
		Options: options.Index().SetUnique(true),
	})

	sequenceMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	tradingEventMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	tradingResultMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	if err != nil {
		panic(err)
	}

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMQTopic, mysqlDB)
	if err != nil {
		panic(err)
	}
	candleRepo := candleRepoRedis.CreateCandleRepo(mysqlDB, redisCache)
	quotationRepo := quotationRepoMySQLAndRedis.CreateQuotationRepo(mysqlDB, redisCache)
	orderRepo := orderMysqlReop.CreateOrderRepo(mysqlDB)

	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(ctx, tradingRepo, quotationRepo, 100) // TODO: 100?
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, tradingRepo, candleRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, tradingRepo, orderRepo, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
	syncTradingUseCase := trading.CreateSyncTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase)
	tradingUseCase := trading.CreateTradingUseCase(ctx, tradingRepo, orderUserCase, userAssetUseCase, syncTradingUseCase, matchingUseCase, 100, logger) // TODO: orderBookDepth use function? 100?
	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(logger, sequencerRepo, tradingRepo, tradingUseCase, 3000, 500*time.Millisecond)

	go func() {
		if err := background.RunAsyncTradingSequencer(ctx, tradingSequencerUseCase, quotationUseCase, candleUseCase, orderUserCase, tradingUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()

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
		teardownFn: func() {
			err := mysqlContainer.Terminate(ctx)
			if err != nil {
				panic(err)
			}
			err = redisContainer.Terminate(ctx)
			if err != nil {
				panic(err)
			}
			err = mongodbContainer.Terminate(ctx)
			if err != nil {
				panic(err)
			}
		},
	}
}

func TestServer(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test order book",
			fn: func(t *testing.T) {
				testSetup := testSetupFn()
				defer testSetup.teardownFn()

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

				time.Sleep(1000 * time.Millisecond)

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
				defer testSetup.teardownFn()

				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(w, r)

				time.Sleep(1000 * time.Millisecond)

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
				defer testSetup.teardownFn()

				r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), r)

				time.Sleep(1000 * time.Millisecond)

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

				time.Sleep(1000 * time.Millisecond)

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
				defer testSetup.teardownFn()

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
				defer testSetup.teardownFn()

				r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{ "direction": 1, "price": 2082.34, "quantity": 1 }`))
				r.Header.Add("user-id", userAIDString)
				testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), r)

				time.Sleep(1000 * time.Millisecond)

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

func BenchmarkServer(b *testing.B) {
	testSetup := testSetupFn()
	defer testSetup.teardownFn()

	reqFn := func(payloadRawData string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(payloadRawData))
		r.Header.Add("user-id", userAIDString)
		return r
	}
	direction := []domain.DirectionEnum{domain.DirectionBuy, domain.DirectionSell}

	for i := 0; i < b.N; i++ {
		randNum := rand.Float64() * 10
		testSetup.createOrder.ServeHTTP(httptest.NewRecorder(), reqFn(`{ "direction": `+strconv.Itoa(int(direction[i%2]))+`, "price": `+strconv.FormatFloat(randNum, 'f', -1, 64)+`, "quantity": 3}`))
	}
}
