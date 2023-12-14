package middleware

import (
	"context"
	"fmt"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateAuth[IN, OUT any](authServiceRepo domain.AuthServiceRepository) endpoint.Middleware[IN, OUT] {
	return func(next endpoint.BiStream[IN, OUT]) endpoint.BiStream[IN, OUT] {
		return func(ctx context.Context, s endpoint.Stream[IN, OUT]) error {
			token := httpKit.GetToken(ctx)
			userID, err := authServiceRepo.Verify(token)
			if err != nil {
				return err // TODO
			}
			fmt.Println("ajajajjja", userID, err)
			ctx = httpKit.AddUserID(ctx, int(userID)) // TODO: safe covert
			return next(ctx, s)
		}
	}
}
