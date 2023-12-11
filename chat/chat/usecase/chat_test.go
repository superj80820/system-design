package usecase

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/superj80820/system-design/chat/chat/usecase/mocks"
	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
)

func TestChat(t *testing.T) {
	suite.Run(t, new(ChatSuite))
}

type ChatSuite struct {
	suite.Suite

	mockChatRepo *mocks.ChatRepository

	userID int

	runChat func()

	in   chan *domain.ChatRequest
	out  chan *domain.ChatResponse
	done chan bool
}

func (chat *ChatSuite) SetupTest() {
	ctx := context.Background()
	chatRepo := new(mocks.ChatRepository)
	chatUseCase := CreateChatUseCase(chatRepo)
	in, out, done := make(chan *domain.ChatRequest), make(chan *domain.ChatResponse), make(chan bool)

	userID := "100"
	userIDInt, err := strconv.Atoi(userID)
	assert.Nil(chat.T(), err)
	ctx = httpKit.AddToken(ctx, userID)

	chatRepo.On("GetOrCreateUserChatInformation", mock.Anything, userIDInt).Return(nil, nil)
	chatRepo.On("UpdateOnlineStatus", mock.Anything, userIDInt, domain.OnlineStatus).Return(nil)
	chatRepo.On("SendUserStatusMessage", mock.Anything, userIDInt, domain.OnlineStatusType).Return(nil)

	chat.mockChatRepo = chatRepo
	chat.userID = userIDInt
	chat.in, chat.out, chat.done = in, out, done
	chat.runChat = func() {
		go chatUseCase.Chat(ctx, endpoint.CreateServerStream[domain.ChatRequest, domain.ChatResponse](in, out, done))
	}
}

func (chat *ChatSuite) TestRecvReq() {
	friendID := 101
	channelID := 1001
	messageID := 10001
	message := "message"

	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Return()
	chat.mockChatRepo.
		On("GetAccountChannels", mock.Anything, chat.userID).
		Return([]int64{}, nil)
	chat.mockChatRepo.
		On("GetAccountFriends", mock.Anything, chat.userID).
		Return([]int64{}, nil)
	chat.mockChatRepo.
		On("GetHistoryMessage", mock.Anything, chat.userID, 0, 1).
		Return([]*domain.FriendOrChannelMessage{}, true, nil).
		Once()

	chat.runChat()

	// consume all response
	<-chat.out
	<-chat.out
	<-chat.out

	chat.mockChatRepo.
		On("InsertFriendMessage", mock.Anything, int64(chat.userID), int64(friendID), message).
		Return(int64(messageID), nil)
	chat.mockChatRepo.
		On("SendFriendMessage", mock.Anything, chat.userID, friendID, messageID, message).
		Return(nil)
	chat.in <- &domain.ChatRequest{
		Action: domain.SendMessageToFriend,
		SendFriendReq: &domain.SendFriendReq{
			FriendID: int64(friendID),
			Message:  message,
		},
	}

	chat.mockChatRepo.
		On("InsertChannelMessage", mock.Anything, int64(chat.userID), int64(channelID), message).
		Return(int64(messageID), nil)
	chat.mockChatRepo.
		On("SendChannelMessage", mock.Anything, chat.userID, channelID, messageID, message).
		Return(nil)
	chat.in <- &domain.ChatRequest{
		Action: domain.SendMessageToChannel,
		SendChannelReq: &domain.SendChannelReq{
			ChannelID: int64(channelID),
			Message:   message,
		},
	}

	chat.mockChatRepo.
		On("GetHistoryMessageByFriend", mock.Anything, chat.userID, friendID, 0, 1).
		Return([]*domain.FriendMessage{
			{
				MessageID: int64(messageID),
			},
		}, true, nil)
	chat.in <- &domain.ChatRequest{
		Action: domain.GetFriendHistoryMessage,
		GetFriendHistoryMessageReq: &domain.GetFriendHistoryMessageReq{
			FriendID:        friendID,
			CurMaxMessageID: 0,
			Page:            1,
		},
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.FriendMessageHistoryResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), int64(messageID), chatResponse.FriendMessageHistory.HistoryMessage[0].MessageID)
	assert.Equal(chat.T(), true, chatResponse.FriendMessageHistory.IsEnd)

	chat.mockChatRepo.
		On("GetHistoryMessageByChannel", mock.Anything, channelID, 0, 1).
		Return([]*domain.ChannelMessage{
			{
				MessageID: int64(messageID),
			},
		}, true, nil)
	chat.in <- &domain.ChatRequest{
		Action: domain.GetChannelHistoryMessage,
		GetChannelHistoryMessageReq: &domain.GetChannelHistoryMessageReq{
			ChannelID:       channelID,
			CurMaxMessageID: 0,
			Page:            1,
		},
	}
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.ChannelMessageHistoryResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), int64(messageID), chatResponse.ChannelMessageHistory.HistoryMessage[0].MessageID)
	assert.Equal(chat.T(), true, chatResponse.ChannelMessageHistory.IsEnd)

	chat.mockChatRepo.
		On("GetHistoryMessage", mock.Anything, chat.userID, 0, 1).
		Return([]*domain.FriendOrChannelMessage{
			{
				MessageType: domain.ChannelMessageType,
				ChannelMessage: &domain.ChannelMessage{
					MessageID: int64(messageID),
				},
			},
		}, true, nil)
	chat.in <- &domain.ChatRequest{
		Action: domain.GetHistoryMessage,
		GetHistoryMessageReq: &domain.GetHistoryMessageReq{
			CurMaxMessageID: 0,
			Page:            1,
		},
	}
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOrChannelMessageHistoryResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.ChannelMessageType, chatResponse.FriendOrChannelMessageHistory.HistoryMessage[0].MessageType)
	assert.Equal(chat.T(), int64(messageID), chatResponse.FriendOrChannelMessageHistory.HistoryMessage[0].ChannelMessage.MessageID)
	assert.Equal(chat.T(), true, chatResponse.FriendOrChannelMessageHistory.IsEnd)
}

