package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/chat/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChannelInfo struct {
	ID   int    `bson:"channel_id"`
	Name string `bson:"channel_name"`
}

type ChatRepo struct {
	messageCollection *mongo.Collection
	channelCollection *mongo.Collection
}

type Cursor[T any] struct {
	*mongo.Cursor
}

func (c *Cursor[T]) Decode() (*T, error) { // TODO: need?
	result := new(T)
	if err := c.Cursor.Decode(result); err != nil {
		return nil, errors.Wrap(err, "TODO")
	}
	return result, nil
}

func MakeChatRepo(client *mongo.Client) *ChatRepo {
	messageCollection := client.Database("chat").Collection("message")
	channelCollection := client.Database("chat").Collection("channel")

	return &ChatRepo{
		messageCollection: messageCollection,
		channelCollection: channelCollection,
	}
}

func (chat *ChatRepo) GetChannelID(channelName string) int {
	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		panic(err) // TODO
	}

	var (
		channelID   int
		channelInfo ChannelInfo
	)

	filter := bson.D{{Key: "channel_name", Value: channelName}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = chat.channelCollection.FindOne(ctx, filter).Decode(&channelInfo)
	if err == mongo.ErrNoDocuments {
		fmt.Println("record does not exist")

		channelID = int(uniqueIDGenerate.Generate().GetInt64())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // TODO: need time?
		defer cancel()
		chat.channelCollection.InsertOne(ctx, bson.D{
			{Key: "channel_id", Value: channelID},
			{Key: "channel_name", Value: channelName},
		})
	} else if err != nil {
		log.Fatal(err)
	} else {
		channelID = channelInfo.ID
	}

	return channelID
}

func (chat *ChatRepo) GetHistory(curMaxMessageID, channelID int) *Cursor[domain.ChannelMessage] {
	var (
		opts          []*options.FindOptions
		historyFilter bson.M
	)

	if curMaxMessageID == -1 {
		historyFilter = bson.M{"channel_id": channelID}
		opts = append(opts, options.Find().SetLimit(10))
	} else {
		historyFilter = bson.M{"channel_id": channelID, "message_id": bson.M{"$gte": curMaxMessageID}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := chat.messageCollection.Find(ctx, historyFilter, opts...)
	if err != nil {
		log.Fatal(err)
	} // TODO

	return &Cursor[domain.ChannelMessage]{cur}
}

func (chat *ChatRepo) InsertMessage(message *domain.ChannelMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // TODO: need time?
	defer cancel()
	chat.messageCollection.InsertOne(ctx, bson.D{
		{Key: "channel_id", Value: message.ChannelID},
		{Key: "message_id", Value: message.MessageID},
		{Key: "user_id", Value: message.UserID},
		{Key: "content", Value: message.Content},
	})
}
