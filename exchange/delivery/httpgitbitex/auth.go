package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

var DecodeAuthLoginRequest = httpTransportKit.DecodeJsonRequest[authLoginRequest]

func EncodeAuthResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	res := response.(*authLoginResponse)
	http.SetCookie(w, &http.Cookie{
		Name:    "accessToken",
		Value:   res.AccessToken,
		MaxAge:  604800,
		Expires: time.Now().Add(604800 * time.Second),
		Path:    "/",
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func MakeAuthLoginEndpoint(svc domain.AuthUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(authLoginRequest)
		account, err := svc.Login(req.Email, req.Password)
		if err != nil {
			return nil, err
		}
		return &authLoginResponse{AccessToken: account.AccessToken, RefreshToken: account.RefreshToken}, nil
	}
}
