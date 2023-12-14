package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeRefreshAccessTokenRequest  = httpTransportKit.DecodeJsonRequest[refreshAccessTokenRequest]
	EncodeRefreshAccessTokenResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)
)

type refreshAccessTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func MakeRefreshAccessTokenEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(refreshAccessTokenRequest)
		accessToken, err := svc.RefreshAccessToken(req.RefreshToken)
		if err != nil {
			return nil, err
		}
		return &refreshAccessTokenResponse{
			AccessToken: accessToken,
		}, nil
	}
}
