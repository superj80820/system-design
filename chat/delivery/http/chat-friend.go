package http

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type ChatAddFriendRequest struct {
	FriendID int `json:"friend_id"`
}

func MakeChatAddFriendEndpoint(svc domain.ChatService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(ChatAddFriendRequest)
		userID := httpKit.GetUserID(ctx)
		err = svc.AddFriend(ctx, userID, req.FriendID)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
}

var (
	DecodeAddFriendRequest  = httpTransportKit.DecodeJsonRequest[ChatAddFriendRequest]
	EncodeAddFriendResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
)
