package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	httptransport "github.com/go-kit/kit/transport/http"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	redisKit "github.com/superj80820/system-design/kit/redis"
	utilKit "github.com/superj80820/system-design/kit/util"
	deliveryHTTP "github.com/superj80820/system-design/url-shorter/url/delivery/http"
	"github.com/superj80820/system-design/url-shorter/url/delivery/http/middleware"
	"github.com/superj80820/system-design/url-shorter/url/usecase"
)

func main() {
	logger := loggerKit.NewLogger(nil)
	singletonDB, err := mysqlKit.CreateSingletonDB("root:example@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		panic(err)
	}
	singletonCache, err := redisKit.CreateSingletonCache("localhost:6379", "", 0)
	if err != nil {
		panic(err)
	}

	rateLimit := utilKit.CreateCacheRateLimit(singletonCache, 3, 10)

	urlService, err := usecase.CreateURLService(singletonDB, singletonCache, logger)
	if err != nil {
		panic(err)
	}

	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomCtx),
		httptransport.ServerErrorEncoder(deliveryHTTP.EncodeError(logger)),
	}
	urlSVC := middleware.CreateRateLimitMiddleware(urlService, rateLimit.Pass)
	urlShortenHandler := httptransport.NewServer(
		deliveryHTTP.MakeURLShortenEndpoint(urlSVC),
		deliveryHTTP.DecodeURLShortenRequests,
		deliveryHTTP.EncodeURLShortenResponse,
		options...,
	)
	urlGetHandler := httptransport.NewServer(
		deliveryHTTP.MakeURLGetEndpoint(urlSVC),
		deliveryHTTP.DecodeURLGetRequests,
		deliveryHTTP.EncodeURLGetResponse,
		options...,
	)
	r.Methods("POST").Path("/api/v1/data/shorten").Handler(urlShortenHandler)
	r.Methods("GET").Path("/api/v1/shortUrl/{shortURL}").Handler(urlGetHandler)
	log.Fatal(http.ListenAndServe(":9090", r))
}
