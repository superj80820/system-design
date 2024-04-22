package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	wsDelivery "github.com/superj80820/system-design/chat/delivery/http/websocket"
	"github.com/superj80820/system-design/kit/core/endpoint"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	wsMiddleware "github.com/superj80820/system-design/kit/http/websocket/middleware"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	kafkaReaderManagerMQKit "github.com/superj80820/system-design/kit/mq/kafka/readermanager"
	kafkaWriterManagerMQKit "github.com/superj80820/system-design/kit/mq/kafka/writermanager"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"

	"github.com/gorilla/mux"
	authHttpRepo "github.com/superj80820/system-design/auth/repository/http"
	"github.com/superj80820/system-design/chat/repository"
	"github.com/superj80820/system-design/chat/usecase"
	"github.com/superj80820/system-design/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	addr                    = *flag.String("addr", "localhost:8080", "http server address")
	kafkaURL                = *flag.String("kafkaURL", "localhost:9092", "kafka url")
	channelMessageTopicName = *flag.String("channelMessageTopicName", "channel-message-topic", "channel message topic name")
	userMessageTopicName    = *flag.String("userMessageTopicName", "user-message-topic", "user message topic name")
	userStatusTopicName     = *flag.String("userStatusTopicName", "user-status-topic", "user status topic name")
	serviceName             = *flag.String("serviceName", "user-service", "service name")
	env                     = *flag.String("env", "development", "env")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	// mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:30001,localhost:30002,localhost:30003/?replicaSet=my-replica-set"))
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
	if err != nil {
		panic(err)
	}

	mysqlDB, err := ormKit.CreateDB(ormKit.UseMySQL("root:password@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"))
	if err != nil {
		panic(err)
	}
	redisCache, err := redisKit.CreateCache("localhost:6379", "", 0)
	if err != nil {
		panic(err)
	}

	logLevel := loggerKit.InfoLevel
	if env == "development" {
		logLevel = loggerKit.DebugLevel
	}
	logger, err := loggerKit.NewLogger("./go.log", logLevel)
	if err != nil {
		panic(err)
	}
	channelMessageTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		channelMessageTopicName,
		kafkaMQKit.ConsumeByPartitionsBindObserver(kafkaReaderManagerMQKit.LastOffset),
		kafkaMQKit.ProduceWay(&kafkaWriterManagerMQKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userMessageTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		userMessageTopicName,
		kafkaMQKit.ConsumeByPartitionsBindObserver(kafkaReaderManagerMQKit.LastOffset),
		kafkaMQKit.ProduceWay(&kafkaWriterManagerMQKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userStatusTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		userStatusTopicName,
		kafkaMQKit.ConsumeByGroupID(serviceName+":user_status", false),
	)
	if err != nil {
		panic(err)
	}
	friendOnlineStatusTopic, err := kafkaMQKit.CreateMQTopic( // TODO: need?
		context.TODO(),
		kafkaURL,
		userStatusTopicName,
		kafkaMQKit.ConsumeByGroupID(serviceName+":friend_online_status", false),
	)
	if err != nil {
		panic(err)
	}
	rateLimit := utilKit.CreateCacheRateLimit(redisCache, 5, 10)
	tracer := traceKit.CreateNoOpTracer()

	authRepo := authHttpRepo.CreateAuthClient("http://localhost:9093")
	chatRepo, err := repository.CreateChatRepo(
		mongoDB,
		mysqlDB,
		channelMessageTopic,
		userMessageTopic,
		userStatusTopic,
		friendOnlineStatusTopic,
	)
	if err != nil {
		panic(err)
	}

	chatUseCase := usecase.CreateChatUseCase(chatRepo, logger)

	r := mux.NewRouter()
	r.Handle("/ws",
		wsTransport.NewServer(
			customMiddleware[*domain.ChatRequest, *domain.ChatResponse](rateLimit, authRepo)(wsDelivery.MakeChatEndpoint(chatUseCase)),
			wsKit.JsonDecodeRequest[*domain.ChatRequest],
			wsKit.JsonEncodeResponse[*domain.ChatResponse],
			wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
			wsTransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
			wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()),
		))

	srv := http.Server{
		Addr:    addr,
		Handler: r,
	}
	go func() {
		fmt.Println("run")
		if err := srv.ListenAndServe(); err != nil {
			logger.Error(fmt.Sprintf("close service failed, error: %+v", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if err := mongoDB.Disconnect(ctx); err != nil {
		panic(err)
	}
	srv.Shutdown(ctx)

}

func customMiddleware[IN, OUT any](
	rateLimit *utilKit.CacheRateLimit,
	authServiceRepo domain.AuthServiceRepo,
) endpoint.Middleware[IN, OUT] {
	return endpoint.Chain(
		wsMiddleware.CreateRateLimit[IN, OUT](rateLimit.Pass),
		wsMiddleware.CreateAuth[IN, OUT](authServiceRepo),
	)
}
