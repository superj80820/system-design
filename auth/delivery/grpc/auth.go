package grpc

import (
	grpctransport "github.com/go-kit/kit/transport/grpc"
	"github.com/superj80820/system-design/auth/delivery"
	"github.com/superj80820/system-design/domain"
)

type AuthServer struct {
	domain.UnimplementedAuthServer
	verify grpctransport.Handler
}

func CreateAuthServer(svc domain.AuthUseCase) domain.AuthServer {
	return &AuthServer{
		verify: grpctransport.NewServer(
			delivery.MakeAuthVerifyEndpoint(svc),
			decodeGRPCAuthVerifyRequest,
			encodeGRPCAuthVerifyResponse,
		),
	}
}