func (chat *ChatSuite) TestGetHistory() {
	userChannels := []int64{1001}
	userFriends := []int64{101}
	historyMessage := []*domain.FriendOrChannelMessage{
		{
			MessageType: domain.ChannelMessageType,
			ChannelMessage: &domain.ChannelMessage{
				MessageID: 10000,
				ChannelID: 1001,
				Content:   "content:a",
				UserID:    101,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			MessageType: domain.ChannelMessageType,
			ChannelMessage: &domain.ChannelMessage{
				MessageID: 10001,
				ChannelID: 1001,
				Content:   "content:b",
				UserID:    102,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			MessageType: domain.FriendMessageType,
			FriendMessage: &domain.FriendMessage{
				MessageID: 10002,
				Content:   "content:c",
				FriendID:  103,
				UserID:    int64(chat.userID),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Return()
	chat.mockChatRepo.
		On("GetAccountChannels", mock.Anything, chat.userID).
		Return(userChannels, nil)
	chat.mockChatRepo.
		On("GetAccountFriends", mock.Anything, chat.userID).
		Return(userFriends, nil)
	chat.mockChatRepo.
		On("GetHistoryMessage", mock.Anything, chat.userID, 0, 1).
		Return(historyMessage, true, nil)
	for _, channel := range userChannels {
		chat.mockChatRepo.
			On("SubscribeChannelMessage", mock.Anything, int(channel), mock.Anything).
			Run(func(args mock.Arguments) {
				notify := args.Get(2).(func(*domain.ChannelMessage) error)

				notify(&domain.ChannelMessage{
					ChannelID: channel,
				})
			})
	}
	for _, friend := range userFriends {
		chat.mockChatRepo.
			On("SubscribeFriendMessage", mock.Anything, chat.userID, int(friend), mock.Anything).
			Run(func(args mock.Arguments) {
				notify := args.Get(3).(func(*domain.FriendMessage) error)

				notify(&domain.FriendMessage{
					UserID: friend,
				})
			})
		chat.mockChatRepo.
			On("SubscribeFriendOnlineStatus", mock.Anything, int(friend), mock.Anything).
			Run(func(args mock.Arguments) {
				notify := args.Get(2).(func(*domain.StatusMessage) error)

				notify(&domain.StatusMessage{
					StatusType: domain.OnlineStatusType,
					UserID:     int(friend),
				})
				notify(&domain.StatusMessage{
					StatusType: domain.OfflineStatusType,
					UserID:     int(friend),
				})
			})
	}

	chat.runChat()

	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.UserChannelsResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), userChannels, chatResponse.UserChannels)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.UserFriendsResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), userFriends, chatResponse.UserFriends)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOrChannelMessageHistoryResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), historyMessage, chatResponse.FriendOrChannelMessageHistory.HistoryMessage)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.ChannelResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), userChannels[0], chatResponse.ChannelMessage.ChannelID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), userFriends[0], chatResponse.FriendMessage.FriendID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOnlineStatusResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.OnlineStatus, chatResponse.FriendOnlineStatus.OnlineStatus)
	assert.Equal(chat.T(), userFriends[0], chatResponse.FriendOnlineStatus.FriendID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOnlineStatusResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.OfflineStatus, chatResponse.FriendOnlineStatus.OnlineStatus)
	assert.Equal(chat.T(), userFriends[0], chatResponse.FriendOnlineStatus.FriendID)
}

