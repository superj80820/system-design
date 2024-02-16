package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	authUseCase "github.com/superj80820/system-design/auth/usecase"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	httpGitbitexDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex"
	wsDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex/ws"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	matchingMySQLAndMQRepo "github.com/superj80820/system-design/exchange/repository/matching/mysqlandmq"
	orderMysqlReop "github.com/superj80820/system-design/exchange/repository/order/mysql"
	quotationRepoMySQLAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/mysqlandredis"
	sequencerKafkaAndMySQLRepo "github.com/superj80820/system-design/exchange/repository/sequencer/kafkaandmysql"
	tradingMySQLAndMongoRepo "github.com/superj80820/system-design/exchange/repository/trading/mysqlandmongo"
	"github.com/superj80820/system-design/exchange/usecase/asset"
	candleUseCaseLib "github.com/superj80820/system-design/exchange/usecase/candle"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/currency"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/quotation"
	"github.com/superj80820/system-design/exchange/usecase/sequencer"
	"github.com/superj80820/system-design/exchange/usecase/trading"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	wsMiddleware "github.com/superj80820/system-design/kit/http/websocket/middleware"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type testSetup struct {
	wsServer               *httptest.Server
	cancelOrderServer      *httptransport.Server
	createOrderServer      *httptransport.Server
	getHistoryOrdersServer *httptransport.Server
	getProductsServer      *httptransport.Server
	getCandlesServer       *httptransport.Server
	getUserOrdersServer    *httptransport.Server
	getUserAssetsServer    *httptransport.Server
	createUserServer       *httptransport.Server
	userLoginServer        *httptransport.Server
	teardownFn             func()
}

var (
	userAEmail    = "a@gmail.com"
	userAPassword = "asdfasdf"
	currencyMap   = map[string]int{
		"BTC":  1,
		"USDT": 2,
	}
)

