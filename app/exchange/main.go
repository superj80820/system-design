package main

import (
	"context"
	"net/http"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
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
	sequencerRepo := sequencerMemoryRepo.CreateTradingSequencerRepo()

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel, loggerKit.NoStdout)
	if err != nil {
		panic(err)
	}

	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])
	tradingUseCase := trading.CreateTradingUseCase(ctx, matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase, tradingRepo, 100) // TODO: orderBookDepth use function?
	tradingAsyncUseCase := trading.CreateAsyncTradingUseCase(ctx, tradingUseCase, tradingRepo, logger)
	tradingSequencerUseCase := sequencer.CreateTradingSequencerUseCase(sequencerRepo, tradingRepo)
	asyncTradingSequencerUseCase := sequencer.CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase, tradingRepo)

	go background.RunAsyncTradingSequencer(ctx, asyncTradingSequencerUseCase)
	go background.RunAsyncTrading(ctx, tradingAsyncUseCase)

	r := mux.NewRouter()
	r.Methods("POST").Path("/api/v1/order").Handler(
		httptransport.NewServer(
			httpDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase),
			httpDelivery.DecodeCreateOrderRequest,
			httpDelivery.EncodeCreateOrderResponse,
		),
	)

	httpSrv := http.Server{
		Addr:    ":9090",
		Handler: r,
	}
	httpSrv.ListenAndServe()
}
