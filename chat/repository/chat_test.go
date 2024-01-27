package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/superj80820/system-design/domain"
	httpKit "github.com/superj80820/system-design/kit/http"
	kafkaMQKit "github.com/superj80820/system-design/kit/mq/kafka"
	kafkaMQReaderManagerKit "github.com/superj80820/system-design/kit/mq/kafka/reader_manager"
	kafkaMQWriterManagerKit "github.com/superj80820/system-design/kit/mq/kafka/writer_manager"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatSuite struct {
	suite.Suite
	chatRepo *ChatRepo

	mongoDB *mongo.Client

	channelMessageTopic     kafkaMQKit.MQTopic
	userMessageTopic        kafkaMQKit.MQTopic
	userStatusTopic         kafkaMQKit.MQTopic
	friendOnlineStatusTopic kafkaMQKit.MQTopic

	mongodbContainer *mongodb.MongoDBContainer
	kafkaContainer   *kafka.KafkaContainer
	mysqlContainer   *mysql.MySQLContainer

	kafkaTestLogConsumer *TestLogConsumer
}

type TestLogConsumer struct {
	Msgs       []string
	words      string
	wordsCh    chan string
	getWordsCh chan bool
}

func (g *TestLogConsumer) Accept(l testcontainers.Log) {
	fmt.Println("---", string(l.Content))
	if strings.Contains(string(l.Content), g.words) {
		select {
		case g.getWordsCh <- true:
		default:
		}
	}
	g.Msgs = append(g.Msgs, string(l.Content))
}

func CreateTestLogConsumer() *TestLogConsumer {
	testLogConsumer := &TestLogConsumer{
		wordsCh:    make(chan string),
		getWordsCh: make(chan bool),
	}
	go func() {
		for words := range testLogConsumer.wordsCh {
			fmt.Println("get", words)
			testLogConsumer.words = words
		}
	}()

	return testLogConsumer
}

func (suite *ChatSuite) SetupSuite() {
	ctx := context.Background()

	kafkaContainer, err := kafka.RunContainer(
		ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		kafka.WithClusterID("test-cluster"),
	)
	assert.Nil(suite.T(), err)
	suite.kafkaContainer = kafkaContainer
	testLogConsumer := CreateTestLogConsumer()
	suite.kafkaContainer.FollowOutput(testLogConsumer)
	err = suite.kafkaContainer.StartLogProducer(ctx)
	assert.Nil(suite.T(), err)
	suite.kafkaTestLogConsumer = testLogConsumer

	kafkaHost, err := kafkaContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9093") // TODO: why
	if err != nil {
		panic(err)
	}
	brokerAddress := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())

	channelMessageTopicName := "channel-message-topic"
	userMessageTopicName := "user-message-topic"
	userStatusTopicName := "user-status-topic"
	serviceName := "chat-service"

	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, channelMessageTopicName, 0)
	assert.Nil(suite.T(), err)
	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, userMessageTopicName, 0)
	assert.Nil(suite.T(), err)
	_, err = kafkaGo.DialLeader(ctx, "tcp", brokerAddress, userStatusTopicName, 0)
	assert.Nil(suite.T(), err)

	channelMessageTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		channelMessageTopicName,
		kafkaMQKit.ConsumeByPartitionsBindObserver(kafkaMQReaderManagerKit.LastOffset),
		kafkaMQKit.ProduceWay(&kafkaMQWriterManagerKit.Hash{}),
	)
	assert.Nil(suite.T(), err)
	suite.channelMessageTopic = channelMessageTopic

	userMessageTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		userMessageTopicName,
		kafkaMQKit.ConsumeByPartitionsBindObserver(kafkaMQReaderManagerKit.LastOffset),
		kafkaMQKit.ProduceWay(&kafkaMQWriterManagerKit.Hash{}),
	)
	assert.Nil(suite.T(), err)
	suite.userMessageTopic = userMessageTopic

	userStatusTopic, err := kafkaMQKit.CreateMQTopic(
		context.TODO(),
		brokerAddress,
		userStatusTopicName,
		kafkaMQKit.ConsumeByGroupID(serviceName+":user_status", false),
	)
	assert.Nil(suite.T(), err)
	suite.userStatusTopic = userStatusTopic

	friendOnlineStatusTopic, err := kafkaMQKit.CreateMQTopic( // TODO: need?
		context.TODO(),
		brokerAddress,
		userStatusTopicName,
		kafkaMQKit.ConsumeByGroupID(serviceName+":friend_online_status", false),
	)
	suite.friendOnlineStatusTopic = friendOnlineStatusTopic

	fmt.Println("-------------------------aaa")
	testLogConsumer.wordsCh <- "Server started, listening for requests"
	testLogConsumer.wordsCh <- "Server started, listening for requests"
	testLogConsumer.wordsCh <- "Server started, listening for requests"
	<-testLogConsumer.getWordsCh
	fmt.Println("-------------------------get")
}

