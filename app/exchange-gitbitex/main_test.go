package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	authMySQLRepo "github.com/superj80820/system-design/auth/repository/auth/mysql"
	"github.com/superj80820/system-design/auth/usecase/account"
	"github.com/superj80820/system-design/auth/usecase/auth"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	httpGitbitexDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex"
	wsDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex/ws"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	matchingMySQLAndMQRepo "github.com/superj80820/system-design/exchange/repository/matching/mysqlandmq"
	orderORMRepo "github.com/superj80820/system-design/exchange/repository/order/ormandmq"
	quotationRepoORMAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/ormandredis"
	sequencerKafkaAndMySQLRepo "github.com/superj80820/system-design/exchange/repository/sequencer/kafkaandmysql"
	tradingMySQLAndMongoRepo "github.com/superj80820/system-design/exchange/repository/trading/mysqlandmongo"
	"github.com/superj80820/system-design/exchange/usecase/asset"
	candleUseCaseLib "github.com/superj80820/system-design/exchange/usecase/candle"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/currency"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/quotation"
	"github.com/superj80820/system-design/exchange/usecase/trading"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	mongoDBContainer "github.com/superj80820/system-design/kit/testing/mongo/container"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
	redisContainer "github.com/superj80820/system-design/kit/testing/redis/container"
	traceKit "github.com/superj80820/system-design/kit/trace"
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
)

