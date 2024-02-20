package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type getProductsResponse []struct {
	*domain.CurrencyProduct
}

var (
	DecodeGetProductsRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetProductsResponse = httpTransportKit.EncodeJsonResponse
)

// response example:
//
//	[{
//	    "id": "BTC-USDT",
//	    "baseCurrency": "BTC",
//	    "quoteCurrency": "USDT",
//	    "baseMinSize": null,
//	    "baseMaxSize": null,
//	    "quoteIncrement": "0.0",
//	    "baseScale": 6,
//	    "quoteScale": 2
//	}]
func MakeGetProductsEndpoint(currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		res := &getProductsResponse{
			struct{ *domain.CurrencyProduct }{currencyUseCase.GetProduct()}, // TODO: maybe just return slice?
		}
		return res, nil
	}
}
