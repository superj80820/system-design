package delivery

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type verifyLIFFRequest struct {
	LIFFToken string `json:"liff_token"`
}

type verifyLineCodeRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
}

type verifyLineCodeResponse struct {
	*domain.Account
	LineUserProfile *domain.LineUserProfile `json:"line_user_profile"`
}

var (
	DecodeVerifyLIFFRequest  = httpTransportKit.DecodeJsonRequest[verifyLIFFRequest]
	EncodeVerifyLIFFResponse = httpTransportKit.EncodeJsonResponse

	DecodeVerifyLineCodeRequest  = httpTransportKit.DecodeJsonRequest[verifyLineCodeRequest]
	EncodeVerifyLineCodeResponse = httpTransportKit.EncodeJsonResponse
)

func MakeVerifyLIFFEndpoint(authLineUseCase domain.AuthLineUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(verifyLIFFRequest)

		userInformation, lineProfile, err := authLineUseCase.VerifyLIFFToken(req.LIFFToken)
		if err != nil {
			return nil, errors.Wrap(err, "verify code failed")
		}

		return verifyLineCodeResponse{
			userInformation,
			lineProfile,
		}, nil
	}
}

func MakeVerifyLineCodeEndpoint(authLineUseCase domain.AuthLineUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(verifyLineCodeRequest)

		userInformation, lineProfile, err := authLineUseCase.VerifyCode(req.Code, req.RedirectURI)
		if err != nil {
			return nil, errors.Wrap(err, "verify code failed")
		}

		return verifyLineCodeResponse{
			userInformation,
			lineProfile,
		}, nil
	}
}
