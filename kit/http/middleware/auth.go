package middleware

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func CreateAuthMiddleware(authFunc func(ctx context.Context, token string) (userID int64, err error)) endpoint.Middleware {
	return func(e endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			token := httpKit.GetToken(ctx)
			fmt.Println("asdfasdf", token)
			userID, err := authFunc(ctx, token)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprint("auth failed"))
			}
			ctx = httpKit.AddUserID(ctx, int(userID)) // TODO: safe covert
			return e(ctx, request)
		}
	}
}
