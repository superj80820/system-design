package http

import (
	"context"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/url-shorter/domain"
)

type urlGetRequest struct {
	ShortURL string `json:"shortURL"`
}
type urlGetResponse struct {
	LongURL string `json:"longURL"`
}

func MakeURLGetEndpoint(svc domain.URLService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(urlGetRequest)
		longURL, err := svc.Get(ctx, req.ShortURL)
		if err != nil {
			return nil, err
		}
		return urlGetResponse{LongURL: longURL}, nil
	}
}

func DecodeURLGetRequests(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	shortURL, ok := vars["shortURL"]
	if !ok {
		return nil, errors.New("get shortURL failed")
	}
	return urlGetRequest{ShortURL: shortURL}, nil
}

func EncodeURLGetResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	res := response.(urlGetResponse)
	w.Header().Set("Location", res.LongURL)
	w.WriteHeader(http.StatusMovedPermanently)
	return nil
}
