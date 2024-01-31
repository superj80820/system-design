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
	authDelivery "github.com/superj80820/system-design/auth/delivery"
	authHttpDelivery "github.com/superj80820/system-design/auth/delivery/http"
	"github.com/superj80820/system-design/auth/usecase"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	quotationRepoMySQLAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/mysqlandredis"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	kafkaWriterManagerMQKit "github.com/superj80820/system-design/kit/mq/kafka/writermanager"
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
	fmt.Println(kafkaWriterManagerMQKit.Hash{})

	currencyMap := map[string]int{
		"BTC":  1,
		"USDT": 2,
	} // TODO

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

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	if err != nil {
		panic(err)
	}
	tracer := traceKit.CreateNoOpTracer()

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMQTopic, mysqlDB)
	if err != nil {
		panic(err)
	}
	orderRepo := orderMysqlReop.CreateOrderRepo(mysqlDB)
	candleRepo := candleRepoRedis.CreateCandleRepo(mysqlDB, redisCache)
	quotationRepo := quotationRepoMySQLAndRedis.CreateQuotationRepo(mysqlDB, redisCache)

	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(ctx, tradingRepo, quotationRepo, 100) // TODO: 100?
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, tradingRepo, candleRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, tradingRepo, orderRepo, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
	syncTradingUseCase := trading.CreateSyncTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase)
	tradingUseCase := trading.CreateTradingUseCase(ctx, tradingRepo, orderUserCase, userAssetUseCase, syncTradingUseCase, matchingUseCase, 100, logger) // TODO: orderBookDepth use function? 100?
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
		if err := background.RunAsyncTradingSequencer(ctx, tradingSequencerUseCase, quotationUseCase, candleUseCase, orderUserCase, tradingUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()

	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		return authUseCase.Verify(token)
	})
	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}
	r := mux.NewRouter()
	r.Methods("GET").Path("/api/v1/assets").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeGetUserAssetsEndpoint(userAssetUseCase)),
			httpDelivery.DecodeGetUserAssetsRequests,
			httpDelivery.EncodeGetUserAssetsResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/orders/{orderID}").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeGetUserOrderEndpoint(orderUserCase)),
			httpDelivery.DecodeGetUserOrderRequest,
			httpDelivery.EncodeGetUserOrderResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeGetUserOrdersEndpoint(orderUserCase)),
			httpDelivery.DecodeGetUserOrdersRequest,
			httpDelivery.EncodeGetUserOrdersResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/orderBook").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetOrderBookEndpoint(matchingUseCase),
			httpDelivery.DecodeGetOrderBookRequest,
			httpDelivery.EncodeGetOrderBookResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/ticks").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetTickEndpoint(quotationUseCase),
			httpDelivery.DecodeGetTickRequests,
			httpDelivery.EncodeGetTickResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/day").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetSecBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetSecBarRequest,
			httpDelivery.EncodeGetSecBarResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/hour").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetHourBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetHourBarRequest,
			httpDelivery.EncodeGetHourBarResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/min").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetMinBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetMinBarRequest,
			httpDelivery.EncodeGetMinBarResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/sec").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetSecBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetSecBarRequest,
			httpDelivery.EncodeGetSecBarResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/history/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeGetHistoryOrdersEndpoint(orderUserCase)),
			httpDelivery.DecodeGetHistoryOrdersRequest,
			httpDelivery.EncodeGetHistoryOrdersResponse,
			options...,
		),
	)
	r.Methods("GET").Path("/api/v1/history/orders/{orderID}/matches").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeGetHistoryMatchOrderDetailsEndpoint(tradingUseCase)),
			httpDelivery.DecodeGetHistoryMatchOrderDetailsRequest,
			httpDelivery.EncodeGetHistoryMatchOrderDetailsResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/v1/orders/{orderID}/cancel").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase)),
			httpDelivery.DecodeCancelOrderRequest,
			httpDelivery.EncodeCancelOrderResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/v1/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase)),
			httpDelivery.DecodeCreateOrderRequest,
			httpDelivery.EncodeCreateOrderResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/v1/deposit").Handler(
		httptransport.NewServer(
			authMiddleware(httpDelivery.MakeCreateDepositEndpoint(tradingSequencerUseCase)),
			httpDelivery.DecodeCreateDepositRequest,
			httpDelivery.EncodeCreateDepositResponse,
			options...,
		),
	)
	r.Methods("POST").Path("/api/v1/user/register").Handler( // TODO: 須用複數嗎
		httptransport.NewServer(
			authHttpDelivery.MakeAccountRegisterEndpoint(accountUseCase),
			authHttpDelivery.DecodeAccountRegisterRequest,
			authHttpDelivery.EncodeAccountRegisterResponse,
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/login").Handler(
		httptransport.NewServer(
			authHttpDelivery.MakeAuthLoginEndpoint(authUseCase),
			authHttpDelivery.DecodeAuthLoginRequest,
			authHttpDelivery.EncodeAuthLoginResponse,
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/logout").Handler(
		httptransport.NewServer(
			authHttpDelivery.MakeAuthLogoutEndpoint(authUseCase),
			authHttpDelivery.DecodeAuthLogoutRequest,
			authHttpDelivery.EncodeAuthLogoutResponse,
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/verify").Handler(
		httptransport.NewServer(
			authDelivery.MakeAuthVerifyEndpoint(authUseCase),
			authHttpDelivery.DecodeAuthVerifyRequest,
			authHttpDelivery.EncodeAuthVerifyResponse,
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/refresh").Handler(
		httptransport.NewServer(
			authHttpDelivery.MakeRefreshAccessTokenEndpoint(authUseCase),
			authHttpDelivery.DecodeRefreshAccessTokenRequest,
			authHttpDelivery.EncodeRefreshAccessTokenResponse,
			options...,
		))

	httpSrv := http.Server{
		Addr:    ":9090",
		Handler: r,
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
