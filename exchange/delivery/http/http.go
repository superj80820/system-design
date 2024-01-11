package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type createOrderRequest struct {
	UserID    int
	Direction domain.DirectionEnum
	Price     int
	Quantity  int
}

var (
	DecodeCreateOrderRequest  = httpTransportKit.DecodeJsonRequest[createOrderRequest]
	EncodeCreateOrderResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
)

func MakeCreateOrderEndpoint(svc domain.TradingSequencerUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(createOrderRequest)
		svc.SendTradeSequenceMessages(&domain.TradingEvent{
			EventType: domain.TradingEventCreateOrderType,
			OrderRequestEvent: &domain.OrderRequestEvent{
				UserID:    req.UserID,
				Direction: req.Direction,
				Price:     decimal.NewFromInt(int64(req.Price)),
				Quantity:  decimal.NewFromInt(int64(req.Quantity)),
			},
		})
		return nil, nil
	}
}