func (suite *ChatSuite) SetupTest() {
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
		mysql.WithScripts(filepath.Join("./../../instrumenting", "ddl.sql")),
		mysql.WithScripts(filepath.Join("./../../instrumenting", "mock.sql")),
	)
	assert.Nil(suite.T(), err)
	suite.mysqlContainer = mysqlContainer

	mysqlDBHost, err := mysqlContainer.Host(ctx)
	assert.Nil(suite.T(), err)
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	assert.Nil(suite.T(), err)

	mysqlDB, err := ormKit.CreateDB(
		ormKit.UseMySQL(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			)))
	if err != nil {
		panic(err)
	}

	chatRepo, err := CreateChatRepo(
		mongoDB,
		mysqlDB,
		suite.channelMessageTopic,
		suite.userMessageTopic,
		suite.userStatusTopic,
		suite.friendOnlineStatusTopic,
		SetPageSize(3),
	)
	assert.Nil(suite.T(), err)

	suite.chatRepo = chatRepo
}

func (suite *ChatSuite) TearDownTest() {
	ctx := context.Background()

	err := suite.mongoDB.Disconnect(ctx)
	assert.Nil(suite.T(), err)

	// Clean up the container
	if err := suite.mongodbContainer.Terminate(ctx); err != nil {
		panic(err)
	}

	// Clean up the container
	if err := suite.mysqlContainer.Terminate(ctx); err != nil {
		panic(err)
	}
}

func (suite *ChatSuite) TearDownSuite() {
	ctx := context.Background()

	assert.True(suite.T(), suite.channelMessageTopic.Shutdown())
	assert.True(suite.T(), suite.userMessageTopic.Shutdown())
	assert.True(suite.T(), suite.userStatusTopic.Shutdown())
	assert.True(suite.T(), suite.friendOnlineStatusTopic.Shutdown())

	// Clean up the container after
	if err := suite.kafkaContainer.Terminate(ctx); err != nil {
		panic(err)
	}
}

func TestChat(t *testing.T) {
	suite.Run(t, new(ChatSuite))
}

// func (suite *ChatSuite) TestSubscribeAndUnSubscribe() {
// 	ctx := context.Background()

// 	userID := 100
// 	friendID := 101
// 	channelID := 1001
// 	ctx = httpKit.AddRequestID(ctx)

// 	testCase := func(
// 		expectedChannelMessageObserversLen,
// 		expectedAccountMessageObserversLen,
// 		expectedAccountStatusObserversLen,
// 		expectedFriendOnlineStatusObserversLen int,
// 	) {
// 		var channelMessageObserversLen int
// 		suite.chatRepo.channelMessageObservers.Range(func(key string, value *mqReaderManagerKit.Observer) bool {
// 			channelMessageObserversLen++
// 			return true
// 		})
// 		var accountMessageObserversLen int
// 		suite.chatRepo.accountMessageObservers.Range(func(key string, value *mqReaderManagerKit.Observer) bool {
// 			accountMessageObserversLen++
// 			return true
// 		})
// 		var accountStatusObserversLen int
// 		suite.chatRepo.accountStatusObservers.Range(func(key string, value *mqReaderManagerKit.Observer) bool {
// 			accountStatusObserversLen++
// 			return true
// 		})
// 		var friendOnlineStatusObserversLen int
// 		suite.chatRepo.friendOnlineStatusObservers.Range(func(key string, value *mqReaderManagerKit.Observer) bool {
// 			friendOnlineStatusObserversLen++
// 			return true
// 		})

