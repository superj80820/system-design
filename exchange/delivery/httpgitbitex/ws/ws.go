package ws

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
	utilKit "github.com/superj80820/system-design/kit/util"
)

// response example:
//
//	{
//		"close24h": "2",
//		"high24h": "2",
//		"lastSize": "1",
//		"low24h": "2",
//		"open24h": "2",
//		"price": "2",
//		"productId": "BTC-USDT",
//		"sequence": 0,
//		"side": "buy",
//		"time": "2024-02-05T10:41:10.564Z",
//		"tradeId": 1,
//		"type": "ticker",
//		"volume24h": "1",
//		"volume30d": "1"
//	  }
type tradingTickNotify struct {
	Close24H  string    `json:"close24h"`
	High24H   string    `json:"high24h"`
	LastSize  string    `json:"lastSize"`
	Low24H    string    `json:"low24h"`
	Open24H   string    `json:"open24h"`
	Price     string    `json:"price"`
	ProductID string    `json:"productId"`
	Sequence  int       `json:"sequence"`
	Side      string    `json:"side"`
	Time      time.Time `json:"time"`
	TradeID   int       `json:"tradeId"`
	Type      string    `json:"type"`
	Volume24H string    `json:"volume24h"`
	Volume30D string    `json:"volume30d"`
}

// response example:
//
//	{
//		"available": "999999999",
//		"currencyCode": "BTC",
//		"hold": "1",
//		"type": "funds",
//		"userId": "11fa31dd-4933-4caf-9c67-5787c9fe6f21"
//	}
type tradingFoundsNotify struct {
	Available    string `json:"available"`
	CurrencyCode string `json:"currencyCode"`
	Hold         string `json:"hold"`
	Type         string `json:"type"`
	UserID       string `json:"userId"`
}

// response example:
//
//	{
//		"makerOrderId": "b60cdaae-6d2b-4bc9-9bb9-92f0ac48d718",
//		"price": "29",
//		"productId": "BTC-USDT",
//		"sequence": 40,
//		"side": "buy",
//		"size": "1",
//		"takerOrderId": "d4d13109-a7a1-47ec-bc58-cb58c4840695",
//		"time": "2024-02-05T16:34:21.455Z",
//		"tradeId": 5,
//		"type": "match"
//	}
type tradingMatchNotify struct {
	MakerOrderID string    `json:"makerOrderId"`
	Price        string    `json:"price"`
	ProductID    string    `json:"productId"`
	Sequence     int       `json:"sequence"`
	Side         string    `json:"side"`
	Size         string    `json:"size"`
	TakerOrderID string    `json:"takerOrderId"`
	Time         time.Time `json:"time"`
	TradeID      int       `json:"tradeId"`
	Type         string    `json:"type"`
}

// response example:
//
//	{
//		"createdAt": "2024-02-05T16:34:13.488Z",
//		"executedValue": "29",
//		"fillFees": "0",
//		"filledSize": "1",
//		"funds": "29",
//		"id": "b60cdaae-6d2b-4bc9-9bb9-92f0ac48d718",
//		"orderType": "limit",
//		"price": "29",
//		"productId": "BTC-USDT",
//		"settled": false,
//		"side": "buy",
//		"size": "1",
//		"status": "filled",
//		"type": "order",
//		"userId": "11fa31dd-4933-4caf-9c67-5787c9fe6f21"
//	}
type tradingOrderNotify struct {
	CreatedAt     time.Time `json:"createdAt"`
	ExecutedValue string    `json:"executedValue"`
	FillFees      string    `json:"fillFees"`
	FilledSize    string    `json:"filledSize"`
	Funds         string    `json:"funds"`
	ID            string    `json:"id"`
	OrderType     string    `json:"orderType"`
	Price         string    `json:"price"`
	ProductID     string    `json:"productId"`
	Settled       bool      `json:"settled"`
	Side          string    `json:"side"`
	Size          string    `json:"size"`
	Status        string    `json:"status"`
	Type          string    `json:"type"`
	UserID        string    `json:"userId"`
}

type tradingOrderBookNotify struct {
	Asks      [][]float64 `json:"asks"`
	Bids      [][]float64 `json:"bids"`
	ProductID string      `json:"productId"`
	Sequence  int         `json:"sequence"`
	Time      int64       `json:"time"`
	Type      string      `json:"type"`
}

type tradingPongNotify struct {
	Type string `json:"type"`
}

func MakeExchangeEndpoint(tradingUseCase domain.TradingUseCase, authUseCase domain.AuthUseCase) endpoint.BiStream[domain.TradingNotifyRequest, domain.TradingNotifyResponse] {
	return func(ctx context.Context, s endpoint.Stream[domain.TradingNotifyRequest, domain.TradingNotifyResponse]) error {
		userIDInt64, err := authUseCase.Verify(httpKit.GetToken(ctx))
		if err != nil {
			// TODO: error
			return tradingUseCase.NotifyForPublic(ctx, s)
		}
		userID, err := utilKit.SafeInt64ToInt(userIDInt64)
		if err != nil {
			return errors.Wrap(err, "safe int64 to int failed")
		}
		return tradingUseCase.NotifyForUser(ctx, userID, s)
	}
}

