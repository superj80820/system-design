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
	"strconv"
	"syscall"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	mqKit "github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	sequencerKafkaAndMySQLRepo "github.com/superj80820/system-design/exchange/repository/sequencer/kafkaandmysql"
	// sequencerMemoryRepo "github.com/superj80820/system-design/exchange/repository/sequencer/memory"
	tradingMemoryRepo "github.com/superj80820/system-design/exchange/repository/trading/memory"
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
	KAFKA_SEQUENCE_TOPIC = "SEQUENCE"
	SERVICE_NAME         = "exchange-service"
)

func main() {
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
	sequenceMessageTopic, err := mqKit.CreateMQTopic(
		ctx,
		fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port()),
		KAFKA_SEQUENCE_TOPIC,
		mqKit.ConsumeByGroupID(SERVICE_NAME, true),
		mqKit.CreateTopic(1, 1),
	)
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
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlDBUsername,
		mysqlDBPassword,
		mysqlDBHost,
		mysqlDBPort.Port(),
		mysqlDBName,
	), "york")
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

	tradingRepo := tradingMemoryRepo.CreateTradingRepo(ctx)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	// sequencerRepo := sequencerMemoryRepo.CreateTradingSequencerRepo(ctx)
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateTradingSequencerRepo(ctx, sequenceMessageTopic, mysqlDB)
	if err != nil {
		panic(err)
	}

	redisCache, err := redisKit.CreateCache(redisHost+":"+redisPort.Port(), "", 0)
	if err != nil {
		panic(err)
	}
	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	if err != nil {
		panic(err)
	}

	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(100) // TODO: 100?
	candleRepo := candleRepoRedis.CreateCandleRepo(redisCache)
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, candleRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
	tradingUseCase := trading.CreateTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo) // TODO: orderBookDepth use function?
	tradingAsyncUseCase := trading.CreateAsyncTradingUseCase(ctx, tradingRepo, tradingUseCase, matchingUseCase, 100, logger)            //TODO:100?
	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(logger, sequencerRepo, tradingRepo, 3000, 500*time.Millisecond)

	go func() {
		if err := background.RunAsyncTradingSequencer(ctx, tradingSequencerUseCase, tradingAsyncUseCase, quotationUseCase, candleUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()
	// go background.RunAsyncTrading(ctx, tradingAsyncUseCase) // TODO: need?

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

	r := mux.NewRouter()
	r.Methods("GET").Path("/api/v1/assets").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetUserAssetsEndpoint(userAssetUseCase),
			httpDelivery.DecodeGetUserAssetsRequests,
			httpDelivery.EncodeGetUserAssetsResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/orders/{orderID}").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetUserOrderEndpoint(orderUserCase),
			httpDelivery.DecodeGetUserOrderRequest,
			httpDelivery.EncodeGetUserOrderResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/orders").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetUserOrdersEndpoint(orderUserCase),
			httpDelivery.DecodeGetUserOrdersRequest,
			httpDelivery.EncodeGetUserOrdersResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/orderBook").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetOrderBookEndpoint(matchingUseCase),
			httpDelivery.DecodeGetOrderBookRequest,
			httpDelivery.EncodeGetOrderBookResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/ticks").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetTickEndpoint(quotationUseCase),
			httpDelivery.DecodeGetTickRequests,
			httpDelivery.EncodeGetTickResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/day").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetSecBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetSecBarRequest,
			httpDelivery.EncodeGetSecBarResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/hour").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetHourBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetHourBarRequest,
			httpDelivery.EncodeGetHourBarResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/min").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetMinBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetMinBarRequest,
			httpDelivery.EncodeGetMinBarResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("GET").Path("/api/v1/bars/sec").Handler(
		httptransport.NewServer(
			httpDelivery.MakeGetSecBarEndpoint(candleUseCase),
			httpDelivery.DecodeGetSecBarRequest,
			httpDelivery.EncodeGetSecBarResponse,
			serverBeforeAddUserID,
		),
	)
	// r.Methods("GET").Path("/api/v1/history/orders").Handler()
	// r.Methods("GET").Path("/api/v1/history/orders/{orderID}/matches").Handler()
	r.Methods("POST").Path("/api/v1/orders/{orderID}/cancel").Handler(
		httptransport.NewServer(
			httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase),
			httpDelivery.DecodeCancelOrderRequest,
			httpDelivery.EncodeCancelOrderResponse,
			serverBeforeAddUserID,
		),
	)
	r.Methods("POST").Path("/api/v1/orders").Handler(
		httptransport.NewServer(
			httpDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase),
			httpDelivery.DecodeCreateOrderRequest,
			httpDelivery.EncodeCreateOrderResponse,
			serverBeforeAddUserID,
		),
	)

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