// 		assert.Equal(suite.T(), expectedChannelMessageObserversLen, channelMessageObserversLen)
// 		assert.Equal(suite.T(), expectedAccountMessageObserversLen, accountMessageObserversLen)
// 		assert.Equal(suite.T(), expectedAccountStatusObserversLen, accountStatusObserversLen)
// 		assert.Equal(suite.T(), expectedFriendOnlineStatusObserversLen, friendOnlineStatusObserversLen)
// 	}

// 	for i := 0; i < 1000; i++ {
// 		suite.chatRepo.SubscribeFriendOnlineStatus(ctx, friendID+i, func(sm *domain.StatusMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeChannelMessage(ctx, channelID+i, func(cm *domain.ChannelMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeFriendMessage(ctx, userID+i, func(fm *domain.FriendMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeUserStatus(ctx, userID+i, func(sm *domain.StatusMessage) error {
// 			return nil
// 		})

// 		// test no same subscriber
// 		suite.chatRepo.SubscribeFriendOnlineStatus(ctx, friendID+i, func(sm *domain.StatusMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeChannelMessage(ctx, channelID+i, func(cm *domain.ChannelMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeFriendMessage(ctx, userID+i, func(fm *domain.FriendMessage) error {
// 			return nil
// 		})
// 		suite.chatRepo.SubscribeUserStatus(ctx, userID+i, func(sm *domain.StatusMessage) error {
// 			return nil
// 		})
// 	}
// 	time.Sleep(time.Second * 1) // TODO: wait subscribed

// 	testCase(1000, 1000, 1000, 1000)

// 	for i := 0; i < 10; i++ {
// 		suite.chatRepo.UnSubscribeFriendOnlineStatus(ctx, friendID+i)
// 		suite.chatRepo.UnSubscribeChannelMessage(ctx, channelID+i)
// 		suite.chatRepo.UnSubscribeFriendMessage(ctx, userID+i)
// 	}

// 	testCase(990, 990, 1000, 990)

// 	suite.chatRepo.UnSubscribeAll(ctx)

// 	testCase(0, 0, 0, 0)

// 	// test no panic when unsubscribe not exist user
// 	for i := 0; i < 10; i++ {
// 		suite.chatRepo.UnSubscribeFriendOnlineStatus(ctx, friendID+i)
// 		suite.chatRepo.UnSubscribeChannelMessage(ctx, channelID+i)
// 		suite.chatRepo.UnSubscribeFriendMessage(ctx, userID+i)
// 	}
// }

func (suite *ChatSuite) TestFriendOnlineStatus() {
	ctx := context.Background()

	friendID := 101
	ctx = httpKit.AddRequestID(ctx)

	suite.kafkaTestLogConsumer.wordsCh <- "group chat-service:friend_online_status for generation"
	done := make(chan bool)
	suite.chatRepo.SubscribeFriendOnlineStatus(ctx, friendID, func(sm *domain.StatusMessage) error {
		assert.Equal(suite.T(), domain.OnlineStatusType, sm.StatusType)
		close(done)
		return nil
	})

	<-suite.kafkaTestLogConsumer.getWordsCh
	fmt.Println("-------------------------get2")

	err := suite.chatRepo.SendUserOnlineStatusMessage(ctx, friendID, domain.OnlineStatusType)
	fmt.Println("sended")
	assert.Nil(suite.T(), err)
	select {
	case <-done:
	case <-time.NewTimer(time.Second * 60).C:
		assert.Fail(suite.T(), "get friend status message failed")
	}
}

// func (suite *ChatSuite) TestUserStatus() {
// 	ctx := context.Background()

// 	userID := 100
// 	friendID := 101
// 	channelID := 1001
// 	ctx = httpKit.AddRequestID(ctx)

