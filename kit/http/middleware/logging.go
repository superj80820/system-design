package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

func CreateLoggingMiddleware(logger *loggerKit.Logger) endpoint.Middleware {
	return func(e endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			defer func(begin time.Time) { // TODO: check defer
				url := httpKit.GetURL(ctx)

				var (
					errorMsg       string
					errorCallStack string
					errorHTTPCode  int
				)
				if err != nil {
					errorCode := httpKit.DecodeErrorCode(err) // TODO: check time
					errorHTTPCode = errorCode.HTTPCode
					errorMsg = errorCode.Error()
					errorCallStack = fmt.Sprintf("%+v", err)
				}
				loggerWithMetadata := logger.With(
					loggerKit.Int("status", errorHTTPCode),
					loggerKit.String("error", errorMsg),
					loggerKit.String("error-call-stack", errorCallStack),
					loggerKit.String("method", "TODO"),
					loggerKit.String("path", url),
					loggerKit.String("query", "TODO"),
					loggerKit.String("ip", httpKit.GetIP(ctx)),
					loggerKit.String("user-agent", "TODO"),
					loggerKit.Time("time", time.Now()), //TODO: check is timestamp?
					loggerKit.Duration("latency", time.Since(begin)),
				)

				if errorHTTPCode == http.StatusInternalServerError {
					loggerWithMetadata.Error(url)
				} else {
					loggerWithMetadata.Info(url)
				}
			}(time.Now())

			res, err := e(ctx, request)

			return res, err
		}
	}
}
