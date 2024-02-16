package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeAccountRegisterRequest  = httpTransportKit.DecodeJsonRequest[accountRegisterRequest]
	EncodeAccountRegisterResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
)

type accountRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type accountRegisterResponse struct {
	ID int64

	AccessToken  string
	RefreshToken string
}

func MakeAccountRegisterEndpoint(svc domain.AccountService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(accountRegisterRequest)
		account, err := svc.Register(req.Email, req.Password)
		if err != nil {
			return nil, err
		}
		return &accountRegisterResponse{
			ID:           account.ID,
			AccessToken:  account.AccessToken,
			RefreshToken: account.RefreshToken,
		}, nil
	}
}
