package websocket

import (
	"context"

	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
)

func MakeChatEndpoint(svc domain.ChatService) endpoint.BiStream[domain.ChatRequest, domain.ChatResponse] { // TODO: need IN, OUT?
	return func(ctx context.Context, s endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) error {
		return svc.Chat(ctx, s)
	}
}

type ChannelRequest struct {
	Action         domain.ChatClientAction `json:"action"`
	SendChannelReq struct {
		ChannelName string `json:"channel_name"`
		Message     string `json:"message"`
	} `json:"send_channel_req,omitempty"`
	ChannelHistoryReq struct {
	} `json:"channel_history_req"`
	JoinChannelReq struct {
		ChannelName     string `json:"channel_name"`
		CurMaxMessageID int    `json:"cur_max_message_id"`
	} `json:"join_channel_req,omitempty"`
}

type ChannelResponse struct {
	Data      string `json:"data"`
	UserID    int    `json:"user_id"`
	MessageID int    `json:"message_id"`
}
