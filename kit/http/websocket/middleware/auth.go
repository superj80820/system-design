package middleware

import (
	"context"

	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateAuth[IN, OUT any](verifyFn func(accessToken string) (int64, error)) endpoint.Middleware[IN, OUT] {
	return func(next endpoint.BiStream[IN, OUT]) endpoint.BiStream[IN, OUT] {
		return func(ctx context.Context, s endpoint.Stream[IN, OUT]) error {
			token := httpKit.GetToken(ctx)
			userID, err := verifyFn(token)
			if err != nil {
				return err // TODO
			}
			ctx = httpKit.AddUserID(ctx, int(userID)) // TODO: safe covert
			return next(ctx, s)
		}
	}
}
