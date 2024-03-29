package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"gorm.io/gorm"

	httpKit "github.com/superj80820/system-design/kit/http"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
)

type channel struct {
	*domain.Channel
}

func (channel) TableName() string {
	return "channel"
}

type accountChatInformation struct {
	*domain.AccountChatInformation // TODO: is great way
}

func (accountChatInformation) TableName() string {
	return "account_chat_information"
}

type accountChannel struct {
	*domain.AccountChannel
}

func (accountChannel) TableName() string {
	return "account_channel"
}

type accountFriend struct {
	*domain.AccountFriend
}

func (accountFriend) TableName() string {
	return "account_friend"
}

type statusMessage struct {
	*domain.StatusMessage
}

func (s statusMessage) GetKey() string {
	return strconv.Itoa(s.UserID)
}

func (s statusMessage) Marshal() ([]byte, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return jsonData, nil
}

type ChannelInfo struct {
	ID   int    `bson:"channel_id"`
	Name string `bson:"channel_name"`
}

var _ domain.ChatRepository = (*ChatRepo)(nil)

type ChatRepo struct {
	messageMetadataCollection *mongo.Collection
	channelMessageCollection  *mongo.Collection
	friendMessageCollection   *mongo.Collection

	channelMessageTopic     mq.MQTopic // TODO: to domain
	accountMessageTopic     mq.MQTopic
	accountStatusTopic      mq.MQTopic
	friendOnlineStatusTopic mq.MQTopic

	mysqlDB *ormKit.DB

	pageSize int

	uniqueIDGenerate *utilKit.UniqueIDGenerate

	channelMessageObservers     utilKit.GenericSyncMap[string, mq.Observer]
	accountMessageObservers     utilKit.GenericSyncMap[string, mq.Observer]
	accountStatusObservers      utilKit.GenericSyncMap[string, mq.Observer]
	friendOnlineStatusObservers utilKit.GenericSyncMap[string, mq.Observer]
}

func CreateChatRepo(
	client *mongo.Client,
	db *ormKit.DB,
	channelMessageTopic,
	accountMessageTopic,
	accountStatusTopic,
	friendOnlineStatusTopic mq.MQTopic,
	options ...ChatRepoOption) (*ChatRepo, error) {
	messageMetadataCollection := client.Database("chat").Collection("message_metadata")
	channelMessageCollection := client.Database("chat").Collection("channel_message")
	friendMessageCollection := client.Database("chat").Collection("friend_message")

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "gen unique id generate failed")
	}

	chatRepo := ChatRepo{
		messageMetadataCollection: messageMetadataCollection,
		channelMessageCollection:  channelMessageCollection,
		friendMessageCollection:   friendMessageCollection,

		pageSize: 20,

		uniqueIDGenerate: uniqueIDGenerate,

		mysqlDB: db,

		channelMessageTopic:     channelMessageTopic,
		accountMessageTopic:     accountMessageTopic,
		accountStatusTopic:      accountStatusTopic,
		friendOnlineStatusTopic: friendOnlineStatusTopic,
	}

	for _, option := range options {
		option(&chatRepo)
	}

	return &chatRepo, nil
}

type ChatRepoOption func(*ChatRepo)

func SetPageSize(size int) ChatRepoOption {
	return func(cr *ChatRepo) {
		cr.pageSize = size
	}
}

func (chat *ChatRepo) SubscribeUserStatus(ctx context.Context, userID int, notify func(*domain.StatusMessage) error) {
	accountStatusObserver := chat.accountStatusTopic.Subscribe(strconv.Itoa(userID), func(message []byte) error {
		var statusMessage domain.StatusMessage
		if err := json.Unmarshal(message, &statusMessage); err != nil {
			return errors.Wrap(err, "unmarshal status message failed")
		}

		if statusMessage.UserID != userID {
			return nil
		}

		return notify(&statusMessage)
	})
	chat.accountStatusObservers.Store(strconv.FormatInt(httpKit.GetRequestID(ctx), 10)+":"+strconv.Itoa(userID), accountStatusObserver)
}

