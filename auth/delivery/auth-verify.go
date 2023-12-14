package delivery

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
)

type AuthVerifyRequest struct {
	AccessToken string `json:"access_token"`
}

type AuthVerifyResponse struct {
	UserID int64 `json:"user_id"`
}

func MakeAuthVerifyEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(AuthVerifyRequest)
		userID, err := svc.Verify(req.AccessToken)
		if err != nil {
			return nil, err
		}
		return &AuthVerifyResponse{
			UserID: userID,
		}, nil
	}
}
