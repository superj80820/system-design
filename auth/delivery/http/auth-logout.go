package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeAuthLogoutRequest  = httpTransportKit.DecodeJsonRequest[authLogoutRequest]
	EncodeAuthLogoutResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
)

type authLogoutRequest struct {
	AccessToken string `json:"access_token"`
}

func MakeAuthLogoutEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(authLogoutRequest)
		if err := svc.Logout(req.AccessToken); err != nil {
			return nil, err
		}
		return nil, nil // TODO: empty response
	}
}
