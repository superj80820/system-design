package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
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

		mysqlDBName := "db"
		mysqlDBUsername := "root"
		mysqlDBPassword := "password"
		mysqlContainer, err := mysql.RunContainer(ctx,
			testcontainers.WithImage("mysql:8"),
			mysql.WithDatabase(mysqlDBName),
			mysql.WithUsername(mysqlDBUsername),
			mysql.WithPassword(mysqlDBPassword),
			mysql.WithScripts(filepath.Join("./../../../instrumenting", "ddl.sql")),
			mysql.WithScripts(filepath.Join("./../../../instrumenting", "mock.sql")),
		)
		if err != nil {
			panic(err)
		}

		// Clean up the container
		defer func() {
			if err := mysqlContainer.Terminate(ctx); err != nil {
				panic(err)
			}
		}()

		mysqlDBHost, err := mysqlContainer.Host(ctx)
		assert.Nil(t, err)
		mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
		assert.Nil(t, err)

		mysqlDB, err := mysqlKit.CreateDB(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			))
		if err != nil {
			panic(err)
		}

		chatRepo, err := CreateChatRepo(mongoDB, mysqlDB, SetPageSize(3))
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
		{
			scenario: "test user online",
			fn:       testUserOnline,
		},
		{
			scenario: "test get account friends",
			fn:       testGetAccountFriends,
		},
		{
			scenario: "test can not add myself to friend",
			fn:       testUserCanNotAddMyselfToFriend,
		},
		{
			scenario: "test user can not add duplicated friends",
			fn:       testUserCanNotAddDuplicatedFriends,
		},
		{
			scenario: "test get account channels",
			fn:       testGetAccountChannels,
		},
		{
			scenario: "test user can not add duplicated channels",
			fn:       testUserCanNotAddDuplicatedChannels,
		},

		{
			scenario: "test user can create same name channels",
			fn:       testUserCanCreateSameNameChannels,
		},
	}

	for _, test := range tests {
		testFn := test.fn
		t.Run(test.scenario, func(t *testing.T) {
			t.Parallel()
			suiteSetup(testFn)
		})
	}
}

func testGetAccountChannels(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 100

	channelIDs := make([]int64, 4)
	for i := 0; i < 4; i++ {
		channelID, err := chatRepo.CreateChannel(ctx, userID, strconv.Itoa(i))
		assert.Nil(t, err)

		channelIDs[i] = channelID
	}

	channels, err := chatRepo.GetAccountChannels(ctx, userID)
	assert.Nil(t, err)

	assert.Equal(t, 4, len(channels))

	for idx, channel := range channels {
		assert.Equal(t, channelIDs[idx], channel)
	}

	anotherUserID := 101
	channelID := channels[0]
	channels, err = chatRepo.GetAccountChannels(ctx, anotherUserID)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(channels))

	err = chatRepo.CreateAccountChannels(ctx, anotherUserID, int(channelID))
	assert.Nil(t, err)
	channels, err = chatRepo.GetAccountChannels(ctx, anotherUserID)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(channels))
}

func testUserCanCreateSameNameChannels(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 103

	_, err := chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(t, err)

	_, err = chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(t, err)
}

func testUserCanNotAddDuplicatedChannels(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 102

	channelID, err := chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(t, err)

	err = chatRepo.CreateAccountChannels(ctx, userID, int(channelID))
	assert.ErrorIs(t, err, mysqlKit.ErrDuplicatedKey)
}

func testGetAccountFriends(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 100

	for friendID := 101; friendID <= 104; friendID++ {
		err := chatRepo.CreateAccountFriends(ctx, userID, friendID)
		assert.Nil(t, err)
	}

	accountFriends, err := chatRepo.GetAccountFriends(ctx, userID)
	assert.Nil(t, err)

	assert.Equal(t, 4, len(accountFriends))

	for idx, friend := range accountFriends {
		assert.Equal(t, int64(101+idx), friend)
	}
}

func testUserCanNotAddMyselfToFriend(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 101

	err := chatRepo.CreateAccountFriends(ctx, userID, userID)
	assert.Equal(t, err.Error(), "can not create same user to friend")
}

func testUserCanNotAddDuplicatedFriends(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 100

	err := chatRepo.CreateAccountFriends(ctx, userID, 105)
	assert.Nil(t, err)

	err = chatRepo.CreateAccountFriends(ctx, userID, 105)
	assert.ErrorIs(t, err, mysqlKit.ErrDuplicatedKey)
}

func testUserOnline(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	userID := 100

	userChatInformation, err := chatRepo.GetOrCreateUserChatInformation(ctx, userID)
	assert.Nil(t, err)
	assert.Equal(t, domain.OfflineStatus, userChatInformation.Online)

	err = chatRepo.UpdateOnlineStatus(ctx, userID, domain.OnlineStatus)
	assert.Nil(t, err)

	userChatInformation, err = chatRepo.GetOrCreateUserChatInformation(ctx, userID)
	assert.Nil(t, err)
	assert.Equal(t, domain.OnlineStatus, userChatInformation.Online)
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
