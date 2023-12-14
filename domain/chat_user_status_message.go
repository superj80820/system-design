package domain

type StatusType int

const (
	AddFriendStatusType StatusType = iota + 1
	RemoveFriendStatusType
	JoinChannelStatusType
	LeaveChannelStatusType
	OnlineStatusType
	OfflineStatusType
)

type StatusMessage struct {
	StatusType         StatusType          `bson:"status_type" json:"status_type"`
	UserID             int                 `bson:"user_id" json:"user_id"`
	AddFriendStatus    *AddFriendStatus    `json:"add_friend_status,omitempty"`
	RemoveFriendStatus *RemoveFriendStatus `json:"remove_friend_status,omitempty"`
	JoinChannelStatus  *JoinChannelStatus  `json:"join_channel_status,omitempty"`
	LeaveChannelStatus *LeaveChannelStatus `json:"leave_channel_status,omitempty"`
}

type AddFriendStatus struct {
	FriendID int `json:"friend_user_id"`
}

type RemoveFriendStatus struct {
	FriendID int `json:"friend_user_id"`
}

type JoinChannelStatus struct {
	ChannelID int `json:"channel_id"`
}

type LeaveChannelStatus struct {
	ChannelID int `json:"channel_id"`
}
