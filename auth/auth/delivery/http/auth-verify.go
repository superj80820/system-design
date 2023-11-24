package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/auth/domain"
	"github.com/superj80820/system-design/kit/code"
)

type authVerifyRequest struct {
	AccessToken string `json:"access_token"`
}

func MakeAuthVerifyEndpoint(svc domain.AuthService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(authVerifyRequest)
		if err := svc.Verify(req.AccessToken); err != nil {
			return nil, err
		}
		return nil, nil // TODO: empty response
	}
}

func DecodeAuthVerifyRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req authVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(err)
	}
	return req, nil
}

func EncodeAuthVerifyResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return nil
}