func (chat *ChatRepo) SubscribeChannelMessage(ctx context.Context, channelID int, notify func(*domain.ChannelMessage) error) {
	channelMessageObserver := chat.channelMessageTopic.Subscribe(strconv.Itoa(channelID), func(message []byte) error {
		var channelMessage domain.ChannelMessage
		if err := json.Unmarshal(message, &channelMessage); err != nil {
			return errors.Wrap(err, "unmarshal channel message failed")
		}

		if channelMessage.ChannelID != int64(channelID) {
			return nil
		}

		return notify(&channelMessage)
	})
	chat.channelMessageObservers.Store(strconv.FormatInt(httpKit.GetRequestID(ctx), 10)+":"+strconv.Itoa(channelID), channelMessageObserver)
}

func (chat *ChatRepo) SubscribeFriendOnlineStatus(ctx context.Context, friendID int, notify func(*domain.StatusMessage) error) {
	friendOnlineStatusObserver := chat.friendOnlineStatusTopic.Subscribe(strconv.Itoa(friendID), func(message []byte) error {
		var statusMessage domain.StatusMessage
		if err := json.Unmarshal(message, &statusMessage); err != nil {
			return errors.Wrap(err, "unmarshal status message failed")
		}

		if statusMessage.UserID != friendID ||
			(statusMessage.StatusType != domain.OnlineStatusType &&
				statusMessage.StatusType != domain.OfflineStatusType) {
			return nil
		}

		return notify(&statusMessage)
	})
	chat.friendOnlineStatusObservers.Store(strconv.FormatInt(httpKit.GetRequestID(ctx), 10)+":"+strconv.Itoa(friendID), friendOnlineStatusObserver)
}

func (chat *ChatRepo) SubscribeFriendMessage(ctx context.Context, userID int, notify func(*domain.FriendMessage) error) {
	accountMessageObserver := chat.accountMessageTopic.Subscribe(strconv.Itoa(userID), func(message []byte) error {
		var friendMessage domain.FriendMessage
		if err := json.Unmarshal(message, &friendMessage); err != nil {
			return errors.Wrap(err, "unmarshal friend message failed")
		}

		if friendMessage.FriendID != int64(userID) {
			return nil
		}

		return notify(&friendMessage)
	})
	chat.accountMessageObservers.Store(strconv.FormatInt(httpKit.GetRequestID(ctx), 10)+":"+strconv.Itoa(userID), accountMessageObserver)
}

