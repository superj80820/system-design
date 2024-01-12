package main

import (
	"context"
	"net/http"
	"strconv"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/exchange/delivery/background"
	httpDelivery "github.com/superj80820/system-design/exchange/delivery/http"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	sequencerMemoryRepo "github.com/superj80820/system-design/exchange/repository/sequencer/memory"
	tradingMemoryRepo "github.com/superj80820/system-design/exchange/repository/trading/memory"
	"github.com/superj80820/system-design/exchange/usecase/asset"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	"github.com/superj80820/system-design/exchange/usecase/sequencer"
	"github.com/superj80820/system-design/exchange/usecase/trading"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

func main() {
	currencyMap := map[string]int{
		"BTC":  1,
		"USDT": 2,
	} // TODO

	ctx := context.Background()
	tradingRepo := tradingMemoryRepo.CreateTradingRepo(ctx)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	sequencerRepo := sequencerMemoryRepo.CreateTradingSequencerRepo(ctx)

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	if err != nil {
		panic(err)
	}

	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
	tradingUseCase := trading.CreateTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo) // TODO: orderBookDepth use function?
	tradingAsyncUseCase := trading.CreateAsyncTradingUseCase(ctx, tradingUseCase, tradingRepo, matchingUseCase, 100, logger)            //TODO:100?
	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(sequencerRepo, tradingRepo)
	asyncTradingSequencerUseCase := sequencer.CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase, tradingRepo, sequencerRepo)

	go background.RunAsyncTradingSequencer(ctx, asyncTradingSequencerUseCase)
	go background.RunAsyncTrading(ctx, tradingAsyncUseCase)

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
	// r.Methods("GET").Path("/api/v1/ticks").Handler()
	// r.Methods("GET").Path("/api/v1/bars/day").Handler()
	// r.Methods("GET").Path("/api/v1/bars/hour").Handler()
	// r.Methods("GET").Path("/api/v1/bars/min").Handler()
	// r.Methods("GET").Path("/api/v1/bars/sec").Handler()
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
	httpSrv.ListenAndServe()
}
