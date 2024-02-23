package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/code"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateGlobalRateLimitMiddleware(key string, passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) endpoint.Middleware {
	return createRateLimitMiddleware(func(ctx context.Context) string {
		return key
	}, passFunc)
}

func CreateRateLimitMiddlewareWithSpecKey(isIPUse, isMethodUse, isUserUse bool, passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) endpoint.Middleware {
	return createRateLimitMiddleware(func(ctx context.Context) string {
		var keySlice []string
		if isIPUse {
			keySlice = append(keySlice, httpKit.GetIP(ctx))
		}
		if isMethodUse {
			keySlice = append(keySlice, httpKit.GetURL(ctx))
		}
		if isUserUse {
			userID := httpKit.GetUserID(ctx)
			if userID != 0 {
				keySlice = append(keySlice, strconv.Itoa(httpKit.GetUserID(ctx)))
			}
		}
		return strings.Join(keySlice, "-")
	}, passFunc)
}

func createRateLimitMiddleware(getKey func(ctx context.Context) string, passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) endpoint.Middleware {
	return func(e endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			pass, _, expiry, err := passFunc(ctx, getKey(ctx))
			if err != nil {
				return nil, errors.Wrap(err, "get rate limit failed")
			}
			if !pass {
				return nil, code.CreateErrorCode(http.StatusTooManyRequests).AddCode(code.RateLimit, expiry)
			}
			return e(ctx, request)
		}
	}
}
