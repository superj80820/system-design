package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/superj80820/system-design/chat/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	mqKit "github.com/superj80820/system-design/kit/mq"
	mqReaderManagerKit "github.com/superj80820/system-design/kit/mq/reader_manager"
	mqWriterManagerKit "github.com/superj80820/system-design/kit/mq/writer_manager"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TestSuite struct {
	suite.Suite
	chatRepo *ChatRepo

	mongoDB *mongo.Client

	mongodbContainer *mongodb.MongoDBContainer
	kafkaContainer   *kafka.KafkaContainer
	mysqlContainer   *mysql.MySQLContainer
}

func (suite *TestSuite) SetupTest() {
	ctx := context.Background()

	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	assert.Nil(suite.T(), err)
	suite.mongodbContainer = mongodbContainer

	mongoHost, err := mongodbContainer.Host(ctx)
	assert.Nil(suite.T(), err)
	mongoPort, err := mongodbContainer.MappedPort(ctx, "27017")
	assert.Nil(suite.T(), err)

	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
	assert.Nil(suite.T(), err)
	suite.mongoDB = mongoDB

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
	assert.Nil(suite.T(), err)
	suite.mysqlContainer = mysqlContainer

	mysqlDBHost, err := mysqlContainer.Host(ctx)
	assert.Nil(suite.T(), err)
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	assert.Nil(suite.T(), err)

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

	kafkaContainer, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	assert.Nil(suite.T(), err)
	suite.kafkaContainer = kafkaContainer

	kafkaHost, err := kafkaContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9093") // TODO: why
	if err != nil {
		panic(err)
	}
	brokerAddress := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())

	channelMessageTopicName := "channel-message"
	userMessageTopicName := "user-message"
	userStatusTopicName := "user-status"
	serviceName := "chat-service"

	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, channelMessageTopicName, 0)
	assert.Nil(suite.T(), err)
	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, userMessageTopicName, 0)
	assert.Nil(suite.T(), err)
	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, userStatusTopicName, 0)
	assert.Nil(suite.T(), err)

	channelMessageTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		channelMessageTopicName,
		mqKit.ConsumeByPartitionsBindObserver(mqReaderManagerKit.LastOffset),
		mqKit.ProduceWay(&mqWriterManagerKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userMessageTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		userMessageTopicName,
		mqKit.ConsumeByPartitionsBindObserver(mqReaderManagerKit.LastOffset),
		mqKit.ProduceWay(&mqWriterManagerKit.Hash{}),
	)
	if err != nil {
		panic(err)
	}
	userStatusTopic, err := mqKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		userStatusTopicName,
		mqKit.ConsumeByGroupID(serviceName+":user_status", mqReaderManagerKit.LastOffset),
	)
	if err != nil {
		panic(err)
	}
	friendOnlineStatusTopic, err := mqKit.CreateMQTopic( // TODO: need?
		context.TODO(),
		brokerAddress,
		userStatusTopicName,
		mqKit.ConsumeByGroupID(serviceName+":friend_online_status", mqReaderManagerKit.LastOffset),
	)

	chatRepo, err := CreateChatRepo(
		mongoDB,
		mysqlDB,
		channelMessageTopic,
		userMessageTopic,
		userStatusTopic,
		friendOnlineStatusTopic,
		SetPageSize(3),
	)
	assert.Nil(suite.T(), err)

	suite.chatRepo = chatRepo
}

func (suite *TestSuite) TearDownTest() {
	ctx := context.TODO()

	err := suite.mongoDB.Disconnect(ctx)
	assert.Nil(suite.T(), err)

	// Clean up the container after
	if err := suite.kafkaContainer.Terminate(ctx); err != nil {
		panic(err)
	}

	// Clean up the container
	if err := suite.mongodbContainer.Terminate(ctx); err != nil {
		panic(err)
	}

	// Clean up the container
	if err := suite.mysqlContainer.Terminate(ctx); err != nil {
		panic(err)
	}
}

