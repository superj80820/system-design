package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type getAccountAssetsRequest struct {
	currencies []string
}

type accountAsset struct {
	ID           string `json:"id"`
	Currency     string `json:"currency"`
	CurrencyIcon string `json:"currencyIcon,omitempty"`
	Available    string `json:"available"`
	Hold         string `json:"hold"`
}

type getAccountAssetsResponse []struct {
	*accountAsset
}

var EncodeGetAccountAssetsResponse = httpTransportKit.EncodeJsonResponse

// response example:
// [
//
//	{
//	    "id": "92e2d434-7b41-404c-82cf-c43cfe03d41d-BTC",
//	    "currency": "BTC",
//	    "currencyIcon": null,
//	    "available": "999999999.250000",
//	    "hold": "0.750000"
//	},
//	{
//	    "id": "92e2d434-7b41-404c-82cf-c43cfe03d41d-USDT",
//	    "currency": "USDT",
//	    "currencyIcon": null,
//	    "available": "1000000000.00000000",
//	    "hold": "0.0"
//	}
//
// ]
func MakeGetAccountAssetsEndpoint(userAssetUseCase domain.UserAssetUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id") // TODO: delete
		}
		req := request.(getAccountAssetsRequest)
		userAssets, err := userAssetUseCase.GetAssets(userID)
		if errors.Is(err, domain.NotFoundUserAssetsErr) {
			return nil, code.CreateErrorCode(http.StatusNotFound)
		} else if err != nil {
			return nil, errors.New("get user assets failed")
		}
		res := make(getAccountAssetsResponse, 0, len(userAssets))
		for _, currencyName := range req.currencies {
			assetID, err := currencyUseCase.GetCurrencyTypeByName(currencyName)
			if err != nil && errors.Is(err, domain.ErrNoData) {
				continue
			}
			asset, ok := userAssets[int(assetID)]
			if !ok {
				continue
			}
			res = append(res, struct{ *accountAsset }{&accountAsset{ // TODO: maybe just return slice?
				ID:        strconv.Itoa(int(assetID)),
				Currency:  currencyName,
				Available: asset.Available.String(),
				Hold:      asset.Frozen.String(),
			}})
		}
		return res, nil
	}
}

func DecodeGetAccountAssetsRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	currency := r.URL.Query()["currency"]
	if len(currency) == 0 {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("not get currency error"))
	}
	return getAccountAssetsRequest{currencies: currency}, nil
}