// 	done := make(chan bool)
// 	recv := make([]*domain.StatusMessage, 0, 3)
// 	suite.chatRepo.SubscribeUserStatus(ctx, userID, func(sm *domain.StatusMessage) error {
// 		recv = append(recv, sm)
// 		if len(recv) == 3 {
// 			close(done)
// 			return nil
// 		}
// 		return nil
// 	})
// 	time.Sleep(time.Second * 1) // TODO: wait subscribed
// 	err := suite.chatRepo.SendUserOnlineStatusMessage(ctx, userID, domain.OnlineStatusType)
// 	assert.Nil(suite.T(), err)
// 	err = suite.chatRepo.SendAddFriendStatusMessage(ctx, userID, friendID)
// 	assert.Nil(suite.T(), err)
// 	err = suite.chatRepo.SendJoinChannelStatusMessage(ctx, userID, channelID)
// 	assert.Nil(suite.T(), err)
// 	select {
// 	case <-done:
// 		assert.Equal(suite.T(), domain.OnlineStatusType, recv[0].StatusType)
// 		assert.Equal(suite.T(), domain.AddFriendStatusType, recv[1].StatusType)
// 		assert.Equal(suite.T(), domain.JoinChannelStatusType, recv[2].StatusType)
// 	case <-time.NewTimer(time.Second * 60).C:
// 		assert.Fail(suite.T(), "get user status message failed")
// 	}
// }

// func (suite *ChatSuite) TestSendFriendMessage() {
// 	ctx := context.Background()

// 	messageContent := "content"
// 	messageID := 1000
// 	userID := 100
// 	friendID := 101
// 	ctx = httpKit.AddRequestID(ctx)

// 	done := make(chan bool)
// 	suite.chatRepo.SubscribeFriendMessage(ctx, userID, func(fm *domain.FriendMessage) error {
// 		assert.Equal(suite.T(), messageContent, fm.Content)
// 		close(done)
// 		return nil
// 	})
// 	time.Sleep(time.Second * 1) // TODO: wait subscribed
// 	err := suite.chatRepo.SendFriendMessage(ctx, friendID, userID, messageID, messageContent)
// 	assert.Nil(suite.T(), err)
// 	select {
// 	case <-done:
// 	case <-time.NewTimer(time.Second * 60).C:
// 		assert.Fail(suite.T(), "get friend message timeout")
// 	}
// }

// func (suite *ChatSuite) TestSendChannelMessage() {
// 	ctx := context.Background()

// 	messageContent := "content"
// 	messageID := 1000
// 	userID := 100
// 	channelID := 101
// 	ctx = httpKit.AddRequestID(ctx)

// 	done := make(chan bool)
// 	suite.chatRepo.SubscribeChannelMessage(ctx, channelID, func(cm *domain.ChannelMessage) error {
// 		assert.Equal(suite.T(), messageContent, cm.Content)
// 		assert.Equal(suite.T(), int64(channelID), cm.ChannelID)
// 		assert.Equal(suite.T(), int64(userID), cm.UserID)
// 		close(done)
// 		return nil
// 	})
// 	time.Sleep(time.Second * 1) // TODO: wait subscribed
// 	err := suite.chatRepo.SendChannelMessage(ctx, userID, channelID, messageID, messageContent)
// 	assert.Nil(suite.T(), err)
// 	select {
// 	case <-done:
// 	case <-time.NewTimer(time.Second * 60).C:
// 		assert.Fail(suite.T(), "get channel message timeout")
// 	}
// }

// func (suite *ChatSuite) TestGetAccountChannels() {
// 	ctx := context.Background()

// 	userID := 100

// 	channelIDs := make([]int64, 4)
// 	for i := 0; i < 4; i++ {
// 		channelID, err := suite.chatRepo.CreateChannel(ctx, userID, strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)

// 		channelIDs[i] = channelID
// 	}

// 	channels, err := suite.chatRepo.GetAccountChannels(ctx, userID)
// 	assert.Nil(suite.T(), err)

// 	assert.Equal(suite.T(), 4, len(channels))

