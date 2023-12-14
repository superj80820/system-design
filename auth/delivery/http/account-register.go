package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeAccountRegisterRequest  = httpTransportKit.DecodeJsonRequest[accountRegisterRequest]
	EncodeAccountRegisterResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
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
