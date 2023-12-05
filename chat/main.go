package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	wsDelivery "github.com/superj80820/system-design/chat/chat/delivery/http/websocket"
	"github.com/superj80820/system-design/kit/core/endpoint"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	wsMiddleware "github.com/superj80820/system-design/kit/http/websocket/middleware"
	mqReaderManagerKit "github.com/superj80820/system-design/kit/mq/reader_manager"
	mqWriterManagerKit "github.com/superj80820/system-design/kit/mq/writer_manager"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"

	"github.com/superj80820/system-design/chat/chat/repository"
	"github.com/superj80820/system-design/chat/chat/usecase"
	"github.com/superj80820/system-design/chat/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	mqKit "github.com/superj80820/system-design/kit/mq"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	addr                    = flag.String("addr", "localhost:8080", "http server address")
	groupID                 = flag.String("group-id", "chat-service", "kafka consume group id")
	kafkaURL                = "localhost:9092"
	channelMessageTopicName = "quickstart-events-par-3"
	userMessageTopicName    = "user-message"
	userStatusTopicName     = "user-status"
	serviceName             = "chat-service"
)

func main() {
	flag.Parse()

	mysqlDB, err := mysqlKit.CreateDB("root:example@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:30001,localhost:30002,localhost:30003/?replicaSet=my-replica-set"))
	if err != nil {
		panic(err) // TODO
	}
	defer func() { // TODO: sequence
		if err := mongoDB.Disconnect(ctx); err != nil {
			panic(err) // TODO
		}
	}()

	singletonCache, err := redisKit.CreateCache("localhost:6379", "", 0)
	if err != nil {
		panic(err)
	}
	channelMessageTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		channelMessageTopicName,
		mqKit.ConsumeByPartitionsBindObserver(mqReaderManagerKit.LastOffset),
		mqKit.ProduceWay(&mqWriterManagerKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userMessageTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		userMessageTopicName,
		mqKit.ConsumeByGroupID(*groupID, mqReaderManagerKit.LastOffset),
	)
	if err != nil {
		panic(err)
	}
	userStatusTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		userStatusTopicName,
		mqKit.ConsumeByGroupID(serviceName, mqReaderManagerKit.LastOffset),
	)
	if err != nil {
		panic(err)
	}

	rateLimit := utilKit.CreateCacheRateLimit(singletonCache, 5, 10)

	tracer := traceKit.CreateNoOpTracer()

	chatRepo, err := repository.CreateChatRepo(mongoDB, mysqlDB)
	if err != nil {
		panic(err)
	}

	chatUseCase := usecase.MakeChatUseCase(
		chatRepo,
		channelMessageTopic,
		userMessageTopic,
		userStatusTopic,
	)

	r := mux.NewRouter()
	r.Handle("/ws/channel",
		wsTransport.NewServer(
			customMiddleware[domain.ChatRequest, domain.ChatResponse](rateLimit)(wsDelivery.MakeChatEndpoint(chatUseCase)),
			wsKit.JsonDecodeRequest[domain.ChatRequest],
			wsKit.JsonEncodeResponse[domain.ChatResponse],
			wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
			wsTransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
			wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()),
		))
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func customMiddleware[IN, OUT any](rateLimit *utilKit.CacheRateLimit) endpoint.Middleware[IN, OUT] {
	return endpoint.Chain(
		wsMiddleware.CreateRateLimit[IN, OUT](rateLimit.Pass),
		wsMiddleware.CreateAuth[IN, OUT](),
	)
}
