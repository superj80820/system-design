package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/superj80820/system-design/kit/code"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.opentelemetry.io/otel/trace"
)

type ctxKeyType int

const ( // TODO check correct
	_CTX_IP_KEY ctxKeyType = iota
	_CTX_HOST
	_CTX_URL_PATH
	_CTX_TRACE_ID
	_CTX_HTTP_CODE
	_CTX_TOKEN
	_CTX_REQUEST_ID
	_CTX_USER_ID
)

type customBeforeCtxOption struct {
	cookieAccessTokenKey string
}

func createCustomBeforeCtxOption() *customBeforeCtxOption {
	return &customBeforeCtxOption{
		cookieAccessTokenKey: "access_token",
	}
}

type Option func(*customBeforeCtxOption)

func OptionSetCookieAccessTokenKey(key string) Option {
	return func(cbco *customBeforeCtxOption) {
		cbco.cookieAccessTokenKey = key
	}
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return strings.Split(IPAddress, ":")[0]
}

func CustomBeforeCtx(tracer trace.Tracer, options ...Option) func(ctx context.Context, r *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		option := createCustomBeforeCtxOption()
		for _, applyOption := range options {
			applyOption(option)
		}
		var accessToken string
		for _, cookie := range r.Cookies() {
			if cookie.Name == option.cookieAccessTokenKey {
				accessToken = cookie.Value
			}
		}
		authentication := r.Header.Get("Authentication")
		if accessToken == "" && strings.Index(authentication, "Bearer") == 0 {
			accessToken = authentication[len("Bearer "):]
		}
		ctx = context.WithValue(ctx, _CTX_TOKEN, accessToken)    // TODO: add
		ctx = context.WithValue(ctx, _CTX_HOST, r.Host)          // TODO: add
		ctx = context.WithValue(ctx, _CTX_URL_PATH, r.URL.Path)  // TODO: add
		ctx = context.WithValue(ctx, _CTX_IP_KEY, ReadUserIP(r)) // TODO: check correct // TODO: add
		ctx = AddRequestID(ctx)

		ctx, span := tracer.Start(ctx, GetURL(ctx))
		defer span.End()

		ctx = AddTraceID(ctx, span.SpanContext().TraceID().String())

		return ctx
	}
}

func CustomAfterCtx(ctx context.Context, w http.ResponseWriter) context.Context {
	w.Header().Add("X-B3-TraceId", trace.SpanContextFromContext(ctx).TraceID().String())
	return ctx
}

func GetTraceID(ctx context.Context) string {
	return ctx.Value(_CTX_TRACE_ID).(string)
}

func GetIP(ctx context.Context) string {
	return ctx.Value(_CTX_IP_KEY).(string)
}

func AddTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, _CTX_TRACE_ID, traceID)
}

func GetURL(ctx context.Context) string {
	return ctx.Value(_CTX_URL_PATH).(string)
}

func AddUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, _CTX_USER_ID, userID)
}

func GetUserID(ctx context.Context) int {
	return ctx.Value(_CTX_USER_ID).(int)
}

func AddToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, _CTX_TOKEN, token)
}

func GetToken(ctx context.Context) string {
	return ctx.Value(_CTX_TOKEN).(string)
}

func AddRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, _CTX_REQUEST_ID, utilKit.GetSnowflakeIDInt64())
}

func GetRequestID(ctx context.Context) int64 {
	return ctx.Value(_CTX_REQUEST_ID).(int64)
}

func EncodeHTTPErrorResponse() func(ctx context.Context, err error, w http.ResponseWriter) {
	return func(ctx context.Context, err error, w http.ResponseWriter) {
		if err == nil {
			panic("encodeError with nil error")
		}

		errorCode := code.CreateHTTPError(code.ParseErrorCode(err))

		fmt.Println("yorkkk", err)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(errorCode.HTTPCode)
		json.NewEncoder(w).Encode(errorCode)
	}
}
