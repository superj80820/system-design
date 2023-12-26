package middleware

import (
	"context"
	"net/http"

	"github.com/superj80820/system-design/kit/code"
)

func EncodeResponseSetSuccessHTTPCode(next func(ctx context.Context, w http.ResponseWriter, response interface{}) error) func(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return func(ctx context.Context, w http.ResponseWriter, response interface{}) error {
		defer func() {
			successCode := code.ParseResponseSuccessCode(response)
			if successCode.HTTPCode == http.StatusOK {
				return
			}
			w.WriteHeader(successCode.HTTPCode)
		}()
		return next(ctx, w, response)
	}
}
