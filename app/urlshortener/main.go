package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	redisKit "github.com/superj80820/system-design/kit/redis"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	deliveryHTTP "github.com/superj80820/system-design/urlshorter/url/delivery/http"
	"github.com/superj80820/system-design/urlshorter/url/usecase"
	"go.opentelemetry.io/otel/trace"
)

const (
	SYSTEM_NAME  = "system"
	SERVICE_NAME = "url_shorter"
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
	logger, err := loggerKit.NewLogger("./go.log", logLevel) // TODO: 實作檔案大小
	if err != nil {
		panic(err)
	}
	singletonDB, err := mysqlKit.CreateDB("root:password@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local")
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

	urlService, err := usecase.CreateURLService(singletonDB, singletonCache, logger)
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
	urlShortenHandler := httptransport.NewServer(
		customMiddleware(deliveryHTTP.MakeURLShortenEndpoint(urlService)),
		deliveryHTTP.DecodeURLShortenRequest,
		deliveryHTTP.EncodeURLShortenResponse,
		options...,
	)
	urlGetHandler := httptransport.NewServer(
		customMiddleware(deliveryHTTP.MakeURLGetEndpoint(urlService)),
		deliveryHTTP.DecodeURLGetRequests,
		deliveryHTTP.EncodeURLGetResponse,
		options...,
	)
	r.Methods("POST").Path("/api/v1/data/shorten").Handler(urlShortenHandler)
	r.Methods("GET").Path("/api/v1/shortUrl/{shortURL}").Handler(urlGetHandler)
	if enableMetric {
		r.Handle("/metrics", promhttp.Handler())
	}

	log.Fatal(http.ListenAndServe(":9091", r))
}
