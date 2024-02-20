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
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type orderType string

const (
	limitOrderType orderType = "limit"
)

type sideType string

const (
	buySideType  sideType = "buy"
	sellSideType sideType = "sell"
)

type createOrderRequest struct {
	ProductID string    `json:"productId"`
	Side      sideType  `json:"side"`
	Type      orderType `json:"type"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Funds     float64   `json:"funds"`
}

type createOrderResponse struct {
	ID            string     `json:"id"`
	Price         *int64     `json:"price"`
	Size          *int64     `json:"size"`
	Funds         *int64     `json:"funds"`
	ProductID     *string    `json:"productId"`
	Side          *sideType  `json:"side"`
	Type          *orderType `json:"type"`
	CreatedAt     *time.Time `json:"createdAt"`
	FillFees      *int64     `json:"fillFees"`
	FilledSize    *int64     `json:"filledSize"`
	ExecutedValue *int64     `json:"executedValue"`
	Status        *string    `json:"status"`
}

type accountOrder struct {
	ID            string    `json:"id"`
	Price         string    `json:"price"`
	Size          string    `json:"size"`
	Funds         string    `json:"funds"`
	ProductID     string    `json:"productId"`
	Side          string    `json:"side"`
	Type          string    `json:"type"`
	CreatedAt     time.Time `json:"createdAt"`
	FillFees      any       `json:"fillFees,omitempty"`
	FilledSize    string    `json:"filledSize"`
	ExecutedValue string    `json:"executedValue"`
	Status        string    `json:"status"`
}

type getAccountOrdersRequest struct {
	ProductID string   `json:"productId"`
	Page      int      `json:"page"`
	PageSize  int      `json:"pageSize"`
	Status    []string `json:"status"`
}

type getAccountOrdersResponse struct {
	Items []*accountOrder `json:"items"`
	Count int             `json:"count"`
}

type getHistoryOrdersRequest struct {
	productID string
}

type historyOrder struct {
	Sequence int       `json:"sequence"`
	Time     time.Time `json:"time"`
	Price    string    `json:"price"`
	Size     string    `json:"size"`
	Side     string    `json:"side"`
}

type getHistoryOrdersResponse []struct {
	*historyOrder
}

var (
	EncodeGetAccountOrdersResponse = httpTransportKit.EncodeJsonResponse
	EncodeGetHistoryOrdersResponse = httpTransportKit.EncodeJsonResponse

	DecodeCreateOrderRequest  = httpTransportKit.DecodeJsonRequest[createOrderRequest]
	EncodeCreateOrderResponse = httpTransportKit.EncodeJsonResponse
)

// request example:
//
//	{
//	    "productId": "BTC-USDT",
//	    "side": "buy",
//	    "type": "limit",
//	    "price": 1,
//	    "size": 2,
//	    "funds": 2
//	}
//
// response example:
//
//	{
//	    "id": "96ccafcc-ee6a-43e1-b4bc-1acb0bc3aab2",
//	    "price": null,
//	    "size": null,
//	    "funds": null,
//	    "productId": null,
//	    "side": null,
//	    "type": null,
//	    "createdAt": null,
//	    "fillFees": null,
//	    "filledSize": null,
//	    "executedValue": null,
//	    "status": null
//	}
func MakeCreateOrderEndpoint(svc domain.TradingUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(createOrderRequest)
		if req.Type != limitOrderType {
			return nil, errors.New("not found order type")
		}
		direction := domain.DirectionUnknown
		switch req.Side {
		case buySideType:
			direction = domain.DirectionBuy
		case sellSideType:
			direction = domain.DirectionSell
		default:
			return nil, errors.New("not found side type")
		}
		tradingEvent, err := svc.ProduceCreateOrderTradingEvent(ctx, userID, direction, decimal.NewFromFloat(req.Price), decimal.NewFromFloat(req.Size))
		if err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		return &createOrderResponse{
			ID: strconv.Itoa(tradingEvent.OrderRequestEvent.OrderID),
		}, nil // TODO: maybe need return id
	}
}

// response example:
//
//	{
//	    "items": [
//	        {
//	            "id": "b63d58c9-a3db-4b1a-850c-42e0822f9a71",
//	            "price": "20.00",
//	            "size": "2.000000",
//	            "funds": "40.00000000",
//	            "productId": "BTC-USDT",
//	            "side": "sell",
//	            "type": "limit",
//	            "createdAt": "2024-01-31T06:15:47.654Z",
//	            "fillFees": null,
//	            "filledSize": "1.250000",
//	            "executedValue": "0.00000000",
//	            "status": "open"
//	        }
//	    ],
//	    "count": 1
//	}
func MakeGetAccountOrdersEndpoint(orderUseCase domain.OrderUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(getAccountOrdersRequest) // TODO: pagination
		productID := currencyUseCase.GetProductID()
		if req.ProductID != productID {
			return nil, code.CreateErrorCode(http.StatusNotFound).AddErrorMetaData(errors.New("not found product id error"))
		}
		userOrders, err := orderUseCase.GetUserOrders(userID) // TODO: pagination
		if err != nil {
			return nil, errors.New("get user orders failed")
		}
		res := getAccountOrdersResponse{
			Count: len(userOrders),
		}
		for _, userOrder := range userOrders {
			// TODO: workaround .StringFixed(5)
			res.Items = append(res.Items, &accountOrder{
				ID:            strconv.Itoa(userOrder.ID),
				Price:         userOrder.Price.StringFixed(5),
				Size:          userOrder.Quantity.StringFixed(5),
				Funds:         userOrder.Quantity.Mul(userOrder.Price).StringFixed(5),
				ProductID:     productID,
				Side:          userOrder.Direction.String(),
				Type:          domain.OrderTypeLimit.String(), // TODO: implement another type
				FillFees:      "0",
				CreatedAt:     userOrder.CreatedAt,
				FilledSize:    userOrder.Quantity.Sub(userOrder.UnfilledQuantity).StringFixed(5),
				ExecutedValue: userOrder.Quantity.Sub(userOrder.UnfilledQuantity).Mul(userOrder.Price).StringFixed(5),
				Status:        "open", // TODO: correct?
			})
		}
		return res, nil
	}
}

// response example:
// [
//
//	{
//		"sequence": 2,
//		"time": "2024-01-31T06:15:47.110Z",
//		"price": "20.00",
//		"size": "1.2500",
//		"side": "sell"
//	},
//	{
//		"sequence": 1,
//		"time": "2024-01-31T06:13:23.960Z",
//		"price": "10.00",
//		"size": "1.000000",
//		"side": "buy"
//	}
//
// ]
func MakerGetHistoryOrdersEndpoint(tradingUseCase domain.TradingUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getHistoryOrdersRequest)

		if req.productID != currencyUseCase.GetProductID() {
			return nil, code.CreateErrorCode(http.StatusNotFound).AddErrorMetaData(errors.New("not found product error"))
		}

		details, err := tradingUseCase.GetHistoryMatchDetails(100) // TODO: get first 1000
		if err != nil {
			return nil, errors.Wrap(err, "get history orders failed")
		}
		res := make(getHistoryOrdersResponse, len(details))
		for idx, detail := range details {
			res[idx] = struct{ *historyOrder }{&historyOrder{
				Sequence: detail.SequenceID,
				Time:     detail.CreatedAt,
				Price:    detail.Price.String(),
				Size:     detail.Quantity.String(),
				Side:     detail.Direction.String(),
			},
			}
		}
		return res, nil
	}
}

func DecodeGetHistoryOrdersRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	productID, ok := vars["productID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	return getHistoryOrdersRequest{
		productID: productID,
	}, nil
}

func DecodeGetAccountOrdersRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	productID := r.URL.Query().Get("productId")
	if productID == "" {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("request format error"))
	}
	page := 1
	pageString := r.URL.Query().Get("page")
	if pageString != "" {
		pageFromQuery, err := strconv.Atoi(pageString)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("request format error"))
		}
		page = pageFromQuery
	}
	pageSize := 30
	pageSizeString := r.URL.Query().Get("pageSize")
	if pageSizeString != "" {
		pageSizeFromQuery, err := strconv.Atoi(pageSizeString)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("request format error"))
		}
		pageSize = pageSizeFromQuery
	}
	statuses := r.URL.Query()["status"]

	return getAccountOrdersRequest{
		ProductID: productID,
		Page:      page,
		PageSize:  pageSize,
		Status:    statuses,
	}, nil
}
