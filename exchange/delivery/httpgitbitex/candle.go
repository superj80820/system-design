package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type getCandlesRequest struct {
	granularity int
	productID   string
}

type getCandlesResponse [][]float64 // TODO: use float64 save?

var EncodeGetCandlesResponse = httpTransportKit.EncodeJsonResponse

// response example:
// [
//
//	1706865300,
//	1.23,
//	1.23,
//	20,
//	1.23,
//	1.41
//
// ]
func MakeGetCandleEndpoint(candleUseCase domain.CandleUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getCandlesRequest)

		if req.productID != currencyUseCase.GetProductID() {
			return nil, code.CreateErrorCode(http.StatusNotFound).AddErrorMetaData(errors.New("not found product error"))
		}

		end := time.Now().UnixMilli()
		start := end - 60*60*24*10*1000
		endString := strconv.FormatInt(end, 10)
		startString := strconv.FormatInt(start, 10)

		bar, err := func() ([]string, error) {
			if req.granularity/60 == 1 {
				bar, err := candleUseCase.GetBar(ctx, domain.CandleTimeTypeMin, startString, endString, domain.DESCSortOrderByEnum)
				if err != nil {
					return nil, errors.Wrap(err, "get bar failed")
				}
				return bar, nil
			} else if req.granularity/60/60 == 1 {
				bar, err := candleUseCase.GetBar(ctx, domain.CandleTimeTypeHour, startString, endString, domain.DESCSortOrderByEnum)
				if err != nil {
					return nil, errors.Wrap(err, "get bar failed")
				}
				return bar, nil
			} else if req.granularity/60/60/24 == 1 {
				bar, err := candleUseCase.GetBar(ctx, domain.CandleTimeTypeDay, startString, endString, domain.DESCSortOrderByEnum)
				if err != nil {
					return nil, errors.Wrap(err, "get bar failed")
				}
				return bar, nil
			}
			return nil, code.CreateErrorCode(http.StatusNotFound).AddErrorMetaData(errors.New("not found candle granularity error"))
		}()
		if err != nil {
			return nil, errors.Wrap(err, "get bar failed")
		}

		res := make(getCandlesResponse, len(bar))
		for idx, valString := range bar {
			var varSlice []float64
			err := json.Unmarshal([]byte(valString), &varSlice)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal filed")
			}
			varSlice[0] = float64(int(varSlice[0]) / 1000) // TODO: safe?
			res[idx] = varSlice
		}

		return res, nil
	}
}

func DecodeGetCandlesRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	productID, ok := vars["productID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get order id failed"))
	}
	granularityString := r.URL.Query().Get("granularity")
	if granularityString == "" {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("request format error"))
	}
	granularity, err := strconv.Atoi(granularityString)
	if err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("request format error"))
	}
	return getCandlesRequest{
		granularity: granularity,
		productID:   productID,
	}, nil
}
