package domain

import (
	"context"
	"time"

	"github.com/superj80820/system-design/kit/core/endpoint"
)

type ChatRepository interface {
	GetOrCreateUserChatInformation(ctx context.Context, userID int) (*AccountChatInformation, error)
	UpdateOnlineStatus(ctx context.Context, userID int, onlineStatus OnlineStatusEnum) error

	CreateChannel(ctx context.Context, userID int, channelName string) (int64, error)

	CreateAccountChannels(ctx context.Context, userID, channelID int) error
	CreateAccountFriends(ctx context.Context, userID, friendID int) error
	GetAccountChannels(ctx context.Context, userID int) ([]int64, error)
	GetAccountFriends(ctx context.Context, userID int) ([]int64, error)

	GetHistoryMessage(ctx context.Context, accountID, offset, page int) ([]*FriendOrChannelMessage, bool, error)
	GetHistoryMessageByChannel(ctx context.Context, channelID, offset, page int) ([]*ChannelMessage, bool, error)
	GetHistoryMessageByFriend(ctx context.Context, accountID, friendID, offset, page int) ([]*FriendMessage, bool, error)

	InsertFriendMessage(ctx context.Context, userID, friendID int64, content string) (int64, error)
	InsertChannelMessage(ctx context.Context, userID, channelID int64, content string) (int64, error)

	SendFriendMessage(ctx context.Context, userID, friendID, messageID int, content string) error
	SendChannelMessage(ctx context.Context, userID, channelID, messageID int, content string) error
	SendUserOnlineStatusMessage(ctx context.Context, userID int, onlineStatus StatusType) error
	SendJoinChannelStatusMessage(ctx context.Context, userID int, channelID int) error
	SendAddFriendStatusMessage(ctx context.Context, userID int, friendID int) error

	SubscribeUserStatus(ctx context.Context, userID int, notify func(*StatusMessage) error)
	SubscribeFriendOnlineStatus(ctx context.Context, friend int, notify func(*StatusMessage) error)
	SubscribeFriendMessage(ctx context.Context, userID int, notify func(*FriendMessage) error)
	SubscribeChannelMessage(ctx context.Context, channelID int, notify func(*ChannelMessage) error)

	UnSubscribeFriendOnlineStatus(ctx context.Context, friendID int)
	UnSubscribeFriendMessage(ctx context.Context, userID int)
	UnSubscribeChannelMessage(ctx context.Context, channelID int)

	UnSubscribeAll(ctx context.Context)
}

type Channel struct {
	ChannelID        int64 `bson:"channel_id" json:"channel_id" gorm:"column:id"` // TODO: check gorm label
	Name             string
	CreatorAccountID int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AccountChannel struct {
	ID        int64
	AccountID int64
	ChannelID int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AccountFriend struct {
	ID         int64
	Account1ID int64 `gorm:"column:account_1_id"`
	Account2ID int64 `gorm:"column:account_2_id"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MessageType int

const (
	FriendMessageType MessageType = iota + 1
	ChannelMessageType
)

type FriendOrChannelMessage struct {
	MessageType    MessageType     `bson:"message_type" json:"message_type"` // TODO: name repeat?
	FriendMessage  *FriendMessage  `json:"friend_message,omitempty"`
	ChannelMessage *ChannelMessage `json:"channel_message,omitempty"`
}

type MessageMetadata struct {
	MessageType MessageType `bson:"message_type" json:"message_type"`
	MetadataID  int64       `bson:"metadata_id" json:"metadata_id"`
	MessageID   int64       `bson:"message_id" json:"message_id"`
	UserID      int64       `bson:"user_id" json:"user_id"`
	CreatedAt   time.Time   `bson:"created_at"`
	UpdatedAt   time.Time   `bson:"updated_at"`
}

type ChannelMessage struct {
	MessageID int64     `bson:"message_id" json:"message_id"`
	ChannelID int64     `bson:"channel_id" json:"channel_id"`
	Content   string    `bson:"content" json:"content"`
	UserID    int64     `bson:"user_id" json:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type FriendMessage struct {
	MessageID int64     `bson:"message_id" json:"message_id"`
	Content   string    `bson:"content" json:"content"`
	FriendID  int64     `bson:"friend_id" json:"friend_id"`
	UserID    int64     `bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type ChatService interface {
	Chat(ctx context.Context, userID int, stream endpoint.Stream[ChatRequest, ChatResponse]) error

	AddFriend(ctx context.Context, userID, friendID int) error
	CreateChannel(ctx context.Context, userID int, channelName string) (int64, error)
	JoinChannel(ctx context.Context, userID, channelID int) error
}

type OnlineStatusEnum int

const (
	OfflineStatus OnlineStatusEnum = iota
	OnlineStatus
)

type AccountChatInformation struct {
	ID        int64            `json:"id"`
	AccountID int64            `json:"account_id"`
	Online    OnlineStatusEnum `json:"online"`
	CreatedAt time.Time        `json:"create_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}
