package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	loggerKit "github.com/superj80820/system-design/kit/logger"

	httpKit "github.com/superj80820/system-design/kit/http"
)

func EncodeError(logger loggerKit.Logger) func(ctx context.Context, err error, w http.ResponseWriter) {
	return func(ctx context.Context, err error, w http.ResponseWriter) {
		if err == nil {
			panic("encodeError with nil error")
		}
		errorCode := httpKit.DecodeErrorCode(err)
		if errorCode.HTTPCode == http.StatusInternalServerError {
			logger.Log("err", "internal error. "+err.Error(), "call stack", fmt.Sprintf("%+v", err))
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(errorCode.HTTPCode)
		json.NewEncoder(w).Encode(errorCode)
	}
}
