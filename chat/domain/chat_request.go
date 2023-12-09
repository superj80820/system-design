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
	Action        ChatClientAction `json:"action"`
	SendFriendReq struct {
		FriendID int64  `json:"friend_id"`
		Message  string `json:"message"`
	} `json:"send_friend_req,omitempty"`
	SendChannelReq struct {
		ChannelID int64  `json:"channel_id"`
		Message   string `json:"message"`
	} `json:"send_channel_req,omitempty"`
	ChannelHistoryReq struct {
	} `json:"channel_history_req"`
	JoinChannelReq struct {
		ChannelName     string `json:"channel_name"`
		CurMaxMessageID int    `json:"cur_max_message_id,omitempty"`
	} `json:"join_channel_req,omitempty"`
	GetFriendHistoryMessageReq struct { // TODO
		FriendID        int `json:"friend_id"`
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	} `json:"get_friend_history_message_req,omitempty"`
	GetChannelHistoryMessageReq struct { // TODO
		ChannelID       int `json:"channel_id"`
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	} `json:"get_channel_history_message_req,omitempty"`
	GetHistoryMessageReq struct { // TODO
		CurMaxMessageID int `json:"cur_max_message_id"`
		Page            int `json:"page"`
	} `json:"get_history_message_req,omitempty"`
}
