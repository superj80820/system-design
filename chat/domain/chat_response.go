package domain

type ResponseMessageType int

const (
	FriendResponseMessageType ResponseMessageType = iota + 1
	ChannelResponseMessageType
	StatusMessageResponseMessageType
	FriendOrChannelResponseMessageType
	UserChannelsResponseMessageType
	UserFriendsResponseMessageType
	FriendOrChannelMessageHistoryResponseMessageType
	FriendMessageHistoryResponseMessageType
	ChannelMessageHistoryResponseMessageType
	FriendOnlineStatusResponseMessageType
)

type ChatResponse struct {
	MessageType                   ResponseMessageType            `json:"message_type"`
	StatusMessage                 *StatusMessage                 `json:"status_message,omitempty"`
	FriendMessage                 *FriendMessage                 `json:"friend_message,omitempty"`
	ChannelMessage                *ChannelMessage                `json:"channel_message,omitempty"`
	FriendOrChannelMessageHistory *FriendOrChannelMessageHistory `json:"friend_or_channel_message_history,omitempty"`
	FriendMessageHistory          *FriendMessageHistory          `json:"friend_message_history,omitempty"`
	ChannelMessageHistory         *ChannelMessageHistory         `json:"channel_message_history,omitempty"`
	UserChannels                  []int64                        `json:"user_channels,omitempty"`
	UserFriends                   []int64                        `json:"user_friends,omitempty"`
	FriendOnlineStatus            *FriendOnlineStatus            `json:"friend_online_status,omitempty"`
}

type ChannelMessageHistory struct {
	HistoryMessage []*ChannelMessage `json:"history_message"`
	IsEnd          bool              `json:"is_end"`
}

type FriendMessageHistory struct {
	HistoryMessage []*FriendMessage `json:"history_message"`
	IsEnd          bool             `json:"is_end"`
}

type FriendOrChannelMessageHistory struct {
	HistoryMessage []*FriendOrChannelMessage `json:"history_message"`
	IsEnd          bool                      `json:"is_end"`
}

type FriendOnlineStatus struct {
	OnlineStatus OnlineStatusEnum `json:"online_status"`
	FriendID     int64            `json:"friend_id"`
}
