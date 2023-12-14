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

type friendMessage struct {
	*domain.FriendMessage
}

func (f friendMessage) GetKey() string {
	return strconv.FormatInt(f.FriendID, 10)
}

func (f friendMessage) Marshal() ([]byte, error) {
	jsonData, err := json.Marshal(f)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return jsonData, nil
}

// TODO: test tx
func (chat *ChatRepo) InsertFriendMessage(ctx context.Context, userID, friendID int64, content string) (int64, error) {
	messageID := chat.uniqueIDGenerate.Generate().GetInt64()
	metadataID := chat.uniqueIDGenerate.Generate().GetInt64()

	_, err := chat.messageMetadataCollection.InsertOne(ctx, domain.MessageMetadata{
		MessageType: domain.FriendMessageType,
		MetadataID:  metadataID,
		MessageID:   messageID,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		return 0, errors.Wrap(err, "insert metadata failed")
	}
	_, err = chat.friendMessageCollection.InsertOne(ctx, domain.FriendMessage{
		MessageID: messageID,
		Content:   content,
		FriendID:  friendID,
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return 0, errors.Wrap(err, "insert friend message failed")
	}
	return messageID, nil
}

func (chat *ChatRepo) GetHistoryMessageByFriend(ctx context.Context, accountID, friendID, offset, page int) ([]*domain.FriendMessage, bool, error) { // TODO: function args
	skip := int64(chat.pageSize*(page-1) + offset)
	limit := int64(chat.pageSize)
	filter := bson.D{{Key: "user_id", Value: accountID}, {Key: "friend_id", Value: friendID}} // TODO: create index

	countOpts := options.Count()
	countOpts.SetSkip(int64(offset))
	historyMessageCount, err := chat.friendMessageCollection.CountDocuments(ctx, filter, countOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "count history message failed")
	}

	findOpts := options.Find()
	findOpts.SetSkip(skip)
	findOpts.SetLimit(limit)
	cur, err := chat.friendMessageCollection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}
	defer cur.Close(ctx)
	var results []*domain.FriendMessage
	if err := cur.All(ctx, &results); err != nil {
		return nil, false, errors.Wrap(err, "get history message failed")
	}

	curHistoryMessageCount := chat.pageSize*(page-1) + len(results)
	if int64(curHistoryMessageCount) >= historyMessageCount {
		return results, true, nil
	}
	return results, false, nil
}

func (chat *ChatRepo) SendFriendMessage(ctx context.Context, userID, friendID, messageID int, content string) error {
	if err := chat.accountMessageTopic.Produce(ctx, friendMessage{&domain.FriendMessage{
		MessageID: int64(messageID),
		Content:   content,
		UserID:    int64(userID),
		FriendID:  int64(friendID),
	}}); err != nil {
		return errors.Wrap(err, "produce message failed")
	}
	return nil
}
