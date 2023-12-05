package domain

import (
	"context"
	"time"

	"github.com/superj80820/system-design/kit/core/endpoint"
)

type Channel struct {
	ChannelID int `bson:"channel_id" json:"channel_id"`
}

type Account struct {
	ID int
}

type ChatRepository interface {
	GetHistoryMessage(ctx context.Context, accountID, offset, page int) ([]*FriendOrChannelMessage, bool, error)
	GetHistoryMessageByChannel(ctx context.Context, channelID, offset, page int) ([]*ChannelMessage, bool, error)
	GetHistoryMessageByFriend(ctx context.Context, accountID, friendID, offset, page int) ([]*FriendMessage, bool, error)
	InsertFriendMessage(ctx context.Context, friendMessage *FriendMessage) error
	InsertChannelMessage(ctx context.Context, channelMessage *ChannelMessage) error
	GetChannelByName(channelName string) (*Channel, error)

	Online(ctx context.Context, id int)
	Offline(ctx context.Context, id int)
	GetAccountChannels(ctx context.Context, id int) ([]*Channel, error)
	GetAccountFriends(ctx context.Context, id int) ([]*Account, error)
}

type ChatClientAction string

const (
	SendMessageToChannel     ChatClientAction = "send_message_to_channel"
	SendMessageToFriend      ChatClientAction = "send_message_to_friend"
	GetFriendHistoryMessage  ChatClientAction = "get_friend_history_message"
	GetChannelHistoryMessage ChatClientAction = "get_channel_history_message"
	GetHistoryMessage        ChatClientAction = "get_history_message"
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
	GetFriendHistoryMessage struct { // TODO
		FriendID        int `json:"friend_id"`
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	}
	GetChannelHistoryMessage struct { // TODO
		ChannelID       int `json:"channel_id"`
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	}
	GetHistoryMessage struct { // TODO
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	}
}

type ChatResponse struct {
	Data      string `json:"data"`
	UserID    int    `json:"user_id"`
	MessageID int    `json:"message_id"`
}

type MessageType int

const (
	FriendMessageType MessageType = iota + 1
	ChannelMessageType
)

type FriendOrChannelMessage struct {
	MessageType    MessageType `bson:"message_type" json:"message_type"` // TODO: name repeat?
	FriendMessage  *FriendMessage
	ChannelMessage *ChannelMessage
}

type MessageMetadata struct {
	MessageType MessageType `bson:"message_type" json:"message_type"`
	MetadataID  int64       `bson:"metadata_id" json:"metadata_id"`
	MessageID   int64       `bson:"message_id" json:"message_id"`
	UserID      int64       `bson:"user_id" json:"user_id"`
	CreatedAt   time.Time   // TODO
	UpdatedAt   time.Time   //TODO
}

type ChannelMessage struct {
	MessageID int64  `bson:"message_id" json:"message_id"`
	ChannelID int64  `bson:"channel_id" json:"channel_id"`
	Content   string `bson:"content" json:"content"`
	UserID    int64  `bson:"user_id" json:"user_id"`
}

type FriendMessage struct {
	MessageID int64  `bson:"message_id" json:"message_id"`
	Content   string `bson:"content" json:"content"`
	FriendID  int64  `bson:"friend_id" json:"friend_id"`
	UserID    int64  `bson:"user_id" json:"user_id"`
}

type ChatService interface {
	Chat(context.Context, endpoint.Stream[ChatRequest, ChatResponse]) error
}

type AccountChatInformation struct{}
