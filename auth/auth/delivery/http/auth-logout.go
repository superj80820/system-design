package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/auth/domain"
	"github.com/superj80820/system-design/kit/code"
)

type authLogoutRequest struct {
	AccessToken string `json:"access_token"`
}

func MakeAuthLogoutEndpoint(svc domain.AuthService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(authLogoutRequest)
		if err := svc.Logout(req.AccessToken); err != nil {
			return nil, err
		}
		return nil, nil // TODO: empty response
	}
}

func DecodeAuthLogoutRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req authLogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(err)
	}
	return req, nil
}

func EncodeAuthLogoutResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return nil
}
