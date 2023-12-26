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

			// TODO: important think why can't cover channel
			return next(ctx, &rateLimit[IN, OUT]{Stream: s, check: check})
		}
	}
}

type rateLimit[IN, OUT any] struct {
	endpoint.Stream[IN, OUT]

	check func() error
}

func (a *rateLimit[IN, OUT]) RecvFromIn() (IN, error) {
	if err := a.check(); err != nil {
		var noop IN
		return noop, err
	}
	return a.Stream.Recv()
}
