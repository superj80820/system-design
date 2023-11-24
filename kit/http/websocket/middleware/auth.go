package middleware

import (
	"context"

	"github.com/superj80820/system-design/kit/core/endpoint"
)

func CreateAuth[IN, OUT any]() endpoint.Middleware[IN, OUT] {
	return func(next endpoint.BiStream[IN, OUT]) endpoint.BiStream[IN, OUT] {
		return func(ctx context.Context, s endpoint.Stream[IN, OUT]) error {
			// TODO: auth
			return next(ctx, s)
		}
	}
}
