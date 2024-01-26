package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type createOrderRequest struct {
	Direction domain.DirectionEnum `json:"direction"`
	Price     decimal.Decimal      `json:"price"`    // TODO: is safe?
	Quantity  decimal.Decimal      `json:"quantity"` // TODO: is safe?
}

type cancelOrderRequest struct {
	OrderID int
}

type getOrderBookRequest struct {
	MaxDepth int
}

type getUserOrderRequest struct {
	OrderID int
}

var (
	DecodeGetSecBarRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetSecBarResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetMinBarRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetMinBarResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetHourBarRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetHourBarResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetDayBarRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetDayBarResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetTickRequests = httpTransportKit.DecodeEmptyRequest
	EncodeGetTickResponse = httpTransportKit.EncodeJsonResponse

	EncodeGetUserOrderResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetUserAssetsRequests = httpTransportKit.DecodeEmptyRequest
	EncodeGetUserAssetsResponse = httpTransportKit.EncodeJsonResponse

	DecodeGetUserOrdersRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetUserOrdersResponse = httpTransportKit.EncodeJsonResponse

	EncodeCancelOrderResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)

	DecodeCreateOrderRequest  = httpTransportKit.DecodeJsonRequest[createOrderRequest]
	EncodeCreateOrderResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)

	EncodeGetOrderBookResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)
)

func MakeCreateOrderEndpoint(svc domain.TradingSequencerUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(createOrderRequest)
		if err := svc.ProduceTradingEvent(ctx, &domain.TradingEvent{
			EventType: domain.TradingEventCreateOrderType,
			OrderRequestEvent: &domain.OrderRequestEvent{
				UserID:    userID,
				Direction: req.Direction,
				Price:     req.Price,
				Quantity:  req.Quantity,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		return nil, nil
	}
}

func MakeGetUserAssetsEndpoint(svc domain.UserAssetUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		userAssets, err := svc.GetAssets(userID)
		if err != nil {
			return nil, errors.Wrap(err, "get user assets failed")
		}
		return userAssets, nil
	}
}

func MakeGetUserOrderEndpoint(svc domain.OrderUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(getUserOrderRequest)
		order, err := svc.GetOrder(req.OrderID)
		if err != nil {
			return nil, errors.Wrap(err, "get order failed")
		}
		return order, nil
	}
}

func MakeGetSecBarEndpoint(svc domain.CandleUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		end := time.Now().UnixMilli()
		start := end - 3600*1000
		bars, err := svc.GetBar(ctx, domain.CandleTimeTypeSec, strconv.FormatInt(start, 10), strconv.FormatInt(end, 10))
		if err != nil {
			return nil, errors.Wrap(err, "get bar failed")
		}
		return bars, nil
	}
}

func MakeGetMinBarEndpoint(svc domain.CandleUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		end := time.Now().UnixMilli()
		start := end - 1440*60000
		bars, err := svc.GetBar(ctx, domain.CandleTimeTypeMin, strconv.FormatInt(start, 10), strconv.FormatInt(end, 10))
		if err != nil {
			return nil, errors.Wrap(err, "get bar failed")
		}
		return bars, nil
	}
}

func MakeGetHourBarEndpoint(svc domain.CandleUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		end := time.Now().UnixMilli()
		start := end - 720*3600000
		bars, err := svc.GetBar(ctx, domain.CandleTimeTypeHour, strconv.FormatInt(start, 10), strconv.FormatInt(end, 10))
		if err != nil {
			return nil, errors.Wrap(err, "get bar failed")
		}
		return bars, nil
	}
}

func MakeGetDayBarEndpoint(svc domain.CandleUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		end := time.Now().UnixMilli()
		start := end - 366*86400000
		bars, err := svc.GetBar(ctx, domain.CandleTimeTypeDay, strconv.FormatInt(start, 10), strconv.FormatInt(end, 10))
		if err != nil {
			return nil, errors.Wrap(err, "get bar failed")
		}
		return bars, nil
	}
}

func MakeGetTickEndpoint(svc domain.QuotationUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		ticks, err := svc.GetTicks()
		if err != nil {
			return nil, errors.Wrap(err, "get tick failed")
		}
		return ticks, nil
	}
}

func MakeCancelOrderEndpoint(svc domain.TradingSequencerUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(cancelOrderRequest)
		if err := svc.ProduceTradingEvent(ctx, &domain.TradingEvent{
			EventType: domain.TradingEventCancelOrderType,
			OrderCancelEvent: &domain.OrderCancelEvent{
				UserID:  userID,
				OrderId: req.OrderID,
			},
		}); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
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

func MakeGetUserOrdersEndpoint(svc domain.OrderUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		userOrders, err := svc.GetUserOrders(userID)
		if err != nil {
			return nil, errors.Wrap(err, "get user order failed")
		}
		userOrdersSlice := make([]*domain.OrderEntity, 0, len(userOrders))
		for _, val := range userOrders {
			userOrder := val
			userOrdersSlice = append(userOrdersSlice, userOrder)
		}
		return userOrdersSlice, nil
	}
}

func DecodeCancelOrderRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	orderIDString, ok := vars["orderID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	orderID, err := strconv.Atoi(orderIDString)
	if err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	return cancelOrderRequest{OrderID: orderID}, nil
}

func DecodeGetUserOrderRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	orderIDString, ok := vars["orderID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	orderID, err := strconv.Atoi(orderIDString)
	if err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	return getUserOrderRequest{OrderID: orderID}, nil
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