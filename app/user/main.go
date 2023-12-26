package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	kitgrpc "github.com/go-kit/kit/transport/grpc"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	delivery "github.com/superj80820/system-design/auth/delivery"
	deliveryGRPC "github.com/superj80820/system-design/auth/delivery/grpc"
	deliveryHTTP "github.com/superj80820/system-design/auth/delivery/http"
	"github.com/superj80820/system-design/auth/usecase"
	chatDeliveryHTTP "github.com/superj80820/system-design/chat/delivery/http"
	chatRepository "github.com/superj80820/system-design/chat/repository"
	chatUseCase "github.com/superj80820/system-design/chat/usecase"
	"github.com/superj80820/system-design/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	mqKit "github.com/superj80820/system-design/kit/mq"
	mqReaderManagerKit "github.com/superj80820/system-design/kit/mq/reader_manager"
	mqWriterManagerKit "github.com/superj80820/system-design/kit/mq/writer_manager"
	ormKit "github.com/superj80820/system-design/kit/orm"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

const (
	SYSTEM_NAME              = "system"
	SERVICE_NAME             = "auth"
	ACCESS_TOKEN_SECRET_KEY  = "accessTokenSecretKey"
	REFRESH_TOKEN_SECRET_KEY = "refreshTokenSecretKey"

	kafkaURL                = "localhost:9092"
	channelMessageTopicName = "channel-message-topic"
	userMessageTopicName    = "user-message-topic"
	userStatusTopicName     = "user-status-topic"
	serviceName             = "auth-service"

	grpcAddr = ":8083"
)

func main() {
	var (
		enableTracer = utilKit.GetEnvBool("ENABLE_TRACER", false)
		enableMetric = utilKit.GetEnvBool("ENABLE_METRIC", false)
		env          = utilKit.GetEnvString("ENV", "development")
	)

	logLevel := loggerKit.InfoLevel
	if env == "development" {
		logLevel = loggerKit.DebugLevel
	}
	logger, err := loggerKit.NewLogger("./go.log", logLevel)
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
	singletonDB, err := ormKit.CreateDB(ormKit.UseMySQL("root:password@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"))
	if err != nil {
		panic(err)
	}
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
		mqKit.ConsumeByPartitionsBindObserver(mqReaderManagerKit.LastOffset),
		mqKit.ProduceWay(&mqWriterManagerKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userStatusTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		kafkaURL,
		userStatusTopicName,
		mqKit.ConsumeByGroupID(serviceName+":user_status", mqReaderManagerKit.LastOffset),
	)
	if err != nil {
		panic(err)
	}
	friendOnlineStatusTopic, err := mqKit.CreateMQTopic( // TODO: need?
		context.TODO(),
		kafkaURL,
		userStatusTopicName,
		mqKit.ConsumeByGroupID(serviceName+":friend_online_status", mqReaderManagerKit.LastOffset),
	)
	if err != nil {
		panic(err)
	}

	chatRepo, err := chatRepository.CreateChatRepo(
		mongoDB,
		singletonDB,
		channelMessageTopic,
		userMessageTopic,
		userStatusTopic,
		friendOnlineStatusTopic,
	)
	if err != nil {
		panic(err)
	}

	rateLimit := utilKit.CreateCacheRateLimit(singletonCache, 3, 10)
	var tracer trace.Tracer
	if enableTracer {
		tracer, err = traceKit.CreateTracer(context.Background(), SERVICE_NAME)
		if err != nil {
			panic(err)
		}
	} else {
		tracer = traceKit.CreateNoOpTracer()
	}

	accountService, err := usecase.CreateAccountService(singletonDB, logger)
	if err != nil {
		panic(err)
	}
	authService, err := usecase.CreateAuthService(singletonDB, logger)
	if err != nil {
		panic(err)
	}
	chat := chatUseCase.CreateChatUseCase(chatRepo, logger)

	customMiddleware := endpoint.Chain(
		httpMiddlewareKit.CreateLoggingMiddleware(logger),
		httpMiddlewareKit.CreateRateLimitMiddleware(rateLimit.Pass),
		httpMiddlewareKit.CreateMetrics(SYSTEM_NAME, SERVICE_NAME),
	)
	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		return authService.Verify(token)
	})

	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		panic(err)
	}

	g := new(run.Group)
	{
		baseServer := grpc.NewServer(grpc.UnaryInterceptor(kitgrpc.Interceptor))
		g.Add(func() error {
			domain.RegisterAuthServer(baseServer, deliveryGRPC.CreateAuthServer(authService))
			return baseServer.Serve(grpcListener)
		}, func(err error) {
			if err != nil {
				logger.Error(err.Error()) // TODO: error
			}
			grpcListener.Close()
		})
	}
	{
		r := mux.NewRouter()
		options := []httptransport.ServerOption{
			httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
			httptransport.ServerAfter(httpKit.CustomAfterCtx),
			httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
		}
		r.Methods("POST").Path("/api/v1/user/register").Handler( // TODO: 須用複數嗎
			httptransport.NewServer(
				customMiddleware(deliveryHTTP.MakeAccountRegisterEndpoint(accountService)),
				deliveryHTTP.DecodeAccountRegisterRequest,
				deliveryHTTP.EncodeAccountRegisterResponse,
				options...,
			))
		r.Methods("POST").Path("/api/v1/user/friend").Handler( // TODO: 須用複數嗎
			httptransport.NewServer(
				customMiddleware(authMiddleware(chatDeliveryHTTP.MakeChatAddFriendEndpoint(chat))),
				chatDeliveryHTTP.DecodeAddFriendRequest,
				chatDeliveryHTTP.EncodeAddFriendResponse,
				options...,
			))
		r.Methods("POST").Path("/api/v1/auth/login").Handler(
			httptransport.NewServer(
				customMiddleware(deliveryHTTP.MakeAuthLoginEndpoint(authService)),
				deliveryHTTP.DecodeAuthLoginRequest,
				deliveryHTTP.EncodeAuthLoginResponse,
				options...,
			))
		r.Methods("POST").Path("/api/v1/auth/logout").Handler(
			httptransport.NewServer(
				customMiddleware(deliveryHTTP.MakeAuthLogoutEndpoint(authService)),
				deliveryHTTP.DecodeAuthLogoutRequest,
				deliveryHTTP.EncodeAuthLogoutResponse,
				options...,
			))
		r.Methods("POST").Path("/api/v1/auth/verify").Handler(
			httptransport.NewServer(
				customMiddleware(delivery.MakeAuthVerifyEndpoint(authService)),
				deliveryHTTP.DecodeAuthVerifyRequest,
				deliveryHTTP.EncodeAuthVerifyResponse,
				options...,
			))
		r.Methods("POST").Path("/api/v1/auth/refresh").Handler(
			httptransport.NewServer(
				customMiddleware(deliveryHTTP.MakeRefreshAccessTokenEndpoint(authService)),
				deliveryHTTP.DecodeRefreshAccessTokenRequest,
				deliveryHTTP.EncodeRefreshAccessTokenResponse,
				options...,
			))
		if enableMetric {
			r.Handle("/metrics", promhttp.Handler())
		}
		httpSrv := http.Server{
			Addr:    ":9093",
			Handler: r,
		}
		g.Add(func() error {
			return httpSrv.ListenAndServe()
		}, func(err error) {
			if err != nil {
				logger.Error(err.Error()) // TODO: error
			}
			httpSrv.Close()
		})
	}
	g.Run()
}
