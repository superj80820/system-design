package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/code"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateRateLimit[IN, OUT any](passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) endpoint.Middleware[IN, OUT] {
	return func(next endpoint.BiStream[IN, OUT]) endpoint.BiStream[IN, OUT] {
		return func(ctx context.Context, s endpoint.Stream[IN, OUT]) error {
			check := func() error {
				pass, _, expiry, err := passFunc(ctx, httpKit.GetIP(ctx))
				if err != nil {
					return errors.Wrap(err, fmt.Sprint("get rate limit failed"))
				}
				if !pass {
					return code.CreateErrorCode(http.StatusTooManyRequests).AddCode(code.RateLimit, expiry)
				}
				return nil
			}

			if err := check(); err != nil {
				return err
			}

			return next(ctx, s.AddInMiddleware(func(in *IN) (*IN, error) { // TODO: think why can't cover
				if err := check(); err != nil {
					fmt.Println("arte11")
					return nil, err
				}
				fmt.Println("arte")
				return in, nil
			}))
		}
	}
}
