package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeAuthLoginRequest  = httpTransportKit.DecodeJsonRequest[AuthLoginRequest]
	EncodeAuthLoginResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)
)

type AuthLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func MakeAuthLoginEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(AuthLoginRequest)
		account, err := svc.Login(req.Email, req.Password)
		if err != nil {
			return nil, err
		}
		return &AuthLoginResponse{AccessToken: account.AccessToken, RefreshToken: account.RefreshToken}, nil
	}
}