func (chat *ChatSuite) TestGetFriendOnline() {
	friendID := 101

	statusMessageCh := make(chan *domain.StatusMessage)
	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Run(func(args mock.Arguments) {
			notify := args.Get(2).(func(*domain.StatusMessage) error)

			for statusMessage := range statusMessageCh {
				assert.Nil(chat.T(), notify(statusMessage))
			}
		})

	chat.runChat()

	statusMessageCh <- &domain.StatusMessage{
		StatusType: domain.OnlineStatusType,
		UserID:     friendID,
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.StatusMessageResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.OnlineStatusType, chatResponse.StatusMessage.StatusType)
	assert.Equal(chat.T(), friendID, chatResponse.StatusMessage.UserID)
}

func (chat *ChatSuite) TestAddFriend() {
	friendID := 101
	messageID := 10000
	content := "content"

	statusMessageCh := make(chan *domain.StatusMessage)
	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Run(func(args mock.Arguments) {
			notify := args.Get(2).(func(*domain.StatusMessage) error)

			for statusMessage := range statusMessageCh {
				assert.Nil(chat.T(), notify(statusMessage))
			}
		})

	wg := new(sync.WaitGroup)
	wg.Add(2)
	chat.mockChatRepo.
		On("SubscribeFriendMessage", mock.Anything, chat.userID, friendID, mock.Anything).
		Run(func(args mock.Arguments) {
			defer wg.Done()

			notify := args.Get(3).(func(*domain.FriendMessage) error)
			notify(&domain.FriendMessage{
				MessageID: int64(messageID),
				Content:   content,
				FriendID:  int64(chat.userID),
				UserID:    int64(friendID),
			})
		}).
		Once()
	chat.mockChatRepo.
		On("SubscribeFriendOnlineStatus", mock.Anything, friendID, mock.Anything).
		Run(func(args mock.Arguments) {
			defer wg.Done()

			notify := args.Get(2).(func(*domain.StatusMessage) error)
			notify(&domain.StatusMessage{
				StatusType: domain.OnlineStatusType,
				UserID:     friendID,
			})
			notify(&domain.StatusMessage{
				StatusType: domain.OfflineStatusType,
				UserID:     friendID,
			})
		}).
		Once()

	chat.runChat()

	statusMessageCh <- &domain.StatusMessage{
		StatusType: domain.AddFriendStatusType,
		UserID:     chat.userID,
		AddFriendStatus: &domain.AddFriendStatus{
			FriendID: friendID,
		},
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.StatusMessageResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.AddFriendStatusType, chatResponse.StatusMessage.StatusType)
	assert.Equal(chat.T(), chat.userID, chatResponse.StatusMessage.UserID)
	assert.Equal(chat.T(), friendID, chatResponse.StatusMessage.AddFriendStatus.FriendID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), int64(messageID), chatResponse.FriendMessage.MessageID)
	assert.Equal(chat.T(), content, chatResponse.FriendMessage.Content)
	assert.Equal(chat.T(), int64(friendID), chatResponse.FriendMessage.FriendID)
	assert.Equal(chat.T(), int64(chat.userID), chatResponse.FriendMessage.UserID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOnlineStatusResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.OnlineStatus, chatResponse.FriendOnlineStatus.OnlineStatus)
	assert.Equal(chat.T(), int64(friendID), chatResponse.FriendOnlineStatus.FriendID)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.FriendOnlineStatusResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.OfflineStatus, chatResponse.FriendOnlineStatus.OnlineStatus)
	assert.Equal(chat.T(), int64(friendID), chatResponse.FriendOnlineStatus.FriendID)
	wg.Wait()
}

