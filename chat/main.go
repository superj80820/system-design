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
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:30001,localhost:30002,localhost:30003/?replicaSet=my-replica-set"))
	if err != nil {
		panic(err) // TODO
	}
	defer func() { // TODO: sequence
		if err := client.Disconnect(ctx); err != nil {
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
		mqKit.ConsumeByPartitionsBindObserver(mqKit.LastOffset),
		mqKit.ProduceWay(&mqKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	// userMessageTopic, err := mqKit.CreateMQTopic(
	// 	context.TODO(),
	// 	kafkaURL,
	// 	userMessageTopicName,
	// 	mqKit.ConsumeByGroupID(*groupID, mqKit.LastOffset),
	// )
	// if err != nil {
	// 	panic(err)
	// }
	// userStatusTopic, err := mqKit.CreateMQTopic(
	// 	context.TODO(),
	// 	kafkaURL,
	// 	userStatusTopicName,
	// )
	// if err != nil {
	// 	panic(err)
	// }

	rateLimit := utilKit.CreateCacheRateLimit(singletonCache, 3, 10)

	tracer := traceKit.CreateNoOpTracer()

	chatRepo := repository.MakeChatRepo(client)

	chatUseCase := usecase.MakeChatUseCase(chatRepo, channelMessageTopic, nil, nil)

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