// 	for idx, channel := range channels {
// 		assert.Equal(suite.T(), channelIDs[idx], channel)
// 	}

// 	anotherUserID := 101
// 	channelID := channels[0]
// 	channels, err = suite.chatRepo.GetAccountChannels(ctx, anotherUserID)
// 	assert.Nil(suite.T(), err)

// 	assert.Equal(suite.T(), 0, len(channels))

// 	err = suite.chatRepo.CreateAccountChannels(ctx, anotherUserID, int(channelID))
// 	assert.Nil(suite.T(), err)
// 	channels, err = suite.chatRepo.GetAccountChannels(ctx, anotherUserID)
// 	assert.Nil(suite.T(), err)

// 	assert.Equal(suite.T(), 1, len(channels))
// }

// func (suite *ChatSuite) TestUserCanCreateSameNameChannels() {
// 	ctx := context.Background()

// 	userID := 103

// 	_, err := suite.chatRepo.CreateChannel(ctx, userID, "a")
// 	assert.Nil(suite.T(), err)

// 	_, err = suite.chatRepo.CreateChannel(ctx, userID, "a")
// 	assert.Nil(suite.T(), err)
// }

// func (suite *ChatSuite) TestUserCanNotAddDuplicatedChannels() {
// 	ctx := context.Background()

// 	userID := 102

// 	channelID, err := suite.chatRepo.CreateChannel(ctx, userID, "a")
// 	assert.Nil(suite.T(), err)

// 	err = suite.chatRepo.CreateAccountChannels(ctx, userID, int(channelID))
// 	assert.ErrorIs(suite.T(), err, ormKit.ErrDuplicatedKey)
// }

// func (suite *ChatSuite) TestGetAccountFriends() {
// 	ctx := context.Background()

// 	userID := 100

// 	for friendID := 101; friendID <= 104; friendID++ {
// 		err := suite.chatRepo.CreateAccountFriends(ctx, userID, friendID)
// 		assert.Nil(suite.T(), err)
// 	}

// 	accountFriends, err := suite.chatRepo.GetAccountFriends(ctx, userID)
// 	assert.Nil(suite.T(), err)

// 	assert.Equal(suite.T(), 4, len(accountFriends))

// 	for idx, friend := range accountFriends {
// 		assert.Equal(suite.T(), int64(101+idx), friend)
// 	}
// }

// func (suite *ChatSuite) TestUserCanNotAddMyselfToFriend() {
// 	ctx := context.Background()

// 	userID := 101

// 	err := suite.chatRepo.CreateAccountFriends(ctx, userID, userID)
// 	assert.Equal(suite.T(), err.Error(), "can not create same user to friend")
// }

// func (suite *ChatSuite) TestUserCanNotAddDuplicatedFriends() {
// 	ctx := context.Background()

// 	userID := 100

// 	err := suite.chatRepo.CreateAccountFriends(ctx, userID, 105)
// 	assert.Nil(suite.T(), err)

// 	err = suite.chatRepo.CreateAccountFriends(ctx, userID, 105)
// 	assert.ErrorIs(suite.T(), err, ormKit.ErrDuplicatedKey)
// }

// func (suite *ChatSuite) TestUserOnline() {
// 	ctx := context.Background()

// 	userID := 100

// 	userChatInformation, err := suite.chatRepo.GetOrCreateUserChatInformation(ctx, userID)
// 	assert.Nil(suite.T(), err)
// 	assert.Equal(suite.T(), domain.OfflineStatus, userChatInformation.Online)

// 	err = suite.chatRepo.UpdateOnlineStatus(ctx, userID, domain.OnlineStatus)
// 	assert.Nil(suite.T(), err)

// 	userChatInformation, err = suite.chatRepo.GetOrCreateUserChatInformation(ctx, userID)
// 	assert.Nil(suite.T(), err)
// 	assert.Equal(suite.T(), domain.OnlineStatus, userChatInformation.Online)
// }

// func (suite *ChatSuite) TestGetHistoryMessage() {
// 	ctx := context.Background()

