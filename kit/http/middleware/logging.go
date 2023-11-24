package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/kit/code"
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
					statusHTTPCode int
					errorStack     string
				)
				if err == nil { // success response
					successCode := code.ParseResponseSuccessCode(response)
					statusHTTPCode = successCode.HTTPCode
				} else { // error response
					errorCode := code.ParseErrorCode(err) // TODO: check time
					statusHTTPCode = errorCode.GeneralCode
					errorMsg = errorCode.Message
					errorStack = errorCode.CallStack
				}
				loggerWithMetadata := logger.With(
					loggerKit.Int("status", statusHTTPCode),
					loggerKit.String("error", errorMsg),
					loggerKit.String("trace-id", httpKit.GetTraceID(ctx)),
					loggerKit.String("method", "TODO"),
					loggerKit.String("path", url),
					loggerKit.String("query", "TODO"),
					loggerKit.String("ip", httpKit.GetIP(ctx)),
					loggerKit.String("user-agent", "TODO"),
					loggerKit.Time("time", time.Now()), //TODO: check is timestamp?
					loggerKit.Duration("latency", time.Since(begin)),
				)

				msg := url
				if statusHTTPCode == http.StatusInternalServerError {
					if loggerWithMetadata.Level == loggerKit.DebugLevel { // development: append error call stack to message(with new line for human read)
						msg += "\n" + errorStack
						loggerWithMetadata.With(
							loggerKit.String("error-call-stack", "already add to msg field for development"),
						).Error(msg)
					} else { // production: append error call stack to field
						loggerWithMetadata.With(
							loggerKit.String("error-call-stack", errorStack),
						).Error(msg)
					}

				} else {
					loggerWithMetadata.With(
						loggerKit.String("error-call-stack", ""),
					).Info(msg)
				}
			}(time.Now())

			res, err := e(ctx, request)

			return res, err
		}
	}
}
