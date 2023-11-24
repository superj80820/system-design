package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/kit/code"
	"github.com/superj80820/system-design/url-shorter/domain"
)

type urlShortenRequest struct {
	LongURL string `json:"longURL"`
}
type urlShortenResponse struct {
	ShortURL string `json:"shortURL"`
}

func MakeURLShortenEndpoint(svc domain.URLService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(urlShortenRequest)
		shortURL, err := svc.Save(ctx, req.LongURL)
		if err != nil {
			return nil, err
		}
		return &urlShortenResponse{ShortURL: shortURL}, nil // TODO: pointer
	}
}

func DecodeURLShortenRequest(ctx context.Context, r *http.Request) (interface{}, error) { // TODO: york can use interface for r
	var request urlShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddCode(code.InvalidBody).AddErrorMetaData(err)
	}
	return request, nil
}

func EncodeURLShortenResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
