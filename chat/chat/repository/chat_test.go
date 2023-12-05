package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestChat(t *testing.T) {
	suiteSetup := func(fn func(*testing.T, context.Context, *ChatRepo)) {
		ctx := context.Background()

		mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
		if err != nil {
			panic(err)
		}

		// Clean up the container
		defer func() {
			if err := mongodbContainer.Terminate(ctx); err != nil {
				panic(err)
			}
		}()

		mongoHost, err := mongodbContainer.Host(ctx)
		assert.Nil(t, err)
		mongoPort, err := mongodbContainer.MappedPort(ctx, "27017")
		assert.Nil(t, err)

		mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
		assert.Nil(t, err)
		defer func() {
			if err := mongoDB.Disconnect(ctx); err != nil {
				assert.Nil(t, err)
			}
		}()

		chatRepo, err := CreateChatRepo(mongoDB, nil, SetPageSize(3))
		assert.Nil(t, err)

		fn(t, ctx, chatRepo)
	}

	tests := []struct {
		scenario string
		fn       func(*testing.T, context.Context, *ChatRepo)
	}{
		{
			scenario: "test get history message by channel",
			fn:       testGetHistoryMessageByChannel,
		},
		{
			scenario: "test get history message by friend",
			fn:       testGetHistoryMessageByFriend,
		},
		{
			scenario: "test get history message",
			fn:       testGetHistoryMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			suiteSetup(test.fn)
		})
	}
}

func testGetHistoryMessage(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	for i := 1; i <= 10; i++ {
		err := chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 2, 2, "content: d"+strconv.Itoa(i))
		assert.Nil(t, err)
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendOrChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendOrChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessage(ctx, 1, 0, curPage)
			assert.Nil(t, err)
			if isEnd {
				assert.Equal(t, 2, len(historyMessage))
			} else {
				assert.Equal(t, 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 8, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(t, domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
			assert.Equal(t, domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
			assert.Equal(t, "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].FriendMessage.Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].ChannelMessage.Content)
		}
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendOrChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendOrChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessage(ctx, 1, 2, curPage)
			assert.Nil(t, err)
			assert.Equal(t, 3, len(historyMessage))

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 7, curPage)

		for i := 0; i < 9; i++ {
			assert.Equal(t, domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
			assert.Equal(t, domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
			assert.Equal(t, "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].FriendMessage.Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].ChannelMessage.Content)
		}
	}
}

func testGetHistoryMessageByFriend(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	for i := 1; i <= 10; i++ {
		err := chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertFriendMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
		assert.Nil(t, err)
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 0, curPage)
			assert.Nil(t, err)
			if isEnd {
				assert.Equal(t, 1, len(historyMessage))
			} else {
				assert.Equal(t, 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 5, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+1), allHistoryMessage[i].Content)
		}
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 2, curPage)
			assert.Nil(t, err)
			if isEnd {
				assert.Equal(t, 2, len(historyMessage))
			} else {
				assert.Equal(t, 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 4, curPage)

		for i := 0; i < 8; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+3), allHistoryMessage[i].Content)
		}
	}
}

func testGetHistoryMessageByChannel(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	for i := 1; i <= 10; i++ {
		err := chatRepo.InsertChannelMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 2, 1, "content: b"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 1, 2, "content: c"+strconv.Itoa(i))
		assert.Nil(t, err)
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.ChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.ChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByChannel(ctx, 1, 0, curPage)
			assert.Nil(t, err)
			if isEnd {
				assert.Equal(t, 2, len(historyMessage))
			} else {
				assert.Equal(t, 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 8, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].Content)
		}
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.ChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.ChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByChannel(ctx, 1, 2, curPage)
			assert.Nil(t, err)
			assert.Equal(t, 3, len(historyMessage))

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 7, curPage)

		for i := 0; i < 9; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].Content)
		}
	}
}
