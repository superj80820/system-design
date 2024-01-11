package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-kit/kit/endpoint"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type createOrderRequest struct {
	UserID    int                  `json:"user_id"` // TODO
	Direction domain.DirectionEnum `json:"direction"`
	Price     decimal.Decimal      `json:"price"`    // TODO: is safe?
	Quantity  decimal.Decimal      `json:"quantity"` // TODO: is safe?
}

type getOrderBookRequest struct {
	MaxDepth int
}

var (
	DecodeCreateOrderRequest  = httpTransportKit.DecodeJsonRequest[createOrderRequest]
	EncodeCreateOrderResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)

	EncodeGetOrderBookResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)
)

func MakeCreateOrderEndpoint(svc domain.TradingSequencerUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(createOrderRequest)
		svc.SendTradeSequenceMessages(&domain.TradingEvent{
			EventType: domain.TradingEventCreateOrderType,
			OrderRequestEvent: &domain.OrderRequestEvent{
				UserID:    req.UserID,
				Direction: req.Direction,
				Price:     req.Price,
				Quantity:  req.Quantity,
			},
		})
		return nil, nil
	}
}

func MakeGetOrderBookEndpoint(svc domain.MatchingUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getOrderBookRequest) // TODO: maybe no need?
		orderBook := svc.GetOrderBook(req.MaxDepth)
		return orderBook, nil
	}
}

func DecodeGetOrderBookRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	maxDepth := r.URL.Query().Get("max_depth")
	if maxDepth == "" {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get max depth failed"))
	}
	maxDepthInt, err := strconv.Atoi(maxDepth)
	if err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("max depth format error"))
	}
	return getOrderBookRequest{MaxDepth: maxDepthInt}, nil
}
