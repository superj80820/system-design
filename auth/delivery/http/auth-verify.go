package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var EncodeAuthVerifyResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)

type authVerifyRequest struct {
	AccessToken string `json:"access_token"`
}

type authVerifyResponse struct {
	UserID int64 `json:"user_id"`
}

func MakeAuthVerifyEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(authVerifyRequest)
		userID, err := svc.Verify(req.AccessToken)
		if err != nil {
			fmt.Println("hihi")
			return nil, code.CreateErrorCode(http.StatusUnauthorized)
		}
		return &authVerifyResponse{
			UserID: userID,
		}, nil
	}
}

func DecodeAuthVerifyRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	token := r.Header.Get("Authentication")
	if token == "" {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get token failed"))
	}
	return authVerifyRequest{AccessToken: token}, nil
}