// func (suite *TestSuite) TestChat() {
// 	tests := []struct {
// 		scenario string
// 		fn       func(*testing.T, context.Context)
// 	}{
// 		{
// 			scenario: "test get history message by channel",
// 			fn:       testGetHistoryMessageByChannel,
// 		},
// 		{
// 			scenario: "test get history message by friend",
// 			fn:       testGetHistoryMessageByFriend,
// 		},
// 		{
// 			scenario: "test get history message",
// 			fn:       testGetHistoryMessage,
// 		},
// 		{
// 			scenario: "test user online",
// 			fn:       testUserOnline,
// 		},
// 		{
// 			scenario: "test get account friends",
// 			fn:       testGetAccountFriends,
// 		},
// 		{
// 			scenario: "test can not add myself to friend",
// 			fn:       testUserCanNotAddMyselfToFriend,
// 		},
// 		{
// 			scenario: "test user can not add duplicated friends",
// 			fn:       testUserCanNotAddDuplicatedFriends,
// 		},
// 		{
// 			scenario: "test get account channels",
// 			fn:       testGetAccountChannels,
// 		},
// 		{
// 			scenario: "test user can not add duplicated channels",
// 			fn:       testUserCanNotAddDuplicatedChannels,
// 		},
// 		{
// 			scenario: "test user can create same name channels",
// 			fn:       testUserCanCreateSameNameChannels,
// 		},
// 		{
// 			scenario: "test user status message",
// 			fn:       testUserStatus,
// 		},
// 		{
// 			scenario: "test send friend message",
// 			fn:       testSendFriendMessage,
// 		},
// 		{
// 			scenario: "test send channel message",
// 			fn:       testSendChannelMessage,
// 		},
// 	}

// 	for _, test := range tests {
// 		testFn := test.fn
// 		suite.Run(test.scenario, func() {
// 			suite.T().Parallel()
// 			testFn(suite.T(), context.TODO())
// 		})
// 	}
// }

func TestChat(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestUserStatus() {
	ctx := context.Background()

	userID := 100
	ctx = httpKit.AddRequestID(ctx)

	done := make(chan bool)
	suite.chatRepo.SubscribeUserStatus(ctx, userID, func(sm *domain.StatusMessage) error {
		assert.Equal(suite.T(), domain.OnlineStatusType, sm.StatusType)
		close(done)
		return nil
	})
	time.Sleep(time.Second * 5) // TODO: wait subscribed
	err := suite.chatRepo.SendUserStatusMessage(ctx, userID, domain.OnlineStatusType)
	assert.Nil(suite.T(), err)
	select {
	case <-done:
	case <-time.NewTimer(time.Second * 60).C:
		assert.Fail(suite.T(), "get user status message failed")
	}
}

func (suite *TestSuite) TestSendFriendMessage() {
	ctx := context.Background()

	messageContent := "content"
	messageID := 1000
	userID := 100
	friendID := 101
	ctx = httpKit.AddRequestID(ctx)

	done := make(chan bool)
	suite.chatRepo.SubscribeFriendMessage(ctx, friendID, func(fm *domain.FriendMessage) error {
		assert.Equal(suite.T(), messageContent, fm.Content)
		close(done)
		return nil
	})
	time.Sleep(time.Second * 5) // TODO: wait subscribed
	err := suite.chatRepo.SendFriendMessage(ctx, userID, friendID, messageID, messageContent)
	assert.Nil(suite.T(), err)
	select {
	case <-done:
	case <-time.NewTimer(time.Second * 60).C:
		assert.Fail(suite.T(), "get friend message timeout")
	}
}

func (suite *TestSuite) TestSendChannelMessage() {
	ctx := context.Background()

	messageContent := "content"
	messageID := 1000
	userID := 100
	channelID := 101
	ctx = httpKit.AddRequestID(ctx)

	done := make(chan bool)
	suite.chatRepo.SubscribeChannelMessage(ctx, channelID, func(cm *domain.ChannelMessage) error {
		assert.Equal(suite.T(), messageContent, cm.Content)
		assert.Equal(suite.T(), int64(channelID), cm.ChannelID)
		assert.Equal(suite.T(), int64(userID), cm.UserID)
		close(done)
		return nil
	})
	time.Sleep(time.Second * 5) // TODO: wait subscribed
	err := suite.chatRepo.SendChannelMessage(ctx, userID, channelID, messageID, messageContent)
	assert.Nil(suite.T(), err)
	select {
	case <-done:
	case <-time.NewTimer(time.Second * 60).C:
		assert.Fail(suite.T(), "get channel message timeout")
	}
}

func (suite *TestSuite) TestGetAccountChannels() {
	ctx := context.Background()

	userID := 100

	channelIDs := make([]int64, 4)
	for i := 0; i < 4; i++ {
		channelID, err := suite.chatRepo.CreateChannel(ctx, userID, strconv.Itoa(i))
		assert.Nil(suite.T(), err)

		channelIDs[i] = channelID
	}

	channelInformation, err := suite.chatRepo.GetChannel(int(channelIDs[0]))
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), channelIDs[0], channelInformation.ChannelID)
	assert.Equal(suite.T(), "0", channelInformation.Name)

	channels, err := suite.chatRepo.GetAccountChannels(ctx, userID)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), 4, len(channels))

	for idx, channel := range channels {
		assert.Equal(suite.T(), channelIDs[idx], channel)
	}

	anotherUserID := 101
	channelID := channels[0]
	channels, err = suite.chatRepo.GetAccountChannels(ctx, anotherUserID)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), 0, len(channels))

	err = suite.chatRepo.CreateAccountChannels(ctx, anotherUserID, int(channelID))
	assert.Nil(suite.T(), err)
	channels, err = suite.chatRepo.GetAccountChannels(ctx, anotherUserID)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), 1, len(channels))
}

