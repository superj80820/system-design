package delivery

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type verifyLIFFRequest struct {
	AccessToken string `json:"access_token"`
	LIFFID      string `json:"liff_id"`
}

var (
	DecodeVerifyLIFFRequest  = httpTransportKit.DecodeJsonRequest[verifyLIFFRequest]
	EncodeVerifyLIFFResponse = httpTransportKit.EncodeEmptyResponse
)

func MakeVerfiyLIFFEndpoint(lineLIFFUseCase domain.LineLIFFUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(verifyLIFFRequest)

		if err := lineLIFFUseCase.VerifyLIFF(req.AccessToken, req.LIFFID); err != nil {
			return nil, errors.Wrap(err, "verify liff failed")
		}

		return nil, nil
	}
}
