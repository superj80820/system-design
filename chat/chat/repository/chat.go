package repository

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/chat/domain"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
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

type ChannelInfo struct {
	ID   int    `bson:"channel_id"`
	Name string `bson:"channel_name"`
}

type ChatRepo struct {
	messageMetadataCollection *mongo.Collection
	channelMessageCollection  *mongo.Collection
	friendMessageCollection   *mongo.Collection

	mysqlDB *mysqlKit.DB

	pageSize int

	uniqueIDGenerate *utilKit.UniqueIDGenerate
}

type ChatRepoOption func(*ChatRepo)

func SetPageSize(size int) ChatRepoOption {
	return func(cr *ChatRepo) {
		cr.pageSize = size
	}
}

func CreateChatRepo(client *mongo.Client, db *mysqlKit.DB, options ...ChatRepoOption) (*ChatRepo, error) {
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
	}

	for _, option := range options {
		option(&chatRepo)
	}

	return &chatRepo, nil
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
	// TODO: tx
	channelID := chat.uniqueIDGenerate.Generate().GetInt64()
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
	if mySQLErr, ok := mysqlKit.ConvertMySQLErr(result.Error); ok {
		return 0, errors.Wrap(mySQLErr, "create mysql error")
	} else if result.Error != nil {
		return 0, errors.Wrap(result.Error, "create channel failed")
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
	if mySQLErr, ok := mysqlKit.ConvertMySQLErr(result.Error); ok {
		return 0, errors.Wrap(mySQLErr, "get mysql error")
	} else if result.Error != nil {
		return 0, errors.Wrap(result.Error, "create user to channel failed")
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
	if mySQLErr, ok := mysqlKit.ConvertMySQLErr(result.Error); ok {
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
	if mySQLErr, ok := mysqlKit.ConvertMySQLErr(result.Error); ok {
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