func (chat *ChatRepo) GetHistoryMessage(ctx context.Context, accountID, offset, page int) ([]*domain.FriendOrChannelMessage, bool, error) {
	skip := int64(chat.pageSize*(page-1) + offset)
	limit := int64(chat.pageSize)
	filter := bson.D{{Key: "user_id", Value: accountID}}

	countOpts := options.Count()
	countOpts.SetSkip(int64(offset))
	historyMessageCount, err := chat.messageMetadataCollection.CountDocuments(ctx, filter, countOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "count history message failed")
	}

	findOpts := options.Find()
	findOpts.SetSkip(skip)
	findOpts.SetLimit(limit)
	cur, err := chat.messageMetadataCollection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}
	defer cur.Close(ctx)

	var metadatas []*domain.MessageMetadata
	if err := cur.All(ctx, &metadatas); err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}

	channelMessages := make(map[int64]*domain.ChannelMessage)
	friendMessages := make(map[int64]*domain.FriendMessage)
	channelMessagesCh := make(chan *domain.ChannelMessage)
	friendMessagesCh := make(chan *domain.FriendMessage)
	eg := new(errgroup.Group)

	eg.Go(func() error {
		var messageIDs bson.A
		for _, metadata := range metadatas {
			if metadata.MessageType == domain.ChannelMessageType {
				messageIDs = append(messageIDs, metadata.MessageID)
			}
		}
		channelMessageFilter := bson.D{} // TODO:?
		if len(messageIDs) != 0 {
			channelMessageFilter = bson.D{{"message_id", bson.D{{"$in", messageIDs}}}} // TODO: check performance
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cur, err := chat.channelMessageCollection.Find(ctx, channelMessageFilter)
		if err != nil {
			return errors.Wrap(err, "get history message failed")
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var channelMessage domain.ChannelMessage
			if err := cur.Decode(&channelMessage); err != nil {
				return errors.Wrap(err, "decode channel message failed")
			}
			channelMessagesCh <- &channelMessage
		}
		return nil
	})

	eg.Go(func() error {
		var messageIDs bson.A
		for _, metadata := range metadatas {
			if metadata.MessageType == domain.FriendMessageType {
				messageIDs = append(messageIDs, metadata.MessageID)
			}
		}
		friendMessageFilter := bson.D{} // TODO:?
		if len(messageIDs) != 0 {
			friendMessageFilter = bson.D{{"message_id", bson.D{{"$in", messageIDs}}}} // TODO: check performance
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cur, err := chat.friendMessageCollection.Find(ctx, friendMessageFilter)
		if err != nil {
			return errors.Wrap(err, "get history message failed")
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var friendMessage domain.FriendMessage
			if err := cur.Decode(&friendMessage); err != nil {
				return errors.Wrap(err, "decode friend message failed")
			}
			friendMessagesCh <- &friendMessage
		}
		return nil
	})

	if err := func() error { // TODO: is great way?
		errCh := make(chan error)
		go func() {
			errCh <- eg.Wait()
		}()
		for {
			select {
			case message := <-channelMessagesCh:
				channelMessages[message.MessageID] = message
			case message := <-friendMessagesCh:
				friendMessages[message.MessageID] = message
			case err := <-errCh:
				return err
			}
		}
	}(); err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}

	results := make([]*domain.FriendOrChannelMessage, len(metadatas))
	for idx, metadata := range metadatas {
		switch metadata.MessageType {
		case domain.ChannelMessageType:
			results[idx] = &domain.FriendOrChannelMessage{
				MessageType:    domain.ChannelMessageType,
				ChannelMessage: channelMessages[metadata.MessageID],
			}
		case domain.FriendMessageType:
			results[idx] = &domain.FriendOrChannelMessage{
				MessageType:   domain.FriendMessageType,
				FriendMessage: friendMessages[metadata.MessageID],
			}
		}
	}

	curHistoryMessageCount := chat.pageSize*(page-1) + len(results)
	if int64(curHistoryMessageCount) >= historyMessageCount {
		return results, true, nil
	}
	return results, false, nil
}