func (chat *ChatSuite) TestRemoveFriend() {
	friendID := 101

	statusMessageCh := make(chan *domain.StatusMessage)
	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Run(func(args mock.Arguments) {
			notify := args.Get(2).(func(*domain.StatusMessage) error)

			for statusMessage := range statusMessageCh {
				assert.Nil(chat.T(), notify(statusMessage))
			}
		})

	wg := new(sync.WaitGroup)
	wg.Add(2)
	chat.mockChatRepo.
		On("UnSubscribeFriendMessage", mock.Anything, friendID).
		Run(func(args mock.Arguments) {
			wg.Done()
		}).
		Once()
	chat.mockChatRepo.
		On("UnSubscribeFriendOnlineStatus", mock.Anything, friendID).
		Run(func(args mock.Arguments) {
			wg.Done()
		}).
		Once()

	chat.runChat()

	statusMessageCh <- &domain.StatusMessage{
		StatusType: domain.RemoveFriendStatusType,
		UserID:     chat.userID,
		RemoveFriendStatus: &domain.RemoveFriendStatus{
			FriendID: friendID,
		},
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.StatusMessageResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.RemoveFriendStatusType, chatResponse.StatusMessage.StatusType)
	assert.Equal(chat.T(), chat.userID, chatResponse.StatusMessage.UserID)
	assert.Equal(chat.T(), friendID, chatResponse.StatusMessage.RemoveFriendStatus.FriendID)
	wg.Wait()
}

func (chat *ChatSuite) TestAddChannel() {
	channelID := 1000
	messageID := 10000
	content := "content"
	createdAt := time.Now()
	updatedAt := time.Now()

	statusMessageCh := make(chan *domain.StatusMessage)
	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Run(func(args mock.Arguments) {
			notify := args.Get(2).(func(*domain.StatusMessage) error)

			for statusMessage := range statusMessageCh {
				assert.Nil(chat.T(), notify(statusMessage))
			}
		})

	wg := new(sync.WaitGroup)
	wg.Add(1)
	chat.mockChatRepo.
		On("SubscribeChannelMessage", mock.Anything, channelID, mock.Anything).
		Run(func(args mock.Arguments) {
			defer wg.Done()

			notify := args.Get(2).(func(*domain.ChannelMessage) error)

			notify(&domain.ChannelMessage{
				MessageID: int64(messageID),
				ChannelID: int64(channelID),
				Content:   content,
				UserID:    int64(chat.userID),
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			})
		}).
		Once()

	chat.runChat()

	statusMessageCh <- &domain.StatusMessage{
		StatusType: domain.AddChannelStatusType,
		UserID:     chat.userID,
		AddChannelStatus: &domain.AddChannelStatus{
			ChannelID: channelID,
		},
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.StatusMessageResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.AddChannelStatusType, chatResponse.StatusMessage.StatusType)
	chatResponse = <-chat.out
	assert.Equal(chat.T(), domain.ChannelResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), int64(messageID), chatResponse.ChannelMessage.MessageID)
	assert.Equal(chat.T(), int64(channelID), chatResponse.ChannelMessage.ChannelID)
	assert.Equal(chat.T(), content, chatResponse.ChannelMessage.Content)
	assert.Equal(chat.T(), int64(chat.userID), chatResponse.ChannelMessage.UserID)
	wg.Wait()
}

func (chat *ChatSuite) TestRemoveChannel() {
	channelID := 1000

	statusMessageCh := make(chan *domain.StatusMessage)
	chat.mockChatRepo.
		On("SubscribeUserStatus", mock.Anything, chat.userID, mock.Anything).
		Run(func(args mock.Arguments) {
			notify := args.Get(2).(func(*domain.StatusMessage) error)

			for statusMessage := range statusMessageCh {
				assert.Nil(chat.T(), notify(statusMessage))
			}
		})

	wg := new(sync.WaitGroup)
	wg.Add(1)
	chat.mockChatRepo.
		On("UnSubscribeFriendMessage", mock.Anything, channelID).
		Run(func(args mock.Arguments) {
			wg.Done()
		}).
		Once()

	chat.runChat()

	statusMessageCh <- &domain.StatusMessage{
		StatusType: domain.RemoveChannelStatusType,
		UserID:     chat.userID,
		RemoveChannelStatus: &domain.RemoveChannelStatus{
			ChannelID: channelID,
		},
	}
	chatResponse := <-chat.out
	assert.Equal(chat.T(), domain.StatusMessageResponseMessageType, chatResponse.MessageType)
	assert.Equal(chat.T(), domain.RemoveChannelStatusType, chatResponse.StatusMessage.StatusType)
	assert.Equal(chat.T(), chat.userID, chatResponse.StatusMessage.UserID)
	assert.Equal(chat.T(), channelID, chatResponse.StatusMessage.RemoveChannelStatus.ChannelID)
	wg.Wait()
}
