package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"path/filepath"
	"syscall"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/auth/usecase"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	httpGitbitexDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex"
	wsDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex/ws"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	matchingMySQLAndMQRepo "github.com/superj80820/system-design/exchange/repository/matching/mysqlandmq"
	quotationRepoMySQLAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/mysqlandredis"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	wsMiddleware "github.com/superj80820/system-design/kit/http/websocket/middleware"
	traceKit "github.com/superj80820/system-design/kit/trace"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	orderMysqlReop "github.com/superj80820/system-design/exchange/repository/order/mysql"
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
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	redisKit "github.com/superj80820/system-design/kit/redis"
)

const (
	KAFKA_SEQUENCE_TOPIC       = "SEQUENCE"
	KAFKA_TRADING_EVENT_TOPIC  = "TRADING_EVENT"
	KAFKA_TRADING_RESULT_TOPIC = "TRADING_RESULT"
	SERVICE_NAME               = "exchange-service"
)

func main() {
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

	// TODO: for develop
	kafkaContainer, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		panic(err)
	}
	kafkaHost, err := kafkaContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9093") // TODO: is correct?
	if err != nil {
		panic(err)
	}

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
	authSchemaSQL, err := os.ReadFile("../../auth/repository/schema.sql")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./schema.sql", []byte(string(quotationSchemaSQL)+"\n"+string(tradingSchemaSQL)+"\n"+string(candleSchemaSQL)+"\n"+string(orderSchemaSQL)+"\n"+string(sequencerSchemaSQL)+"\n"+string(authSchemaSQL)), 0644)
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

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("docker.io/redis:7"),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		panic(err)
	}
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

	fmt.Println("for debug: mysql port: ", mysqlDBPort.Port(), " mongo port: ", "mongodb://"+mongoHost+":"+mongoPort.Port())

	eventsCollection := mongoDB.Database("exchange").Collection("events")
	eventsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"sequence_id": -1,
		},
		Options: options.Index().SetUnique(true),
	})

	sequenceMQTopic, err := kafkaMQKit.CreateMQTopic(
		ctx,
		fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port()),
		KAFKA_SEQUENCE_TOPIC,
		kafkaMQKit.ConsumeByGroupID(SERVICE_NAME, true),
		kafkaMQKit.CreateTopic(1, 1),
	)
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
	tracer := traceKit.CreateNoOpTracer()

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	assetRepo := assetMemoryRepo.CreateAssetRepo(assetMQTopic)
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMQTopic, mysqlDB)
	if err != nil {
		panic(err)
	}
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
	accountUseCase, err := usecase.CreateAccountUseCase(mysqlDB, logger)
	if err != nil {
		panic(err)
	}
	authUseCase, err := usecase.CreateAuthUseCase(mysqlDB, logger)
	if err != nil {
		panic(err)
	}

	go func() {
		if err := background.RunAsyncTradingSequencer(ctx, tradingSequencerUseCase, quotationUseCase, candleUseCase, orderUseCase, tradingUseCase, matchingUseCase); err != nil {
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
	r := mux.NewRouter()
	r.Methods("DELETE").Path("/api/orders/{orderID}").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase)),
			httpDelivery.DecodeCancelOrderRequest,
			httpDelivery.EncodeCancelOrderResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase)),
			httpGitbitexDelivery.DecodeCreateOrderRequest,
			httpGitbitexDelivery.EncodeCreateOrderResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/products/{productID}/trades").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakerGetHistoryOrdersEndpoint(orderUseCase, currencyUseCase)),
			httpGitbitexDelivery.DecodeGetHistoryOrdersRequest,
			httpGitbitexDelivery.EncodeGetHistoryOrdersResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/products").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeGetProductsEndpoint(currencyUseCase),
			httpGitbitexDelivery.DecodeGetProductsRequest,
			httpGitbitexDelivery.EncodeGetProductsResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/products/{productID}/candles").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeGetCandleEndpoint(candleUseCase, currencyUseCase),
			httpGitbitexDelivery.DecodeGetCandlesRequest,
			httpGitbitexDelivery.EncodeGetCandlesResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeGetAccountOrdersEndpoint(orderUseCase, currencyUseCase)),
			httpGitbitexDelivery.DecodeGetAccountOrdersRequest,
			httpGitbitexDelivery.EncodeGetAccountOrdersResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/accounts").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeGetAccountAssetsEndpoint(userAssetUseCase, currencyUseCase)),
			httpGitbitexDelivery.DecodeGetAccountAssetsRequest,
			httpGitbitexDelivery.EncodeGetAccountAssetsResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/users").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeAccountRegisterEndpoint(accountUseCase, tradingSequencerUseCase, currencyUseCase),
			httpGitbitexDelivery.DecodeAccountRegisterRequest,
			httpGitbitexDelivery.EncodeAccountRegisterResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/users/accessToken").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeAuthLoginEndpoint(authUseCase),
			httpGitbitexDelivery.DecodeAuthLoginRequest,
			httpGitbitexDelivery.EncodeAuthResponse,
			options...,
		),
	)
	r.Handle("/ws",
		wsTransport.NewServer(
			wsMiddleware.CreateAuth[domain.TradingNotifyRequest, any](func(token string) (int64, error) {
				return authUseCase.Verify(token)
			})(wsDelivery.MakeExchangeEndpoint(tradingUseCase)),
			wsDelivery.DecodeStreamExchangeRequest,
			wsKit.JsonEncodeResponse[any],
			wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
			wsTransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),     // TODO
			wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()), // TODO: maybe to default
		),
	)

	httpSrv := http.Server{
		Addr:    ":9090",
		Handler: cors.Default().Handler(r),
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(fmt.Sprintf("http server get error, error: %+v", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	httpSrv.Shutdown(ctx)
	kafkaContainer.Terminate(ctx)
	mysqlContainer.Terminate(ctx)
	redisContainer.Terminate(ctx)
}
