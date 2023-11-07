package http

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// const ( // TODO check correct
// 	_CTX_IP_KEY   = "ip"
// 	_CTX_HOST     = "host"
// 	_CTX_URL_PATH = "url_path"
// 	_CTX_TRACE_ID = "trace_id"
// )

type ctxKeyType int

const ( // TODO check correct
	_CTX_IP_KEY ctxKeyType = iota
	_CTX_HOST
	_CTX_URL_PATH
	_CTX_TRACE_ID
)

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

func CustomBeforeCtx(tracer trace.Tracer) func(ctx context.Context, r *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
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
