package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	deliveryHTTP "github.com/superj80820/system-design/auth/auth/delivery/http"
	"github.com/superj80820/system-design/auth/auth/usecase"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.opentelemetry.io/otel/trace"
)

const (
	SYSTEM_NAME              = "system"
	SERVICE_NAME             = "auth"
	ACCESS_TOKEN_SECRET_KEY  = "accessTokenSecretKey"
	REFRESH_TOKEN_SECRET_KEY = "refreshTokenSecretKey"
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

	singletonDB, err := mysqlKit.CreateDB("root:example@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		panic(err)
	}
	singletonCache, err := redisKit.CreateCache("localhost:6379", "", 0)
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

	customMiddleware := endpoint.Chain(
		httpMiddlewareKit.CreateLoggingMiddleware(logger),
		httpMiddlewareKit.CreateRateLimitMiddleware(rateLimit.Pass),
		httpMiddlewareKit.CreateMetrics(SYSTEM_NAME, SERVICE_NAME),
	)

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
			httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(deliveryHTTP.EncodeAccountRegisterResponse),
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/login").Handler(
		httptransport.NewServer(
			customMiddleware(deliveryHTTP.MakeAuthLoginEndpoint(authService)),
			deliveryHTTP.DecodeAuthLoginRequest,
			httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(deliveryHTTP.EncodeAuthLoginResponse),
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/logout").Handler(
		httptransport.NewServer(
			customMiddleware(deliveryHTTP.MakeAuthLogoutEndpoint(authService)),
			deliveryHTTP.DecodeAuthLogoutRequest,
			httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(deliveryHTTP.EncodeAuthLogoutResponse),
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/verify").Handler(
		httptransport.NewServer(
			customMiddleware(deliveryHTTP.MakeAuthVerifyEndpoint(authService)),
			deliveryHTTP.DecodeAuthVerifyRequest,
			httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(deliveryHTTP.EncodeAuthVerifyResponse),
			options...,
		))
	r.Methods("POST").Path("/api/v1/auth/refresh").Handler(
		httptransport.NewServer(
			customMiddleware(deliveryHTTP.MakeRefreshAccessTokenEndpoint(authService)),
			deliveryHTTP.DecodeRefreshAccessTokenRequest,
			httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(deliveryHTTP.EncodeRefreshAccessTokenResponse),
			options...,
		))
	if enableMetric {
		r.Handle("/metrics", promhttp.Handler())
	}

	log.Fatal(http.ListenAndServe(":9092", r))
}
