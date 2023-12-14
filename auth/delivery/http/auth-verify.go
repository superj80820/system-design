package http

import (
	"github.com/superj80820/system-design/auth/delivery"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

var (
	DecodeAuthVerifyRequest  = httpTransportKit.DecodeJsonRequest[delivery.AuthVerifyRequest]
	EncodeAuthVerifyResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)
)