func (suite *TestSuite) TestUserCanCreateSameNameChannels() {
	ctx := context.Background()

	userID := 103

	_, err := suite.chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(suite.T(), err)

	_, err = suite.chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(suite.T(), err)
}

func (suite *TestSuite) TestUserCanNotAddDuplicatedChannels() {
	ctx := context.Background()

	userID := 102

	channelID, err := suite.chatRepo.CreateChannel(ctx, userID, "a")
	assert.Nil(suite.T(), err)

	err = suite.chatRepo.CreateAccountChannels(ctx, userID, int(channelID))
	assert.ErrorIs(suite.T(), err, mysqlKit.ErrDuplicatedKey)
}

func (suite *TestSuite) TestGetAccountFriends() {
	ctx := context.Background()

	userID := 100

	for friendID := 101; friendID <= 104; friendID++ {
		err := suite.chatRepo.CreateAccountFriends(ctx, userID, friendID)
		assert.Nil(suite.T(), err)
	}

	accountFriends, err := suite.chatRepo.GetAccountFriends(ctx, userID)
	assert.Nil(suite.T(), err)

	assert.Equal(suite.T(), 4, len(accountFriends))

	for idx, friend := range accountFriends {
		assert.Equal(suite.T(), int64(101+idx), friend)
	}
}

func (suite *TestSuite) TestUserCanNotAddMyselfToFriend() {
	ctx := context.Background()

	userID := 101

	err := suite.chatRepo.CreateAccountFriends(ctx, userID, userID)
	assert.Equal(suite.T(), err.Error(), "can not create same user to friend")
}

func (suite *TestSuite) TestUserCanNotAddDuplicatedFriends() {
	ctx := context.Background()

	userID := 100

	err := suite.chatRepo.CreateAccountFriends(ctx, userID, 105)
	assert.Nil(suite.T(), err)

	err = suite.chatRepo.CreateAccountFriends(ctx, userID, 105)
	assert.ErrorIs(suite.T(), err, mysqlKit.ErrDuplicatedKey)
}

func (suite *TestSuite) TestUserOnline() {
	ctx := context.Background()

	userID := 100

	userChatInformation, err := suite.chatRepo.GetOrCreateUserChatInformation(ctx, userID)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), domain.OfflineStatus, userChatInformation.Online)

	err = suite.chatRepo.UpdateOnlineStatus(ctx, userID, domain.OnlineStatus)
	assert.Nil(suite.T(), err)

	userChatInformation, err = suite.chatRepo.GetOrCreateUserChatInformation(ctx, userID)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), domain.OnlineStatus, userChatInformation.Online)
}

func (suite *TestSuite) TestGetHistoryMessage() {
	ctx := context.Background()

	for i := 1; i <= 10; i++ {
		_, err := suite.chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertChannelMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertChannelMessage(ctx, 2, 2, "content: d"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessage(ctx, 1, 0, curPage)
			assert.Nil(suite.T(), err)
			if isEnd {
				assert.Equal(suite.T(), 2, len(historyMessage))
			} else {
				assert.Equal(suite.T(), 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(suite.T(), 8, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(suite.T(), domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
			assert.Equal(suite.T(), domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].FriendMessage.Content)
			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].ChannelMessage.Content)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessage(ctx, 1, 2, curPage)
			assert.Nil(suite.T(), err)
			assert.Equal(suite.T(), 3, len(historyMessage))

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(suite.T(), 7, curPage)

		for i := 0; i < 9; i++ {
			assert.Equal(suite.T(), domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
			assert.Equal(suite.T(), domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].FriendMessage.Content)
			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].ChannelMessage.Content)
		}
	}
}
