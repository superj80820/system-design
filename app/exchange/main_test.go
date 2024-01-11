package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"fmt"
	"net/http"
	"strings"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
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

func TestServer(t *testing.T) {
	currencyMap := map[string]int{
		"BTC":  1,
		"USDT": 2,
	}

	ctx := context.Background()
	tradingRepo := tradingMemoryRepo.CreateTradingRepo(ctx)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	sequencerRepo := sequencerMemoryRepo.CreateTradingSequencerRepo(ctx)

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
	asyncTradingSequencerUseCase := sequencer.CreateAsyncTradingSequencerUseCase(tradingSequencerUseCase, tradingRepo, sequencerRepo)

	go background.RunAsyncTradingSequencer(ctx, asyncTradingSequencerUseCase)
	go background.RunAsyncTrading(ctx, tradingAsyncUseCase)

	assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(2, currencyMap["BTC"], decimal.NewFromInt(100000)))
	assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(2, currencyMap["USDT"], decimal.NewFromInt(100000)))

	orderBookServer := httptest.NewServer(httptransport.NewServer(
		httpDelivery.MakeGetOrderBookEndpoint(matchingUseCase),
		httpDelivery.DecodeGetOrderBookRequest,
		httpDelivery.EncodeGetOrderBookResponse,
	))
	createOrderServer := httptest.NewServer(httptransport.NewServer(
		httpDelivery.MakeCreateOrderEndpoint(tradingSequencerUseCase),
		httpDelivery.DecodeCreateOrderRequest,
		httpDelivery.EncodeCreateOrderResponse,
	))
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2082.34, "quantity": 1 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 2, "price": 2087.6, "quantity": 2 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2087.8, "quantity": 1 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2085.01, "quantity": 5 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 2, "price": 2088.02, "quantity": 3 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 2, "price": 2087.6, "quantity": 6 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2081.11, "quantity": 7 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2086, "quantity": 3 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2088.33, "quantity": 1 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 2, "price": 2086.54, "quantity": 2 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 2, "price": 2086.55, "quantity": 5 }`)
	createOrderRequest(createOrderServer.URL, `{ "user_id": 2, "direction": 1, "price": 2086.55, "quantity": 3 }`)

	// test all
	{
		orderBook, err := getOrderBookRequest(orderBookServer.URL, "10")
		assert.Nil(t, err)
		buyExpected := []struct {
			price    string
			quantity string
		}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}, {price: "2082.34", quantity: "1"}, {price: "2081.11", quantity: "7"}}
		for idx, val := range orderBook.Buy {
			assert.Equal(t, buyExpected[idx].price, val.Price.String())
			assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
		}
		sellExpected := []struct {
			price    string
			quantity string
		}{{price: "2086.55", quantity: "4"}, {price: "2087.6", quantity: "6"}, {price: "2088.02", quantity: "3"}}
		for idx, val := range orderBook.Sell {
			assert.Equal(t, sellExpected[idx].price, val.Price.String())
			assert.Equal(t, sellExpected[idx].quantity, val.Quantity.String())
		}
	}

	// test with max depth
	{
		orderBook, err := getOrderBookRequest(orderBookServer.URL, "2")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(orderBook.Buy))
		assert.Equal(t, 2, len(orderBook.Sell))
		buyExpected := []struct {
			price    string
			quantity string
		}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}}
		for idx, val := range orderBook.Buy {
			assert.Equal(t, buyExpected[idx].price, val.Price.String())
			assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
		}
	}
}

func getOrderBookRequest(url, maxDepth string) (*domain.OrderBookEntity, error) {
	url = url + "?max_depth=" + maxDepth
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.New("new request failed")
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("do request failed")
	}
	defer res.Body.Close()

	var orderBook domain.OrderBookEntity
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("read body failed")
	}
	if err := json.Unmarshal(body, &orderBook); err != nil {
		return nil, errors.New("unmarshal json failed")
	}

	return &orderBook, nil
}

func createOrderRequest(url, payloadRawData string) {
	method := "POST"

	payload := strings.NewReader(payloadRawData)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()
}
