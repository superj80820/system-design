package domain

type ChatClientAction string

const (
	SendMessageToChannel     ChatClientAction = "send_message_to_channel"
	SendMessageToFriend      ChatClientAction = "send_message_to_friend"
	GetFriendHistoryMessage  ChatClientAction = "get_friend_history_message"
	GetChannelHistoryMessage ChatClientAction = "get_channel_history_message"
	GetHistoryMessage        ChatClientAction = "get_history_message"
)

type ChatRequest struct {
	Action                      ChatClientAction             `json:"action"`
	SendFriendReq               *SendFriendReq               `json:"send_friend_req,omitempty"`
	SendChannelReq              *SendChannelReq              `json:"send_channel_req,omitempty"`
	GetFriendHistoryMessageReq  *GetFriendHistoryMessageReq  `json:"get_friend_history_message_req,omitempty"`
	GetChannelHistoryMessageReq *GetChannelHistoryMessageReq `json:"get_channel_history_message_req,omitempty"`
	GetHistoryMessageReq        *GetHistoryMessageReq        `json:"get_history_message_req,omitempty"`
}

type SendFriendReq struct {
	FriendID int64  `json:"friend_id"`
	Message  string `json:"message"`
}

type SendChannelReq struct {
	ChannelID int64  `json:"channel_id"`
	Message   string `json:"message"`
}

type GetFriendHistoryMessageReq struct {
	FriendID        int `json:"friend_id"`
	CurMaxMessageID int `json:"cur_max_message_id"`
	Page            int `json:"page"`
}

type GetChannelHistoryMessageReq struct {
	ChannelID       int `json:"channel_id"`
	CurMaxMessageID int `json:"cur_max_message_id"`
	Page            int `json:"page"`
}

type GetHistoryMessageReq struct {
	CurMaxMessageID int `json:"cur_max_message_id"`
	Page            int `json:"page"`
}
