package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/code"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateRateLimitMiddleware(passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) endpoint.Middleware {
	return func(e endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			pass, _, expiry, err := passFunc(ctx, httpKit.GetIP(ctx))
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprint("get rate limit failed"))
			}
			if !pass {
				return nil, code.CreateErrorCode(http.StatusTooManyRequests).AddCode(code.RateLimit, expiry)
			}
			return e(ctx, request)
		}
	}
}
