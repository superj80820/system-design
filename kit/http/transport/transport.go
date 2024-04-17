package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/superj80820/system-design/kit/code"
)

func DecodeEmptyRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	return nil, nil
}

func DecodeJsonRequest[T any](ctx context.Context, r *http.Request) (interface{}, error) {
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(err)
	}
	return req, nil
}

func EncodeJsonResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func EncodeEmptyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error { // TODO: json ok
	return nil
}

func EncodeOKResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error { // TODO: json ok
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("200 OK"))
	return nil
}
