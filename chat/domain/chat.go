package domain

import (
	"context"

	"github.com/superj80820/system-design/kit/core/endpoint"
)

type ChatClientAction string

const (
	SendChannel    ChatClientAction = "send_channel"
	SendUser       ChatClientAction = "send_user"
	ChannelHistory ChatClientAction = "channel_history"
	JoinChannel    ChatClientAction = "join_channel"
)

type ChatRequest struct {
	Action         ChatClientAction `json:"action"`
	SendChannelReq struct {
		ChannelName string `json:"channel_name"`
		Message     string `json:"message"`
	} `json:"send_channel_req,omitempty"`
	ChannelHistoryReq struct {
	} `json:"channel_history_req"`
	JoinChannelReq struct {
		ChannelName     string `json:"channel_name"`
		CurMaxMessageID int    `json:"cur_max_message_id,omitempty"`
	} `json:"join_channel_req,omitempty"`
}

type ChatResponse struct {
	Data      string `json:"data"`
	UserID    int    `json:"user_id"`
	MessageID int    `json:"message_id"`
}

type ChatService interface {
	Chat(context.Context, endpoint.Stream[ChatRequest, ChatResponse]) error
}

type ChannelMessage struct {
	MessageID int    `bson:"message_id" json:"message_id"`
	ChannelID int    `bson:"channel_id" json:"channel_id"`
	Content   string `bson:"content" json:"content"`
	UserID    int    `bson:"user_id" json:"user_id"`
}
