package domain

type StatusType int

const (
	AddFriendStatusType StatusType = iota + 1
	RemoveFriendStatusType
	AddChannelStatusType
	RemoveChannelStatusType
	OnlineStatusType
	OfflineStatusType
)

type StatusMessage struct {
	StatusType          StatusType           `bson:"status_type" json:"status_type"`
	UserID              int                  `bson:"user_id" json:"user_id"`
	AddFriendStatus     *AddFriendStatus     `json:"add_friend_status,omitempty"`
	RemoveFriendStatus  *RemoveFriendStatus  `json:"remove_friend_status,omitempty"`
	AddChannelStatus    *AddChannelStatus    `json:"add_channel_status,omitempty"`
	RemoveChannelStatus *RemoveChannelStatus `json:"remove_channel_status,omitempty"`
}

type AddFriendStatus struct {
	FriendID int `json:"friend_user_id"`
}

type RemoveFriendStatus struct {
	FriendID int `json:"friend_user_id"`
}

type AddChannelStatus struct {
	ChannelID int `json:"channel_id"`
}

type RemoveChannelStatus struct {
	ChannelID int `json:"channel_id"`
}
