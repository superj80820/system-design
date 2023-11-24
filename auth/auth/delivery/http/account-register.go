package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/auth/domain"
	"github.com/superj80820/system-design/kit/code"
)

type accountRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func MakeAccountRegisterEndpoint(svc domain.AccountService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(accountRegisterRequest)
		if err := svc.Register(req.Email, req.Password); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

func DecodeAccountRegisterRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req accountRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(err)
	}
	return req, nil
}

func EncodeAccountRegisterResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return nil
}
