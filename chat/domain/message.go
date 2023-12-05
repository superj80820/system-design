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
	StatusType StatusType `bson:"status_type" json:"status_type"`
	UserID     int        `bson:"user_id" json:"user_id"`
	Content    string     `bson:"content" json:"content"`
}

type AddFriendStatus struct {
	FriendUserID int
}

type RemoveFriendStatus struct {
	FriendUserID int
}

type AddChannelStatus struct {
	ChannelID int
}

type RemoveChannelStatus struct {
	ChannelID int
}
