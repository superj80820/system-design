package middleware

import (
	"context"
	"net/http"

	"github.com/superj80820/system-design/kit/code"
)

func EncodeResponseSetSuccessHTTPCode(next func(ctx context.Context, w http.ResponseWriter, response interface{}) error) func(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return func(ctx context.Context, w http.ResponseWriter, response interface{}) error {
		defer func() {
			w.WriteHeader(code.ParseResponseSuccessCode(response).HTTPCode)
		}()
		return next(ctx, w, response)
	}
}
