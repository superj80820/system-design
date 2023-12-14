package grpc

import (
	"context"

	"github.com/superj80820/system-design/auth/delivery"
	"github.com/superj80820/system-design/domain"
)

func decodeGRPCAuthVerifyRequest(ctx context.Context, grpcReq interface{}) (request interface{}, err error) {
	req := grpcReq.(*domain.VerifyRequest)
	return &delivery.AuthVerifyRequest{AccessToken: req.AccessToken}, nil
}

func encodeGRPCAuthVerifyResponse(ctx context.Context, grpcReply interface{}) (response interface{}, err error) {
	reply := grpcReply.(*domain.VerifyReply)
	return &delivery.AuthVerifyResponse{UserID: reply.UserID}, nil
}

func (a *AuthServer) Verify(ctx context.Context, req *domain.VerifyRequest) (*domain.VerifyReply, error) {
	_, res, err := a.verify.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.(*domain.VerifyReply), nil

}
