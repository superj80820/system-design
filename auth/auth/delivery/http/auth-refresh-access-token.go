package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/auth/domain"
	"github.com/superj80820/system-design/kit/code"
)

type refreshAccessTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func MakeRefreshAccessTokenEndpoint(svc domain.AuthService) endpoint.Endpoint {
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

func DecodeRefreshAccessTokenRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req refreshAccessTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(err)
	}
	return req, nil
}

func EncodeRefreshAccessTokenResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
