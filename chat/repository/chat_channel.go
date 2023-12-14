package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type channelMessage struct {
	*domain.ChannelMessage
}

func (c channelMessage) GetKey() string {
	return strconv.FormatInt(c.ChannelID, 10)
}

func (c channelMessage) Marshal() ([]byte, error) {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return jsonData, nil
}

// TODO: test tx
func (chat *ChatRepo) InsertChannelMessage(ctx context.Context, userID, channelID int64, content string) (int64, error) {
	messageID := chat.uniqueIDGenerate.Generate().GetInt64()
	metadataID := chat.uniqueIDGenerate.Generate().GetInt64()

	_, err := chat.messageMetadataCollection.InsertOne(ctx, domain.MessageMetadata{
		MessageType: domain.ChannelMessageType,
		MetadataID:  metadataID,
		MessageID:   messageID,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		return 0, errors.Wrap(err, "insert metadata failed")
	}
	_, err = chat.channelMessageCollection.InsertOne(ctx, domain.ChannelMessage{
		MessageID: messageID,
		ChannelID: channelID,
		Content:   content,
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return 0, errors.Wrap(err, "insert channel message failed")
	}
	return messageID, nil
}

func (chat *ChatRepo) GetHistoryMessageByChannel(ctx context.Context, channelID, offset, page int) ([]*domain.ChannelMessage, bool, error) {
	skip := int64(chat.pageSize*(page-1) + offset)
	limit := int64(chat.pageSize)
	filter := bson.D{{Key: "channel_id", Value: channelID}}

	countOpts := options.Count()
	countOpts.SetSkip(int64(offset))
	historyMessageCount, err := chat.channelMessageCollection.CountDocuments(ctx, filter, countOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "count history message failed")
	}

	findOpts := options.Find()
	findOpts.SetSkip(skip)
	findOpts.SetLimit(limit)
	cur, err := chat.channelMessageCollection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}
	defer cur.Close(ctx)
	var results []*domain.ChannelMessage
	if err := cur.All(ctx, &results); err != nil { // TODO: pointer
		return nil, false, errors.Wrap(err, "get history message failed")
	}

	curHistoryMessageCount := chat.pageSize*(page-1) + len(results)
	if int64(curHistoryMessageCount) >= historyMessageCount {
		return results, true, nil
	}
	return results, false, nil
}
func (chat *ChatRepo) SendChannelMessage(ctx context.Context, userID, channelID, messageID int, content string) error {
	if err := chat.channelMessageTopic.Produce(ctx, channelMessage{&domain.ChannelMessage{
		MessageID: int64(messageID),
		ChannelID: int64(channelID),
		UserID:    int64(userID),
		Content:   content,
	}}); err != nil {
		return errors.Wrap(err, "produce message failed")
	}
	return nil
}
