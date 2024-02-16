package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var DecodeAuthLoginRequest = httpTransportKit.DecodeJsonRequest[AuthLoginRequest]

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

func EncodeAuthLoginResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	res := response.(*AuthLoginResponse)
	http.SetCookie(w, &http.Cookie{
		Name:  "access_token",
		Value: res.AccessToken,
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
