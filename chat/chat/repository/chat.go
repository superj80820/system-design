package repository

import (
	"context"
	"strconv"
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

type accountChatInformation struct {
	*domain.AccountChatInformation
}

func (accountChatInformation) TableName() string {
	return "account_chat_information"
}

type channel struct {
	*domain.Channel
}

func (channel) TableName() string {
	return "channel_account"
}

type accountFriend struct {
	*domain.Account
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

func (chat *ChatRepo) GetChannelByName(name string) (*domain.Channel, error) {
	var channelInstance channel
	if err := chat.mysqlDB.First(&channelInstance, "name = ?", name); err != nil {
		return nil, errors.Wrap(err, "get channel failed")
	}
	return channelInstance.Channel, nil
	// TODO
	// uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	// if err != nil {
	// 	panic(err) // TODO
	// }

	// var (
	// 	channelID   int
	// 	channelInfo ChannelInfo
	// )

	// filter := bson.D{{Key: "channel_name", Value: channelName}}
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// err = chat.channelMessageCollection.FindOne(ctx, filter).Decode(&channelInfo)
	// if err == mongo.ErrNoDocuments {
	// 	fmt.Println("record does not exist")

	// 	channelID = int(uniqueIDGenerate.Generate().GetInt64())

	// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // TODO: need time?
	// 	defer cancel()
	// 	chat.channelMessageCollection.InsertOne(ctx, bson.D{
	// 		{Key: "channel_id", Value: channelID},
	// 		{Key: "channel_name", Value: channelName},
	// 	})
	// } else if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	channelID = channelInfo.ID
	// }

	// return channelID
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
	for idx, metadata := range metadatas { // TODO: check sort
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

func (chat *ChatRepo) GetAccountChannels(ctx context.Context, id int) ([]*domain.Channel, error) {
	var channels []*domain.Channel // TODO: test pointer
	if err := chat.
		mysqlDB.
		Model(&channel{}). // TODO: test model work
		Where("account_id = ?", strconv.Itoa(id)).Find(&channels).Error; err != nil {
		return nil, errors.Wrap(err, "get channels failed")
	}
	return channels, nil
}

func (chat *ChatRepo) GetAccountFriends(ctx context.Context, id int) ([]*domain.Account, error) {
	var accounts []*domain.Account // TODO: test pointer
	if err := chat.
		mysqlDB.
		Model(&accountFriend{}). // TODO: test model work
		Where("account_id = ?", strconv.Itoa(id)).Find(&accounts).Error; err != nil {
		return nil, errors.Wrap(err, "get channels failed")
	}
	return accounts, nil
}

func (chat *ChatRepo) Offline(ctx context.Context, id int) {
	chat.
		mysqlDB.
		Model(&accountChatInformation{}).
		Where("account_id = ?", id).
		Update("online", 0)
}

func (chat *ChatRepo) Online(ctx context.Context, id int) {
	chat.
		mysqlDB.
		Model(&accountChatInformation{}).
		Where("account_id = ?", id).
		Update("online", 1)
}