// 	for i := 1; i <= 10; i++ {
// 		_, err := suite.chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertChannelMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertChannelMessage(ctx, 2, 2, "content: d"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.FriendOrChannelMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.FriendOrChannelMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessage(ctx, 1, 0, curPage)
// 			assert.Nil(suite.T(), err)
// 			if isEnd {
// 				assert.Equal(suite.T(), 2, len(historyMessage))
// 			} else {
// 				assert.Equal(suite.T(), 3, len(historyMessage))
// 			}

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 8, curPage)

// 		for i := 0; i < 10; i++ {
// 			assert.Equal(suite.T(), domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
// 			assert.Equal(suite.T(), domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].FriendMessage.Content)
// 			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].ChannelMessage.Content)
// 		}
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.FriendOrChannelMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.FriendOrChannelMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessage(ctx, 1, 2, curPage)
// 			assert.Nil(suite.T(), err)
// 			assert.Equal(suite.T(), 3, len(historyMessage))

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 7, curPage)

// 		for i := 0; i < 9; i++ {
// 			assert.Equal(suite.T(), domain.FriendMessageType, allHistoryMessage[i*2].MessageType)
// 			assert.Equal(suite.T(), domain.ChannelMessageType, allHistoryMessage[i*2+1].MessageType)
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].FriendMessage.Content)
// 			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].ChannelMessage.Content)
// 		}
// 	}
// }

// func (suite *ChatSuite) TestGetHistoryMessageByChannel() {
// 	ctx := context.Background()

// 	for i := 1; i <= 10; i++ {
// 		_, err := suite.chatRepo.InsertChannelMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertChannelMessage(ctx, 2, 1, "content: b"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertChannelMessage(ctx, 1, 2, "content: c"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.ChannelMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.ChannelMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByChannel(ctx, 1, 0, curPage)
// 			assert.Nil(suite.T(), err)
// 			if isEnd {
// 				assert.Equal(suite.T(), 2, len(historyMessage))
// 			} else {
// 				assert.Equal(suite.T(), 3, len(historyMessage))
// 			}

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 8, curPage)

// 		for i := 0; i < 10; i++ {
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].Content)
// 			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].Content)
// 		}
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.ChannelMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.ChannelMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByChannel(ctx, 1, 2, curPage)
// 			assert.Nil(suite.T(), err)
// 			assert.Equal(suite.T(), 3, len(historyMessage))

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 7, curPage)

// 		for i := 0; i < 9; i++ {
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].Content)
// 			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].Content)
// 		}
// 	}
// }

// func (suite *ChatSuite) TestGetHistoryMessageByFriend() {
// 	ctx := context.Background()

// 	for i := 1; i <= 10; i++ {
// 		_, err := suite.chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertFriendMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 		_, err = suite.chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
// 		assert.Nil(suite.T(), err)
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.FriendMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.FriendMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 0, curPage)
// 			assert.Nil(suite.T(), err)
// 			if isEnd {
// 				assert.Equal(suite.T(), 1, len(historyMessage))
// 			} else {
// 				assert.Equal(suite.T(), 3, len(historyMessage))
// 			}

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 5, curPage)

// 		for i := 0; i < 10; i++ {
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i].Content)
// 		}
// 	}

// 	{
// 		var (
// 			isEnd             bool
// 			allHistoryMessage []*domain.FriendMessage
// 		)
// 		curPage := 1
// 		for ; !isEnd; curPage++ {
// 			var (
// 				historyMessage []*domain.FriendMessage
// 				err            error
// 			)
// 			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 2, curPage)
// 			assert.Nil(suite.T(), err)
// 			if isEnd {
// 				assert.Equal(suite.T(), 2, len(historyMessage))
// 			} else {
// 				assert.Equal(suite.T(), 3, len(historyMessage))
// 			}

// 			for _, message := range historyMessage {
// 				allHistoryMessage = append(allHistoryMessage, message)
// 			}
// 		}
// 		assert.Equal(suite.T(), 4, curPage)

// 		for i := 0; i < 8; i++ {
// 			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+3), allHistoryMessage[i].Content)
// 		}
// 	}
// }