var DecodeStreamExchangeRequest = func(ctx context.Context, messageType endpoint.MessageType, data []byte) (domain.TradingNotifyRequest, error) {
	var req domain.TradingNotifyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		var noop domain.TradingNotifyRequest
		return noop, errors.Wrap(err, "json unmarshal failed")
	}
	for idx := range req.Channels {
		if req.Channels[idx] == "funds" {
			req.Channels[idx] = string(domain.AssetsExchangeRequestType)
		}
	}
	return req, nil
}

func EncodeStreamExchangeResponse(ctx context.Context, resp domain.TradingNotifyResponse) ([]byte, endpoint.MessageType, error) {
	var response any
	switch resp.Type {
	case domain.AssetExchangeResponseType:
		response = tradingFoundsNotify{
			Available:    resp.UserAsset.Available.String(),
			CurrencyCode: resp.UserAsset.CurrencyName,
			Hold:         resp.UserAsset.Frozen.String(),
			Type:         "funds",
			UserID:       strconv.Itoa(resp.UserAsset.UserID),
		}
	case domain.TickerExchangeResponseType:
		response = tradingTickNotify{
			// Close24H  : TODO
			// High24H   : TODO
			// LastSize  : TODO
			// Low24H    : TODO
			// Open24H   : TODO
			Price:     resp.Tick.Price.String(),
			ProductID: resp.ProductID,
			Sequence:  resp.Tick.SequenceID,
			Side:      resp.Tick.TakerDirection.String(),
			Time:      resp.Tick.CreatedAt,
			// TradeID   : TODO
			Type: string(domain.TickerExchangeRequestType),
			// Volume24H : TODO
			// Volume30D : TODO
		}
	case domain.MatchExchangeResponseType:
		response = tradingMatchNotify{
			// MakerOrderID : TODO
			Price:        resp.MatchOrderDetail.Price.String(),
			ProductID:    resp.ProductID,
			Sequence:     resp.MatchOrderDetail.SequenceID,
			Side:         resp.MatchOrderDetail.Direction.String(),
			Size:         resp.MatchOrderDetail.Quantity.String(),
			TakerOrderID: strconv.Itoa(resp.MatchOrderDetail.UserID),
			Time:         resp.MatchOrderDetail.CreatedAt,
			// TradeID      : TODO
			Type: string(domain.MatchExchangeRequestType),
		}
	case domain.OrderBookExchangeResponseType:
		orderBookNotify := tradingOrderBookNotify{
			ProductID: resp.ProductID,
			Sequence:  resp.OrderBook.SequenceID,
			Time:      time.Now().UnixMilli(),
			Type:      "snapshot",           // TODO: workaround
			Bids:      make([][]float64, 0), // TODO: client need zero value
			Asks:      make([][]float64, 0), // TODO: client need zero value
		}
		for _, val := range resp.OrderBook.Buy {
			orderBookNotify.Bids = append(orderBookNotify.Bids, []float64{val.Price.InexactFloat64(), val.Quantity.InexactFloat64(), 1}) // TODO: 1 is workaround
		}
		for _, val := range resp.OrderBook.Sell {
			orderBookNotify.Asks = append(orderBookNotify.Asks, []float64{val.Price.InexactFloat64(), val.Quantity.InexactFloat64(), 1}) // TODO: 1 is workaround
		}

		response = orderBookNotify
	case domain.OrderExchangeResponseType:
		response = tradingOrderNotify{
			CreatedAt:     resp.Order.CreatedAt,
			ExecutedValue: resp.Order.Quantity.Sub(resp.Order.UnfilledQuantity).Mul(resp.Order.Price).String(),
			FillFees:      "0",
			FilledSize:    resp.Order.Quantity.Sub(resp.Order.UnfilledQuantity).String(),
			Funds:         resp.Order.Quantity.Mul(resp.Order.Price).String(),
			ID:            strconv.Itoa(resp.Order.ID),
			OrderType:     "limit",
			Price:         resp.Order.Price.String(),
			ProductID:     resp.ProductID,
			// Settled       : TODO: what this?
			Side: resp.Order.Direction.String(),
			Size: resp.Order.Quantity.String(),
			Status: func(status string) string {
				if status == "pending" {
					return "open"
				} else if status == "fully-filled" {
					return "filled"
				}
				return status
			}(resp.Order.Status.String()),
			Type:   string(domain.OrderExchangeRequestType),
			UserID: strconv.Itoa(resp.Order.UserID),
		}
	case domain.PongExchangeResponseType:
		response = tradingPongNotify{
			Type: "pong",
		}
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, 0, errors.Wrap(err, "json marshal failed")
	}
	return data, websocket.TextMessage, nil
}