func testSetupFn(t assert.TestingT) *testSetup {
	currencyProduct := domain.CurrencyProduct{
		ID:             "BTC-USDT",
		BaseCurrency:   "BTC",
		QuoteCurrency:  "USDT",
		QuoteIncrement: "0.0",
		QuoteMaxSize:   decimal.NewFromInt(100000000).String(),
		QuoteMinSize:   decimal.NewFromFloat(0.000001).String(),
		BaseMaxSize:    decimal.NewFromInt(100000000).String(),
		BaseMinSize:    decimal.NewFromFloat(0.000001).String(),
		BaseScale:      6,
		QuoteScale:     2,
	}

	ctx := context.Background()

	quotationSchemaSQL, err := os.ReadFile("../../exchange/repository/quotation/mysqlandredis/schema.sql")
	assert.Nil(t, err)
	tradingSchemaSQL, err := os.ReadFile("../../exchange/repository/trading/mysqlandmongo/schema.sql")
	assert.Nil(t, err)
	orderSchemaSQL, err := os.ReadFile("../../exchange/repository/order/mysql/schema.sql")
	assert.Nil(t, err)
	candleSchemaSQL, err := os.ReadFile("../../exchange/repository/candle/schema.sql")
	assert.Nil(t, err)
	sequencerSchemaSQL, err := os.ReadFile("../../exchange/repository/sequencer/kafkaandmysql/schema.sql")
	assert.Nil(t, err)
	authSchemaSQL, err := os.ReadFile("../../auth/repository/schema.sql")
	assert.Nil(t, err)
	err = os.WriteFile("./schema.sql", []byte(string(quotationSchemaSQL)+"\n"+string(tradingSchemaSQL)+"\n"+string(candleSchemaSQL)+"\n"+string(orderSchemaSQL)+"\n"+string(sequencerSchemaSQL)+"\n"+string(authSchemaSQL)), 0644)
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
	err = os.Remove("./schema.sql")
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

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("docker.io/redis:7"),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	assert.Nil(t, err)
	redisHost, err := redisContainer.Host(ctx)
	assert.Nil(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	assert.Nil(t, err)
	redisCache, err := redisKit.CreateCache(redisHost+":"+redisPort.Port(), "", 0)
	assert.Nil(t, err)

	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	assert.Nil(t, err)
	mongoHost, err := mongodbContainer.Host(ctx)
	assert.Nil(t, err)
	mongoPort, err := mongodbContainer.MappedPort(ctx, "27017")
	assert.Nil(t, err)
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
	assert.Nil(t, err)

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
	assetMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	orderMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	orderSaveMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	candleMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	candleSaveMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100000)
	tickMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	ickSaveMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	matchingSaveMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	matchingMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	orderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	assert.Nil(t, err)
	tracer := traceKit.CreateNoOpTracer()

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	assetRepo := assetMemoryRepo.CreateAssetRepo(assetMQTopic)
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMQTopic, mysqlDB)
	assert.Nil(t, err)
	orderRepo := orderMysqlReop.CreateOrderRepo(mysqlDB, orderMQTopic, orderSaveMQTopic)
	candleRepo := candleRepoRedis.CreateCandleRepo(mysqlDB, redisCache, candleMQTopic, candleSaveMQTopic)
	quotationRepo := quotationRepoMySQLAndRedis.CreateQuotationRepo(mysqlDB, redisCache, tickMQTopic, ickSaveMQTopic)
	matchingRepo := matchingMySQLAndMQRepo.CreateMatchingRepo(mysqlDB, matchingSaveMQTopic, matchingMQTopic, orderBookMQTopic)

	currencyUseCase := currency.CreateCurrencyUseCase(&currencyProduct)
	matchingUseCase := matching.CreateMatchingUseCase(ctx, matchingRepo, quotationRepo, orderRepo, candleRepo, 100) // TODO: 100?
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo, tradingRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(ctx, tradingRepo, quotationRepo, 100) // TODO: 100?
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, candleRepo)
	orderUseCase := order.CreateOrderUseCase(userAssetUseCase, tradingRepo, orderRepo)
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUseCase)
	syncTradingUseCase := trading.CreateSyncTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUseCase, clearingUseCase)
	tradingUseCase := trading.CreateTradingUseCase(ctx, tradingRepo, matchingRepo, quotationRepo, candleRepo, orderRepo, assetRepo, orderUseCase, userAssetUseCase, syncTradingUseCase, matchingUseCase, currencyUseCase, 100, logger) // TODO: orderBookDepth use function? 100?
	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(logger, sequencerRepo, tradingRepo, tradingUseCase, 3000, 500*time.Millisecond)
	accountUseCase, err := authUseCase.CreateAccountUseCase(mysqlDB, logger)
	assert.Nil(t, err)
	authUserUseCase, err := authUseCase.CreateAuthUseCase(mysqlDB, logger)
	assert.Nil(t, err)

	go func() {
		if err := background.RunAsyncTradingSequencer(ctx, tradingSequencerUseCase, quotationUseCase, candleUseCase, orderUseCase, tradingUseCase, matchingUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()

	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		return authUserUseCase.Verify(token)
	})

	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer, httpKit.OptionSetCookieAccessTokenKey("accessToken"))),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}

	cancelOrderServer := httptransport.NewServer(
		authMiddleware(httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase)),
		httpDelivery.DecodeCancelOrderRequest,
		httpDelivery.EncodeCancelOrderResponse,
		options...,
	)
	createOrderServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase)),
		httpGitbitexDelivery.DecodeCreateOrderRequest,
		httpGitbitexDelivery.EncodeCreateOrderResponse,
		options...,
	)
	getHistoryOrdersServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakerGetHistoryOrdersEndpoint(orderUseCase, currencyUseCase)),
		httpGitbitexDelivery.DecodeGetHistoryOrdersRequest,
		httpGitbitexDelivery.EncodeGetHistoryOrdersResponse,
		options...,
	)
	getProductsServer := httptransport.NewServer(
		httpGitbitexDelivery.MakeGetProductsEndpoint(currencyUseCase),
		httpGitbitexDelivery.DecodeGetProductsRequest,
		httpGitbitexDelivery.EncodeGetProductsResponse,
		options...,
	)
	getCandlesServer := httptransport.NewServer(
		httpGitbitexDelivery.MakeGetCandleEndpoint(candleUseCase, currencyUseCase),
		httpGitbitexDelivery.DecodeGetCandlesRequest,
		httpGitbitexDelivery.EncodeGetCandlesResponse,
		options...,
	)
	getUserOrdersServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakeGetAccountOrdersEndpoint(orderUseCase, currencyUseCase)),
		httpGitbitexDelivery.DecodeGetAccountOrdersRequest,
		httpGitbitexDelivery.EncodeGetAccountOrdersResponse,
		options...,
	)
	getUserAssetsServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakeGetAccountAssetsEndpoint(userAssetUseCase, currencyUseCase)),
		httpGitbitexDelivery.DecodeGetAccountAssetsRequest,
		httpGitbitexDelivery.EncodeGetAccountAssetsResponse,
		options...,
	)
	createUserServer := httptransport.NewServer(
		httpGitbitexDelivery.MakeAccountRegisterEndpoint(accountUseCase, tradingSequencerUseCase, currencyUseCase),
		httpGitbitexDelivery.DecodeAccountRegisterRequest,
		httpGitbitexDelivery.EncodeAccountRegisterResponse,
		options...,
	)
	userLoginServer := httptransport.NewServer(
		httpGitbitexDelivery.MakeAuthLoginEndpoint(authUserUseCase),
		httpGitbitexDelivery.DecodeAuthLoginRequest,
		httpGitbitexDelivery.EncodeAuthResponse,
		options...,
	)
	wsServer := httptest.NewServer(wsTransport.NewServer(
		wsMiddleware.CreateAuth[domain.TradingNotifyRequest, any](func(token string) (int64, error) {
			return authUserUseCase.Verify(token)
		})(wsDelivery.MakeExchangeEndpoint(tradingUseCase)),
		wsDelivery.DecodeStreamExchangeRequest,
		wsKit.JsonEncodeResponse[any],
		wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
		wsTransport.ServerBefore(httpKit.CustomBeforeCtx(tracer, httpKit.OptionSetCookieAccessTokenKey("accessToken"))), // TODO
		wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()),                                                   // TODO: maybe to default
	))

	return &testSetup{
		wsServer:               wsServer,
		cancelOrderServer:      cancelOrderServer,
		createOrderServer:      createOrderServer,
		getHistoryOrdersServer: getHistoryOrdersServer,
		getProductsServer:      getProductsServer,
		getCandlesServer:       getCandlesServer,
		getUserOrdersServer:    getUserOrdersServer,
		getUserAssetsServer:    getUserAssetsServer,
		createUserServer:       createUserServer,
		userLoginServer:        userLoginServer,
		teardownFn: func() {
			wsServer.Close()

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
			scenario: "test happy case",
			fn: func(t *testing.T) {
				testSetup := testSetupFn(t)
				defer testSetup.teardownFn()

				testSetup.createUserServer.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, userAEmail, userAPassword))))
				loginRecorder := httptest.NewRecorder()
				testSetup.userLoginServer.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, userAEmail, userAPassword))))

				time.Sleep(1000 * time.Millisecond)

				var token string
				for _, cookie := range loginRecorder.Result().Cookies() {
					if cookie.Name == "accessToken" {
						token = cookie.Value
					}
				}

				jar, err := cookiejar.New(nil)
				assert.Nil(t, err)
				u, err := url.Parse(testSetup.wsServer.URL)
				assert.Nil(t, err)
				jar.SetCookies(u, []*http.Cookie{
					{
						Name:  "accessToken",
						Value: token,
					},
				})
				websocket.DefaultDialer.Jar = jar
				wsConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(testSetup.wsServer.URL, "http"), nil)
				assert.Nil(t, err)
				chMsg := make(chan string)
				go func() {
					defer close(chMsg)

					for {
						err := wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
						assert.Nil(t, err)
						_, msg, err := wsConn.ReadMessage()
						if err != nil {
							return
						}
						chMsg <- string(msg)
					}
				}()
				wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","product_ids":["BTC-USDT"],"channels":["ticker"],"token":""}`))
				wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","product_ids":["BTC-USDT"],"channels":["candles","match","level2","order"],"token":""}`))
				wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","currency_ids":["BTC","USDT"],"channels":["funds"],"token":""}`))
				wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","product_ids":["BTC-USDT"],"channels":["candles_60"],"token":""}`))
				wsConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","product_ids":["BTC-USDT"],"channels":["order"],"token":""}`))

				reqWithAuthFn := func(r *http.Request) *http.Request {
					for _, cookie := range loginRecorder.Result().Cookies() {
						r.AddCookie(cookie)
					}
					return r
				}

				testSetup.createOrderServer.ServeHTTP(httptest.NewRecorder(), reqWithAuthFn(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
					"productId": "BTC-USDT",
					"side": "buy",
					"type": "limit",
					"price": 1,
					"size": 2
				}`))))

				testSetup.createOrderServer.ServeHTTP(httptest.NewRecorder(), reqWithAuthFn(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
					"productId": "BTC-USDT",
					"side": "sell",
					"type": "limit",
					"price": 1,
					"size": 2
				}`))))

				resultTypeCount := make(map[string]int)
				var results []string
				for msg := range chMsg {
					result := make(map[string]json.RawMessage)
					assert.Nil(t, json.Unmarshal([]byte(msg), &result))
					var typeName string
					json.Unmarshal(result["type"], &typeName)
					resultTypeCount[typeName]++
					results = append(results, msg)
				}
				fmt.Println("recv: ", results)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
