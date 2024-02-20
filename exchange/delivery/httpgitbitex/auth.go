package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
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

type selfResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Band  bool   `json:"band"`
}

var (
	DecodeGetSelfRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetSelfResponse = httpTransportKit.EncodeJsonResponse

	DecodeAuthLoginRequest = httpTransportKit.DecodeJsonRequest[authLoginRequest]
)

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

func MakeSelfEndpoint(svc domain.AccountService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id")
		}
		account, err := svc.Get(userID)
		if err != nil {
			return nil, err
		}
		return &selfResponse{
			ID:    strconv.FormatInt(account.ID, 10),
			Email: account.Email,
			Band:  false, // TODO: what this?
		}, nil
	}
}
