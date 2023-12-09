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
	StatusType      StatusType `bson:"status_type" json:"status_type"`
	UserID          int        `bson:"user_id" json:"user_id"`
	AddFriendStatus struct {
		FriendID int `json:"friend_user_id"`
	} `json:"add_friend_status,omitempty"`
	RemoveFriendStatus struct {
		FriendID int `json:"friend_user_id"`
	} `json:"remove_friend_status,omitempty"`
	AddChannelStatus struct {
		ChannelID int `json:"channel_id"`
	} `json:"add_channel_status,omitempty"`
	RemoveChannelStatus struct {
		ChannelID int `json:"channel_id"`
	} `json:"remove_channel_status,omitempty"`
}
