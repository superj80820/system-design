package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	_ "net/http/pprof"
	"syscall"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/shopspring/decimal"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	authMySQLRepo "github.com/superj80820/system-design/auth/repository/auth/mysql"
	"github.com/superj80820/system-design/auth/usecase/account"
	"github.com/superj80820/system-design/auth/usecase/auth"
	"github.com/superj80820/system-design/domain"

	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	httpGitbitexDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex"
	wsDelivery "github.com/superj80820/system-design/exchange/delivery/httpgitbitex/ws"
	assetMemoryAndMySQLRepo "github.com/superj80820/system-design/exchange/repository/asset/memoryandmysql"
	candleRepoRedis "github.com/superj80820/system-design/exchange/repository/candle"
	matchingMemoryAndRedisRepo "github.com/superj80820/system-design/exchange/repository/matching/memoryandredis"
	matchingMySQLAndMQRepo "github.com/superj80820/system-design/exchange/repository/matching/mysqlandmq"
	quotationRepoMySQLAndRedis "github.com/superj80820/system-design/exchange/repository/quotation/ormandredis"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	kafkaContainer "github.com/superj80820/system-design/kit/testing/kafka/container"
	mongoDBContainer "github.com/superj80820/system-design/kit/testing/mongo/container"
	mysqlContainer "github.com/superj80820/system-design/kit/testing/mysql/container"
	redisContainer "github.com/superj80820/system-design/kit/testing/redis/container"
	traceKit "github.com/superj80820/system-design/kit/trace"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/superj80820/system-design/kit/mq"
	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"

	orderORMRepo "github.com/superj80820/system-design/exchange/repository/order/ormandmq"
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
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	redisRateLimitKit "github.com/superj80820/system-design/kit/ratelimit/redis"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func main() {
	utilKit.LoadEnvFile(".env")
	serviceName := utilKit.GetEnvString("SERVICE_NAME", "exchange-service")
	enableHTTPS := utilKit.GetEnvBool("ENABLE_HTTPS", false)
	httpsCertFilePath := utilKit.GetEnvString("HTTPS_CERT_FILE_PATH", "./fullchain.pem")
	httpsPrivateKeyFilePath := utilKit.GetEnvString("HTTPS_PRIVATE_KEY_FILE_PATH", "./privkey.pem")
	sequenceTopicName := utilKit.GetEnvString("SEQUENCE_TOPIC_NAME", "SEQUENCE_TOPIC")
	tradingEventTopicName := utilKit.GetEnvString("TRADING_EVENT_TOPIC_NAME", "TRADING_EVENT_TOPIC")
	assetTopicName := utilKit.GetEnvString("ASSET_TOPIC_NAME", "ASSET_TOPIC")
	tickTopicName := utilKit.GetEnvString("TICK_TOPIC_NAME", "TICK_TOPIC")
	l2OrderBookTopicName := utilKit.GetEnvString("L2_ORDER_BOOK_TOPIC_NAME", "L2_ORDER_BOOK_TOPIC")
	matchingTopicName := utilKit.GetEnvString("MATCHING_TOPIC_NAME", "MATCHING_TOPIC")
	orderTopicName := utilKit.GetEnvString("ORDER_TOPIC_NAME", "ORDER_TOPIC")
	candleTradingResultTopicName := utilKit.GetEnvString("CANDLE_TRADING_RESULT_TOPIC_NAME", "CANDLE_TRADING_RESULT_TOPIC")
	candleTopicName := utilKit.GetEnvString("CANDLE_TOPIC_NAME", "CANDLE_TOPIC")
	kafkaURI := utilKit.GetEnvString("KAFKA_URI", "")
	mysqlURI := utilKit.GetEnvString("MYSQL_URI", "")
	mongoURI := utilKit.GetEnvString("MONGO_URI", "")
	isClearDB := utilKit.GetEnvBool("IS_CLEAR_DB", false)
	enableKafka := utilKit.GetEnvBool("ENABLE_KAFKA", true)
	redisURI := utilKit.GetEnvString("REDIS_URI", "")
	enableUserRateLimit := utilKit.GetEnvBool("ENABLE_USER_RATE_LIMIT", false)
	enableBackupSnapshot := utilKit.GetEnvBool("ENABLE_BACKUP_SNAPSHOT", true)
	backupSnapshotDuration := utilKit.GetEnvInt("BACKUP_SNAPSHOT_DURATION", 600)
	enableAutoPreviewTrading := utilKit.GetEnvBool("ENABLE_AUTO_PREVIEW_TRADING", false)
	autoPreviewTradingEmail := utilKit.GetEnvString("AUTO_PREVIEW_TRADING_EMAIL", "guest@gmail.com")
	autoPreviewTradingPassword := utilKit.GetEnvString("AUTO_PREVIEW_TRADING_PASSWORD", "123456789")
	autoPreviewTradingDuration := utilKit.GetEnvInt("AUTO_PREVIEW_TRADING_DURATION", 1)
	autoPreviewTradingMaxOrderPrice := utilKit.GetEnvFloat64("AUTO_PREVIEW_TRADING_MAX_ORDER_PRICE", 3.5)
	autoPreviewTradingMinOrderPrice := utilKit.GetEnvFloat64("AUTO_PREVIEW_TRADING_MIN_ORDER_PRICE", 1.5)
	autoPreviewTradingMaxQuantity := utilKit.GetEnvFloat64("AUTO_PREVIEW_TRADING_MAX_ORDER_QUANTITY", 2)
	autoPreviewTradingMinQuantity := utilKit.GetEnvFloat64("AUTO_PREVIEW_TRADING_MIN_ORDER_QUANTITY", 1)
	enablePprofServer := utilKit.GetEnvBool("ENABLE_PPROF_SERVER", false)
	accessTokenKey := utilKit.GetRequireEnvString("ACCESS_TOKEN_KEY")
	refreshTokenKey := utilKit.GetRequireEnvString("REFRESH_TOKEN_KEY")
	baseCurrency := utilKit.GetEnvString("CURRENCY_BASE", "BTC")
	quoteCurrency := utilKit.GetEnvString("CURRENCY_QUOTE", "USDT")
	currencyProduct := domain.CurrencyProduct{
		ID:             fmt.Sprintf("%s-%s", baseCurrency, quoteCurrency),
		BaseCurrency:   baseCurrency,
		QuoteCurrency:  quoteCurrency,
		QuoteIncrement: "0.0",
		QuoteMaxSize:   decimal.NewFromInt(utilKit.GetEnvInt64("CURRENCY_QUOTE_MAX_SIZE", 100000000)).String(),
		QuoteMinSize:   decimal.NewFromFloat(utilKit.GetEnvFloat64("CURRENCY_QUOTE_MIN_SIZE", 0.000001)).String(),
		BaseMaxSize:    decimal.NewFromInt(utilKit.GetEnvInt64("CURRENCY_BASE_MAX_SIZE", 100000000)).String(),
		BaseMinSize:    decimal.NewFromFloat(utilKit.GetEnvFloat64("CURRENCY_BASE_MIN_SIZE", 0.000001)).String(),
		BaseScale:      6,
		QuoteScale:     2,
	}

	ctx := context.Background()

	if kafkaURI == "" {
		if enableKafka {
			kafkaContainer, err := kafkaContainer.CreateKafka(ctx)
			if err != nil {
				panic(err)
			}
			defer kafkaContainer.Terminate(ctx)
			kafkaURI = kafkaContainer.GetURI()
		}

		fmt.Println("testcontainers kafka uri: ", kafkaURI)
	}

	if mysqlURI == "" {
		mySQLContainer, err := mysqlContainer.CreateMySQL(ctx, mysqlContainer.UseSQLSchema(filepath.Join(".", "schema.sql")))
		if err != nil {
			panic(err)
		}
		defer mySQLContainer.Terminate(ctx)
		mysqlURI = mySQLContainer.GetURI()

		fmt.Println("testcontainers mysql uri: ", mysqlURI)
	}

	if mongoURI == "" {
		mongoDBContainer, err := mongoDBContainer.CreateMongoDB(ctx)
		if err != nil {
			panic(err)
		}
		defer mongoDBContainer.Terminate(ctx)
		mongoURI = mongoDBContainer.GetURI()

		fmt.Println("testcontainers mongo uri: ", mongoURI)
	}

	if redisURI == "" {
		redisContainer, err := redisContainer.CreateRedis(ctx)
		if err != nil {
			panic(err)
		}
		defer redisContainer.Terminate(ctx)
		redisURI = redisContainer.GetURI()

		fmt.Println("testcontainers redis uri: ", redisURI)
	}

	ormDB, err := ormKit.CreateDB(ormKit.UseMySQL(mysqlURI))
	if err != nil {
		panic(err)
	}

	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		panic(err)
	}

	redisCache, err := redisKit.CreateCache(redisURI, "", 0)
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

	messageChannelBuffer := 10000
	messageCollectDuration := 500 * time.Millisecond
	var (
		sequenceMQTopic,
		assetMQTopic,
		tickMQTopic,
		matchingMQTopic,
		orderMQTopic,
		candleTradingResultMQTopic,
		candleMQTopic,
		tradingEventMQTopic,
		l2OrderBookMQTopic,

		matchingNotifyMQTopic,
		tickNotifyMQTopic,
		assetNotifyMQTopic,
		orderNotifyMQTopic,
		candleNotifyMQTopic,
		l2OrderBookNotifyMQTopic mq.MQTopic
	)
	if enableKafka {
		sequenceMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			sequenceTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+sequenceTopicName, true),
			100000,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		tradingEventMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			tradingEventTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+tradingEventTopicName, true),
			100000,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		assetMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			assetTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+assetTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		tickMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			tickTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+tickTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		matchingMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			matchingTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+matchingTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		l2OrderBookMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			l2OrderBookTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+l2OrderBookTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		orderMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			orderTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+orderTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		candleTradingResultMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			candleTradingResultTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+candleTradingResultTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}
		candleMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			candleTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+candleTopicName, true),
			messageChannelBuffer,
			messageCollectDuration,
			kafkaMQKit.CreateTopic(1, 1),
		)
		if err != nil {
			panic(err)
		}

		assetNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			assetTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+assetTopicName+":NOTIFY", true),
			100,
			500*time.Millisecond,
			kafkaMQKit.UseRingBuffer(),
		)
		if err != nil {
			panic(err)
		}
		tickNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			tickTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+tickTopicName+":NOTIFY", true),
			100,
			500*time.Millisecond,
			kafkaMQKit.UseRingBuffer(),
		)
		if err != nil {
			panic(err)
		}
		matchingNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			matchingTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+matchingTopicName+":NOTIFY", true),
			100,
			500*time.Millisecond,
			kafkaMQKit.UseRingBuffer(), // TODO: think
		)
		if err != nil {
			panic(err)
		}
		l2OrderBookNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			l2OrderBookTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+l2OrderBookTopicName+":NOTIFY", true),
			100,
			500*time.Millisecond,
			kafkaMQKit.UseRingBuffer(),
		)
		if err != nil {
			panic(err)
		}
		orderNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			orderTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+orderTopicName+":NOTIFY", true),
			100,
			500*time.Millisecond,
			kafkaMQKit.UseRingBuffer(), // TODO: think
		)
		if err != nil {
			panic(err)
		}
		candleNotifyMQTopic, err = kafkaMQKit.CreateMQTopic(
			ctx,
			kafkaURI,
			candleTopicName,
			kafkaMQKit.ConsumeByGroupID(serviceName+":"+candleTopicName+":NOTIFY", true),
			10000,
			500*time.Millisecond,
		)
		if err != nil {
			panic(err)
		}
	} else {
		sequenceMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		assetMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		tickMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		matchingMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		orderMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		candleMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		candleTradingResultMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
		tradingEventMQTopic = memoryMQKit.CreateMemoryMQ(ctx, 100000, messageCollectDuration)
		l2OrderBookMQTopic = memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)

		assetNotifyMQTopic = assetMQTopic
		tickNotifyMQTopic = tickMQTopic
		matchingNotifyMQTopic = matchingMQTopic
		candleNotifyMQTopic = candleMQTopic
		l2OrderBookNotifyMQTopic = l2OrderBookMQTopic
		orderNotifyMQTopic = orderMQTopic
	}

	tradingResultMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100000, messageCollectDuration)

	orderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	l1OrderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)
	l3OrderBookMQTopic := memoryMQKit.CreateMemoryMQ(ctx, messageChannelBuffer, messageCollectDuration)

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel, loggerKit.WithRotateLog(10, 10, 10))
	if err != nil {
		panic(err)
	}
	tracer := traceKit.CreateNoOpTracer()

	if isClearDB {
		if err := ormDB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE events").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE match_details").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE orders").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE ticks").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE sec_bars").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE min_bars").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE hour_bars").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE day_bars").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE account_token").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("TRUNCATE TABLE account").Error; err != nil {
			panic(err)
		}
		if err := ormDB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			panic(err)
		}
	}

	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, tradingEventMQTopic, tradingResultMQTopic, ormDB)
	assetRepo := assetMemoryAndMySQLRepo.CreateAssetRepo(assetMQTopic, ormDB)
	sequencerRepo, err := sequencerKafkaAndMySQLRepo.CreateSequencerRepo(ctx, sequenceMQTopic, ormDB)
	if err != nil {
		panic(err)
	}
	orderRepo := orderORMRepo.CreateOrderRepo(ormDB, orderMQTopic)
	candleRepo := candleRepoRedis.CreateCandleRepo(ormDB, redisCache, candleTradingResultMQTopic, candleMQTopic)
	quotationRepo := quotationRepoMySQLAndRedis.CreateQuotationRepo(ormDB, redisCache, tickMQTopic)
	matchingRepo := matchingMySQLAndMQRepo.CreateMatchingRepo(ormDB, matchingMQTopic)
	matchingOrderBookRepo := matchingMemoryAndRedisRepo.CreateOrderBookRepo(redisCache, orderBookMQTopic, l1OrderBookMQTopic, l2OrderBookMQTopic, l3OrderBookMQTopic)

	assetNotifyRepo := assetMemoryAndMySQLRepo.CreateAssetNotifyRepo(assetNotifyMQTopic)
	quotationNotifyRepo := quotationRepoMySQLAndRedis.CreateQuotationNotifyRepo(tickNotifyMQTopic)
	matchingNotifyRepo := matchingMySQLAndMQRepo.CreateMatchingNotifyRepo(matchingNotifyMQTopic)
	matchingOrderBookNotifyRepo := matchingMemoryAndRedisRepo.CreateMatchingOrderBookNotifyRepo(l2OrderBookNotifyMQTopic)
	orderNotifyRepo := orderORMRepo.CreateOrderNotifyRepo(orderNotifyMQTopic)
	candleNotifyRepo := candleRepoRedis.CreateCandleNotifyRepo(candleNotifyMQTopic)

	accountRepo := accountMySQLRepo.CreateAccountRepo(ormDB)
	authRepo, err := authMySQLRepo.CreateAuthRepo(ormDB, accessTokenKey, refreshTokenKey)
	if err != nil {
		panic(err)
	}

	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(sequencerRepo, tradingRepo)
	currencyUseCase := currency.CreateCurrencyUseCase(&currencyProduct)
	matchingUseCase := matching.CreateMatchingUseCase(ctx, matchingRepo, matchingOrderBookRepo)
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	quotationUseCase := quotation.CreateQuotationUseCase(ctx, tradingRepo, quotationRepo)
	candleUseCase := candleUseCaseLib.CreateCandleUseCase(ctx, candleRepo)
	orderUseCase := order.CreateOrderUseCase(userAssetUseCase, orderRepo)
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUseCase)
	syncTradingUseCase := trading.CreateSyncTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUseCase, clearingUseCase)
	tradingUseCase := trading.CreateTradingUseCase(ctx, tradingRepo, matchingRepo, quotationRepo, candleRepo, orderRepo, assetRepo, tradingSequencerUseCase, orderUseCase, userAssetUseCase, syncTradingUseCase, matchingUseCase, logger)

	tradingNotifyUseCase := trading.CreateTradingNotifyUseCase(ctx, matchingOrderBookNotifyRepo, candleNotifyRepo, assetNotifyRepo, quotationNotifyRepo, orderNotifyRepo, matchingNotifyRepo, currencyUseCase, matchingUseCase, 100, logger) // TODO: orderBookDepth use function? 100?

	accountUseCase, err := account.CreateAccountUseCase(accountRepo, logger)
	if err != nil {
		panic(err)
	}
	authUseCase, err := auth.CreateAuthUseCase(authRepo, accountRepo, logger)
	if err != nil {
		panic(err)
	}

	go func() {
		if enableBackupSnapshot {
			readyCh, errCh := background.AsyncBackupSnapshot(ctx, tradingUseCase, time.Duration(backupSnapshotDuration)*time.Second)
			select {
			case <-readyCh:
			case err := <-errCh:
				if err != nil {
					logger.Fatal(fmt.Sprintf("backup snapshot get error, error: %+v", err)) // TODO: correct?
				}
			}
		}
		if err := background.AsyncTradingConsume(ctx, quotationUseCase, candleUseCase, orderUseCase, userAssetUseCase, tradingUseCase, matchingUseCase, tradingSequencerUseCase); err != nil {
			logger.Fatal(fmt.Sprintf("async trading sequencer get error, error: %+v", err)) // TODO: correct?
		}
	}()

	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		return authUseCase.Verify(token)
	})
	userRateLimitMiddleware := httpMiddlewareKit.CreateNoOpRateLimitMiddleware()
	if enableUserRateLimit {
		userRateLimitMiddleware = httpMiddlewareKit.CreateRateLimitMiddlewareWithSpecKey(false, true, true, redisRateLimitKit.CreateCacheRateLimit(redisCache, 500, 60).Pass)
	}
	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer, httpKit.OptionSetCookieAccessTokenKey("accessToken"))),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}
	r := mux.NewRouter()
	api := r.PathPrefix("/api/").Subrouter()
	api.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	api.Methods("DELETE").Path("/orders/{orderID}").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(httpDelivery.MakeCancelOrderEndpoint(tradingSequencerUseCase))),
			httpDelivery.DecodeCancelOrderRequest,
			httpDelivery.EncodeCancelOrderResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/orders").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(httpGitbitexDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase))),
			httpGitbitexDelivery.DecodeCreateOrderRequest,
			httpGitbitexDelivery.EncodeCreateOrderResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/products/{productID}/trades").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakerGetHistoryOrdersEndpoint(tradingUseCase, currencyUseCase),
			httpGitbitexDelivery.DecodeGetHistoryOrdersRequest,
			httpGitbitexDelivery.EncodeGetHistoryOrdersResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/products").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeGetProductsEndpoint(currencyUseCase),
			httpGitbitexDelivery.DecodeGetProductsRequest,
			httpGitbitexDelivery.EncodeGetProductsResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/products/{productID}/candles").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeGetCandleEndpoint(candleUseCase, currencyUseCase),
			httpGitbitexDelivery.DecodeGetCandlesRequest,
			httpGitbitexDelivery.EncodeGetCandlesResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/orders").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeGetAccountOrdersEndpoint(orderUseCase, currencyUseCase)),
			httpGitbitexDelivery.DecodeGetAccountOrdersRequest,
			httpGitbitexDelivery.EncodeGetAccountOrdersResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/accounts").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeGetAccountAssetsEndpoint(userAssetUseCase, currencyUseCase)),
			httpGitbitexDelivery.DecodeGetAccountAssetsRequest,
			httpGitbitexDelivery.EncodeGetAccountAssetsResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/users").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeAccountRegisterEndpoint(accountUseCase, tradingSequencerUseCase, currencyUseCase),
			httpGitbitexDelivery.DecodeAccountRegisterRequest,
			httpGitbitexDelivery.EncodeAccountRegisterResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/users/accessToken").Handler(
		httptransport.NewServer(
			httpGitbitexDelivery.MakeAuthLoginEndpoint(authUseCase),
			httpGitbitexDelivery.DecodeAuthLoginRequest,
			httpGitbitexDelivery.EncodeAuthResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/users/self").Handler(
		httptransport.NewServer(
			authMiddleware(httpGitbitexDelivery.MakeSelfEndpoint(accountUseCase)),
			httpGitbitexDelivery.DecodeGetSelfRequest,
			httpGitbitexDelivery.EncodeGetSelfResponse,
			options...,
		),
	)

	r.PathPrefix("/ws").Handler(
		wsTransport.NewServer(
			wsDelivery.MakeExchangeEndpoint(tradingNotifyUseCase, authUseCase),
			wsDelivery.DecodeStreamExchangeRequest,
			wsDelivery.EncodeStreamExchangeResponse,
			wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
			wsTransport.ServerBefore(httpKit.CustomBeforeCtx(tracer, httpKit.OptionSetCookieAccessTokenKey("accessToken"))),
			wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()), // TODO: maybe to default
		),
	)

	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./index.html")
	})

	httpSrv := http.Server{
		Addr:    ":9090",
		Handler: cors.Default().Handler(r),
	}
	go func() {
		if enableHTTPS {
			if err := httpSrv.ListenAndServeTLS(httpsCertFilePath, httpsPrivateKeyFilePath); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("https server get error, error: %+v", err))
			}
		} else {
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("http server get error, error: %+v", err))
			}
		}
	}()
	if enablePprofServer {
		go func() {
			if err := http.ListenAndServe(":9999", nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("pprof http server get error, error: %+v", err))
			}
		}()
	}
	if enableAutoPreviewTrading {
		go func() {
			if err := background.AsyncAutoPreviewTrading(
				ctx,
				autoPreviewTradingEmail,
				autoPreviewTradingPassword,
				time.Duration(autoPreviewTradingDuration)*time.Second,
				autoPreviewTradingMinOrderPrice,
				autoPreviewTradingMaxOrderPrice,
				autoPreviewTradingMinQuantity,
				autoPreviewTradingMaxQuantity,
				accountUseCase,
				tradingSequencerUseCase,
				currencyUseCase,
			); err != nil {
				logger.Fatal(fmt.Sprintf("auto preview trading get error, error: %+v", err))
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	httpSrv.Shutdown(ctx)
}