func testSetupFn(t assert.TestingT) *testSetup {
	accessTokenKeyPath := "./access-private-key.pem"
	refreshTokenKeyPath := "./refresh-private-key.pem"
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

	mysqlContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(filepath.Join(".", "schema.sql")))
	assert.Nil(t, err)
	redisContainer, err := redisContainer.CreateRedis(ctx)
	assert.Nil(t, err)
	mongoDBContainer, err := mongoDBContainer.CreateMongoDB(ctx)
	assert.Nil(t, err)

	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlContainer.GetURI()))
	assert.Nil(t, err)
	redisCache, err := redisKit.CreateCache(redisContainer.GetURI(), "", 0)
	assert.Nil(t, err)
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoDBContainer.GetURI()))
	assert.Nil(t, err)

	eventsCollection := mongoDB.Database("exchange").Collection("events")
	eventsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"sequence_id": -1,
		},
		Options: options.Index().SetUnique(true),
	})

	messageChannelBuffer := 10
	messageCollectDuration := 100 * time.Millisecond
	sequenceMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	tradingEventMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	tradingResultMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)

	assetMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	orderMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	candleMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	tickMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	matchingMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	orderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	assert.Nil(t, err)
	tracer := traceKit.CreateNoOpTracer()

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	assetRepo := assetMemoryRepo.CreateAssetRepo(assetMQTopic)
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMQTopic, mysqlDB)
	if err != nil {
		panic(err)
	}
	orderRepo := orderORMRepo.CreateOrderRepo(mysqlDB, orderMQTopic)
	candleRepo := candleRepoRedis.CreateCandleRepo(mysqlDB, redisCache, candleMQTopic)
	quotationRepo := quotationRepoORMAndRedis.CreateQuotationRepo(mysqlDB, redisCache, tickMQTopic)
	matchingRepo := matchingMySQLAndMQRepo.CreateMatchingRepo(mysqlDB, matchingMQTopic, orderBookMQTopic)
	matchingOrderBookRepo := matchingMySQLAndMQRepo.CreateOrderBookRepo()
	accountRepo := accountMySQLRepo.CreateAccountRepo(mysqlDB)
	authRepo := authMySQLRepo.CreateAuthRepo(mysqlDB)

	currencyUseCase := currency.CreateCurrencyUseCase(&currencyProduct)
	matchingUseCase := matching.CreateMatchingUseCase(ctx, matchingRepo, matchingOrderBookRepo, 100) // TODO: 100?
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(ctx, tradingRepo, quotationRepo, 100) // TODO: 100?
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, candleRepo)
	orderUseCase := order.CreateOrderUseCase(userAssetUseCase, orderRepo)
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUseCase)
	syncTradingUseCase := trading.CreateSyncTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUseCase, clearingUseCase)
	tradingUseCase := trading.CreateTradingUseCase(ctx, tradingRepo, matchingRepo, quotationRepo, candleRepo, orderRepo, assetRepo, sequencerRepo, orderUseCase, userAssetUseCase, syncTradingUseCase, matchingUseCase, currencyUseCase, 100, logger, 3000, 500*time.Millisecond) // TODO: orderBookDepth use function? 100?
	accountUseCase, err := account.CreateAccountUseCase(accountRepo, logger)
	assert.Nil(t, err)
	authUseCase, err := auth.CreateAuthUseCase(accessTokenKeyPath, refreshTokenKeyPath, authRepo, accountRepo, logger)
	assert.Nil(t, err)

	go func() {
		if err := background.AsyncTradingConsume(ctx, quotationUseCase, candleUseCase, orderUseCase, tradingUseCase, matchingUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()

	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		return authUseCase.Verify(token)
	})

	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer, httpKit.OptionSetCookieAccessTokenKey("accessToken"))),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}

	cancelOrderServer := httptransport.NewServer(
		authMiddleware(httpDelivery.MakeCancelOrderEndpoint(tradingUseCase)),
		httpDelivery.DecodeCancelOrderRequest,
		httpDelivery.EncodeCancelOrderResponse,
		options...,
	)
	createOrderServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakeCreateOrderEndpoint(tradingUseCase)),
		httpGitbitexDelivery.DecodeCreateOrderRequest,
		httpGitbitexDelivery.EncodeCreateOrderResponse,
		options...,
	)
	getHistoryOrdersServer := httptransport.NewServer(
		authMiddleware(httpGitbitexDelivery.MakerGetHistoryOrdersEndpoint(tradingUseCase, currencyUseCase)),
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
		httpGitbitexDelivery.MakeAccountRegisterEndpoint(accountUseCase, tradingUseCase, currencyUseCase),
		httpGitbitexDelivery.DecodeAccountRegisterRequest,
		httpGitbitexDelivery.EncodeAccountRegisterResponse,
		options...,
	)
	userLoginServer := httptransport.NewServer(
		httpGitbitexDelivery.MakeAuthLoginEndpoint(authUseCase),
		httpGitbitexDelivery.DecodeAuthLoginRequest,
		httpGitbitexDelivery.EncodeAuthResponse,
		options...,
	)
	wsServer := httptest.NewServer(wsTransport.NewServer(
		wsDelivery.MakeExchangeEndpoint(tradingUseCase, authUseCase),
		wsDelivery.DecodeStreamExchangeRequest,
		wsDelivery.EncodeStreamExchangeResponse,
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
			err = mongoDBContainer.Terminate(ctx)
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

func BenchmarkServer(b *testing.B) {
	testSetup := testSetupFn(b)
	// defer testSetup.teardownFn()

	testSetup.createUserServer.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, userAEmail, userAPassword))))
	loginRecorder := httptest.NewRecorder()
	testSetup.userLoginServer.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, userAEmail, userAPassword))))

	time.Sleep(1000 * time.Millisecond)

	reqWithAuthFn := func(r *http.Request) *http.Request {
		for _, cookie := range loginRecorder.Result().Cookies() {
			r.AddCookie(cookie)
		}
		return r
	}

	direction := []string{"buy", "sell"}
	for i := 0; i < b.N; i++ {
		randNum := rand.Float64() * 10
		testSetup.createOrderServer.ServeHTTP(httptest.NewRecorder(), reqWithAuthFn(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{
			"productId": "BTC-USDT",
			"side": "%s",
			"type": "limit",
			"price": %f,
			"size": 3
		}`, direction[i%2], randNum)))))
	}
}
