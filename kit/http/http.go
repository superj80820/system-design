package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/superj80820/system-design/kit/code"
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
)

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

func CustomBeforeCtx(tracer trace.Tracer) func(ctx context.Context, r *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		ctx = context.WithValue(ctx, _CTX_TOKEN, r.Header.Get("Authentication"))
		ctx = context.WithValue(ctx, _CTX_HOST, r.Host)
		ctx = context.WithValue(ctx, _CTX_URL_PATH, r.URL.Path)
		ctx = context.WithValue(ctx, _CTX_IP_KEY, ReadUserIP(r)) // TODO: check correct

		ctx, span := tracer.Start(ctx, GetURL(ctx))
		defer span.End()

		ctx = context.WithValue(ctx, _CTX_TRACE_ID, span.SpanContext().TraceID().String())

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

func GetURL(ctx context.Context) string {
	return ctx.Value(_CTX_URL_PATH).(string)
}

func GetToken(ctx context.Context) string {
	return ctx.Value(_CTX_TOKEN).(string)
}

func EncodeHTTPErrorResponse() func(ctx context.Context, err error, w http.ResponseWriter) {
	return func(ctx context.Context, err error, w http.ResponseWriter) {
		if err == nil {
			panic("encodeError with nil error")
		}

		ctx = CustomAfterCtx(ctx, w)

		errorCode := code.CreateHTTPError(code.ParseErrorCode(err))

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(errorCode.HTTPCode)
		json.NewEncoder(w).Encode(errorCode)
	}
}
