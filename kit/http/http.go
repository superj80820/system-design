package http

import (
	"context"
	"net/http"
)

const CTX_IP_KEY = "ip"

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

func CustomCtx(ctx context.Context, r *http.Request) context.Context {
	ctx = context.WithValue(ctx, CTX_IP_KEY, ReadUserIP(r)) // TODO: check correct
	return ctx
}

func GetIP(ctx context.Context) string {
	return ctx.Value(CTX_IP_KEY).(string)
}