func (chat *ChatRepo) CreateChannel(ctx context.Context, userID int, channelName string) (int64, error) {
	channelID := chat.uniqueIDGenerate.Generate().GetInt64()

	if err := chat.mysqlDB.Transaction(func(tx *gorm.DB) error {
		channelInstance := channel{Channel: &domain.Channel{
			ChannelID:        channelID,
			Name:             channelName,
			CreatorAccountID: int64(userID),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}} // TODO: is great way?
		result := chat.
			mysqlDB.
			Create(&channelInstance)
		if mySQLErr, ok := ormKit.ConvertMySQLErr(result.Error); ok {
			return errors.Wrap(mySQLErr, "create mysql error")
		} else if result.Error != nil {
			return errors.Wrap(result.Error, "create channel failed")
		}

		accountChannelInstance := accountChannel{ // TODO: should use CreateAccountChannels?
			AccountChannel: &domain.AccountChannel{
				ID:        chat.uniqueIDGenerate.Generate().GetInt64(),
				AccountID: int64(userID),
				ChannelID: channelID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}
		result = chat.
			mysqlDB.
			Create(&accountChannelInstance)
		if mySQLErr, ok := ormKit.ConvertMySQLErr(result.Error); ok {
			return errors.Wrap(mySQLErr, "get mysql error")
		} else if result.Error != nil {
			return errors.Wrap(result.Error, "create user to channel failed")
		}
		return nil
	}); err != nil {
		return 0, err
	}

	return channelID, nil
}

func (chat *ChatRepo) CreateAccountChannels(ctx context.Context, userID, channelID int) error {
	accountChannelInstance := accountChannel{
		AccountChannel: &domain.AccountChannel{
			ID:        chat.uniqueIDGenerate.Generate().GetInt64(),
			AccountID: int64(userID),
			ChannelID: int64(channelID),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	result := chat.
		mysqlDB.
		Create(&accountChannelInstance)
	if mySQLErr, ok := ormKit.ConvertMySQLErr(result.Error); ok {
		return errors.Wrap(mySQLErr, "get mysql error")
	} else if result.Error != nil {
		return errors.Wrap(result.Error, "create user to channel failed")
	}

	return nil
}

func (chat *ChatRepo) GetAccountChannels(ctx context.Context, userID int) ([]int64, error) {
	var accountChannels []*domain.AccountChannel // TODO: test pointer
	err := chat.
		mysqlDB.
		Model(&accountChannel{}). // TODO: test model work
		Where("account_id = ?", userID).
		Find(&accountChannels).Error
	if err != nil {
		return nil, errors.Wrap(err, "get channels failed")
	}
	channels := make([]int64, len(accountChannels))
	for idx, channel := range accountChannels {
		channels[idx] = channel.ChannelID
	}
	return channels, nil
}

func (chat *ChatRepo) CreateAccountFriends(ctx context.Context, userID, friendID int) error {
	if userID == friendID {
		return errors.New("can not create same user to friend")
	}
	user1ID, user2ID := userID, friendID
	if userID > friendID {
		user1ID, user2ID = friendID, userID
	}
	accountFriendInstance := accountFriend{
		AccountFriend: &domain.AccountFriend{
			ID:         chat.uniqueIDGenerate.Generate().GetInt64(),
			Account1ID: int64(user1ID),
			Account2ID: int64(user2ID),
		},
	}
	result := chat.
		mysqlDB.
		Create(&accountFriendInstance)
	if mySQLErr, ok := ormKit.ConvertMySQLErr(result.Error); ok {
		return errors.Wrap(mySQLErr, "get mysql error")
	} else if result.Error != nil {
		return errors.Wrap(result.Error, "create friend failed")
	}

	return nil
}

func (chat *ChatRepo) GetAccountFriends(ctx context.Context, userID int) ([]int64, error) {
	var accountFriends []*domain.AccountFriend // TODO: test pointer
	err := chat.
		mysqlDB.
		Model(&accountFriend{}). // TODO: test model work
		Where("account_1_id = ? or account_2_id = ?", userID, userID).
		Find(&accountFriends).Error
	if err != nil {
		return nil, errors.Wrap(err, "get channels failed")
	}
	friends := make([]int64, len(accountFriends))
	for idx, friend := range accountFriends {
		if friend.Account1ID == int64(userID) {
			friends[idx] = friend.Account2ID
		} else {
			friends[idx] = friend.Account1ID
		}
	}
	return friends, nil // TODO: check
}

func (chat *ChatRepo) UpdateOnlineStatus(ctx context.Context, userID int, onlineStatus domain.OnlineStatusEnum) error {
	err := chat.
		mysqlDB.
		Model(&accountChatInformation{}).
		Where("account_id = ?", userID).
		Updates(accountChatInformation{
			AccountChatInformation: &domain.AccountChatInformation{
				Online:    onlineStatus,
				UpdatedAt: time.Now(),
			},
		}).Error
	if err != nil {
		return errors.Wrap(err, "get user chat information failed")
	}
	return nil
}

func (chat *ChatRepo) GetOrCreateUserChatInformation(ctx context.Context, userID int) (*domain.AccountChatInformation, error) {
	var information accountChatInformation // TODO: is great way?
	err := chat.
		mysqlDB.
		Where(accountChatInformation{AccountChatInformation: &domain.AccountChatInformation{
			AccountID: int64(userID),
		}}).
		Attrs(accountChatInformation{AccountChatInformation: &domain.AccountChatInformation{
			ID:        chat.uniqueIDGenerate.Generate().GetInt64(),
			AccountID: int64(userID),
			Online:    domain.OfflineStatus,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}}).
		FirstOrCreate(&information).Error
	if err != nil {
		return nil, errors.Wrap(err, "get user chat information failed")
	}
	return information.AccountChatInformation, nil
}

func (chat *ChatRepo) SendUserOnlineStatusMessage(ctx context.Context, userID int, onlineStatus domain.StatusType) error {
	if err := chat.accountStatusTopic.Produce(ctx, statusMessage{
		StatusMessage: &domain.StatusMessage{
			StatusType: onlineStatus,
			UserID:     userID,
		}}); err != nil {
		return errors.Wrap(err, "produce user online status failed")
	}
	return nil
}

func (chat *ChatRepo) SendAddFriendStatusMessage(ctx context.Context, userID int, friendID int) error {
	if err := chat.accountStatusTopic.Produce(ctx, statusMessage{
		StatusMessage: &domain.StatusMessage{
			StatusType: domain.AddFriendStatusType,
			UserID:     userID,
			AddFriendStatus: &domain.AddFriendStatus{
				FriendID: friendID,
			},
		}}); err != nil {
		return errors.Wrap(err, "produce add friend status failed")
	}
	return nil
}

func (chat *ChatRepo) SendJoinChannelStatusMessage(ctx context.Context, userID int, channelID int) error {
	if err := chat.accountStatusTopic.Produce(ctx, statusMessage{
		StatusMessage: &domain.StatusMessage{
			StatusType: domain.JoinChannelStatusType,
			UserID:     userID,
			JoinChannelStatus: &domain.JoinChannelStatus{
				ChannelID: channelID,
			},
		}}); err != nil {
		return errors.Wrap(err, "produce add friend status failed")
	}
	return nil
}

func (chat *ChatRepo) UnSubscribeChannelMessage(ctx context.Context, channelID int) {
	key := strconv.FormatInt(httpKit.GetRequestID(ctx), 10) + ":" + strconv.Itoa(channelID)
	if observer, ok := chat.channelMessageObservers.LoadAndDelete(key); ok {
		chat.channelMessageTopic.UnSubscribe(observer)
		chat.channelMessageObservers.Delete(key)
	}
}

func (chat *ChatRepo) UnSubscribeFriendMessage(ctx context.Context, userID int) {
	key := strconv.FormatInt(httpKit.GetRequestID(ctx), 10) + ":" + strconv.Itoa(userID)
	if observer, ok := chat.accountMessageObservers.LoadAndDelete(key); ok {
		chat.accountMessageTopic.UnSubscribe(observer)
		chat.accountMessageObservers.Delete(key)
	}
}

func (chat *ChatRepo) UnSubscribeFriendOnlineStatus(ctx context.Context, friendID int) {
	key := strconv.FormatInt(httpKit.GetRequestID(ctx), 10) + ":" + strconv.Itoa(friendID)
	if observer, ok := chat.friendOnlineStatusObservers.LoadAndDelete(key); ok {
		chat.friendOnlineStatusTopic.UnSubscribe(observer)
		chat.friendOnlineStatusObservers.Delete(key)
	}
}

func (chat *ChatRepo) UnSubscribeAll(ctx context.Context) {
	requestID := strconv.FormatInt(httpKit.GetRequestID(ctx), 10)
	chat.accountMessageObservers.Range(func(key string, observer mq.Observer) bool {
		if strings.Split(key, ":")[0] != requestID { // TODO: performance
			return true
		}
		chat.accountMessageTopic.UnSubscribe(observer)
		chat.accountMessageObservers.Delete(key)
		return true
	})
	chat.channelMessageObservers.Range(func(key string, observer mq.Observer) bool {
		if strings.Split(key, ":")[0] != requestID { // TODO: performance
			return true
		}
		chat.channelMessageTopic.UnSubscribe(observer)
		chat.channelMessageObservers.Delete(key)
		return true
	})
	chat.accountStatusObservers.Range(func(key string, observer mq.Observer) bool {
		if strings.Split(key, ":")[0] != requestID { // TODO: performance
			return true
		}
		chat.accountStatusTopic.UnSubscribe(observer)
		chat.accountStatusObservers.Delete(key)
		return true
	})
	chat.friendOnlineStatusObservers.Range(func(key string, observer mq.Observer) bool {
		if strings.Split(key, ":")[0] != requestID { // TODO: performance
			return true
		}
		chat.friendOnlineStatusTopic.UnSubscribe(observer)
		chat.friendOnlineStatusObservers.Delete(key)
		return true
	})
}
