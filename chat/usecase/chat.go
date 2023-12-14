package usecase

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type ChatUseCase struct {
	logger   *loggerKit.Logger
	chatRepo domain.ChatRepository
}

func CreateChatUseCase(chatRepo domain.ChatRepository, logger *loggerKit.Logger) *ChatUseCase {
	return &ChatUseCase{
		logger:   logger,
		chatRepo: chatRepo,
	}
}

var _ domain.ChatService = (*ChatUseCase)(nil)

func (chat *ChatUseCase) Chat(ctx context.Context, userID int, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) error {
	logger := chat.logger.WithMetadata(ctx)

	_, err := chat.chatRepo.GetOrCreateUserChatInformation(ctx, userID)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("create user chat information failed. user id: %d", userID))
	}

	if err := chat.chatRepo.UpdateOnlineStatus(ctx, userID, domain.OnlineStatus); err != nil {
		return errors.Wrap(err, "update user online status failed")
	}
	chat.chatRepo.SendUserOnlineStatusMessage(ctx, userID, domain.OnlineStatusType)
	chat.chatRepo.SubscribeUserStatus(ctx, userID, func(statusMessage *domain.StatusMessage) error {
		stream.Send(&domain.ChatResponse{
			MessageType:   domain.StatusMessageResponseMessageType,
			StatusMessage: statusMessage,
		})

		switch statusMessage.StatusType {
		case domain.AddFriendStatusType:
			chat.chatRepo.SubscribeFriendOnlineStatus(
				ctx,
				statusMessage.AddFriendStatus.FriendID,
				func(sm *domain.StatusMessage) error {
					switch sm.StatusType {
					case domain.OnlineStatusType:
						stream.Send(&domain.ChatResponse{
							MessageType: domain.FriendOnlineStatusResponseMessageType,
							FriendOnlineStatus: &domain.FriendOnlineStatus{
								OnlineStatus: domain.OnlineStatus,
								FriendID:     int64(sm.UserID),
							},
						})
					case domain.OfflineStatusType:
						stream.Send(&domain.ChatResponse{
							MessageType: domain.FriendOnlineStatusResponseMessageType,
							FriendOnlineStatus: &domain.FriendOnlineStatus{
								OnlineStatus: domain.OfflineStatus,
								FriendID:     int64(sm.UserID),
							},
						})
					}
					return nil
				})
		case domain.RemoveFriendStatusType:
			chat.chatRepo.UnSubscribeFriendOnlineStatus(ctx, statusMessage.RemoveFriendStatus.FriendID)
		case domain.JoinChannelStatusType:
			chat.chatRepo.SubscribeChannelMessage(ctx, statusMessage.JoinChannelStatus.ChannelID, func(cm *domain.ChannelMessage) error {
				stream.Send(&domain.ChatResponse{
					MessageType: domain.ChannelResponseMessageType,
					ChannelMessage: &domain.ChannelMessage{
						MessageID: cm.MessageID,
						ChannelID: cm.ChannelID,
						Content:   cm.Content,
						UserID:    cm.UserID,
					},
				})
				return nil
			})
		case domain.LeaveChannelStatusType:
			chat.chatRepo.UnSubscribeChannelMessage(ctx, statusMessage.LeaveChannelStatus.ChannelID)
		}
		return nil
	})
	chat.chatRepo.SubscribeFriendMessage(
		ctx,
		userID,
		func(fm *domain.FriendMessage) error {
			stream.Send(&domain.ChatResponse{
				MessageType: domain.FriendResponseMessageType,
				FriendMessage: &domain.FriendMessage{
					UserID:    fm.FriendID,
					MessageID: fm.MessageID,
					Content:   fm.Content,
					FriendID:  fm.UserID,
				},
			})
			return nil
		})

	accountChannels, err := chat.chatRepo.GetAccountChannels(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "get account channels failed")
	}
	accountChannelsRes := domain.UserChannels(accountChannels)
	stream.Send(&domain.ChatResponse{
		MessageType:  domain.UserChannelsResponseMessageType,
		UserChannels: &accountChannelsRes,
	})

	accountFriends, err := chat.chatRepo.GetAccountFriends(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "get account friends failed")
	}
	accountFriendsRes := domain.UserFriends(accountFriends)
	stream.Send(&domain.ChatResponse{
		MessageType: domain.UserFriendsResponseMessageType,
		UserFriends: &accountFriendsRes,
	})
	historyMessage, isEnd, err := chat.chatRepo.GetHistoryMessage(ctx, userID, 0, 1) // TODO: number offset, page
	if err != nil {
		return errors.Wrap(err, "get history message failed")
	}
	stream.Send(&domain.ChatResponse{
		MessageType: domain.FriendOrChannelMessageHistoryResponseMessageType,
		FriendOrChannelMessageHistory: &domain.FriendOrChannelMessageHistory{
			HistoryMessage: historyMessage,
			IsEnd:          isEnd,
		},
	})

	for _, accountChannel := range accountChannels {
		accountChannelInt := int(accountChannel) //TODO: think overflow

		chat.chatRepo.SubscribeChannelMessage(
			ctx,
			accountChannelInt,
			func(cm *domain.ChannelMessage) error {
				stream.Send(&domain.ChatResponse{
					MessageType: domain.ChannelResponseMessageType,
					ChannelMessage: &domain.ChannelMessage{
						MessageID: cm.MessageID,
						ChannelID: cm.ChannelID,
						Content:   cm.Content,
						UserID:    cm.UserID,
					},
				})
				return nil
			})
	}
	for _, accountFriend := range accountFriends {
		accountFriendInt := int(accountFriend) //TODO: think overflow
		chat.chatRepo.SubscribeFriendOnlineStatus(
			ctx,
			accountFriendInt,
			func(sm *domain.StatusMessage) error {
				switch sm.StatusType {
				case domain.OnlineStatusType:
					stream.Send(&domain.ChatResponse{
						MessageType: domain.FriendOnlineStatusResponseMessageType,
						FriendOnlineStatus: &domain.FriendOnlineStatus{
							OnlineStatus: domain.OnlineStatus,
							FriendID:     int64(sm.UserID),
						},
					})
				case domain.OfflineStatusType:
					stream.Send(&domain.ChatResponse{
						MessageType: domain.FriendOnlineStatusResponseMessageType,
						FriendOnlineStatus: &domain.FriendOnlineStatus{
							OnlineStatus: domain.OfflineStatus,
							FriendID:     int64(sm.UserID),
						},
					})
				}
				return nil
			})
	}

	defer func() {
		if err := chat.chatRepo.UpdateOnlineStatus(ctx, userID, domain.OnlineStatus); err != nil {
			logger.Error(fmt.Sprintf("%+v", errors.Wrap(err, "update user online status failed"))) // TODO
			return
		}
		if err := chat.chatRepo.SendUserOnlineStatusMessage(ctx, userID, domain.OfflineStatusType); err != nil {
			logger.Error(fmt.Sprintf("%+v", errors.Wrap(err, "send user online status message"))) // TODO
			return
		}
	}()

	for {
		req, err := stream.Recv()
		if err != nil {
			return errors.Wrap(err, "receive input failed")
		}
		switch req.Action {
		case domain.SendMessageToFriend:
			messageID, err := chat.chatRepo.InsertFriendMessage(ctx, int64(userID), req.SendFriendReq.FriendID, req.SendFriendReq.Message)
			if err != nil {
				return errors.Wrap(err, "insert friend message failed")
			}
			fmt.Println("get mee", req.SendFriendReq.Message)
			if err := chat.chatRepo.SendFriendMessage(ctx, userID, int(req.SendFriendReq.FriendID), int(messageID), req.SendFriendReq.Message); err != nil { // TODO: is int64 to int safe? {
				return errors.Wrap(err, "send friend message failed")
			}
		case domain.SendMessageToChannel:
			messageID, err := chat.chatRepo.InsertChannelMessage(ctx, int64(userID), req.SendChannelReq.ChannelID, req.SendChannelReq.Message)
			if err != nil {
				return errors.Wrap(err, "insert channel message failed")
			}
			if err := chat.chatRepo.SendChannelMessage(ctx, userID, int(req.SendChannelReq.ChannelID), int(messageID), req.SendChannelReq.Message); err != nil { // TODO: is int64 to int safe?
				return errors.Wrap(err, "send channel message failed")
			}
		case domain.GetFriendHistoryMessage:
			var friendHistoryMessage []*domain.FriendMessage
			friendHistoryMessage, isEnd, err = chat.chatRepo.GetHistoryMessageByFriend( // TODO: curMaxMessageID?
				ctx,
				userID,
				req.GetFriendHistoryMessageReq.FriendID,
				req.GetFriendHistoryMessageReq.CurMaxMessageID,
				req.GetFriendHistoryMessageReq.Page,
			)
			if err != nil {
				return errors.Wrap(err, "get friend message failed")
			}
			stream.Send(&domain.ChatResponse{
				MessageType: domain.FriendMessageHistoryResponseMessageType,
				FriendMessageHistory: &domain.FriendMessageHistory{
					HistoryMessage: friendHistoryMessage,
					IsEnd:          isEnd,
				},
			})
		case domain.GetChannelHistoryMessage:
			var channelHistoryMessage []*domain.ChannelMessage
			channelHistoryMessage, isEnd, err = chat.chatRepo.GetHistoryMessageByChannel(
				ctx,
				req.GetChannelHistoryMessageReq.ChannelID,
				req.GetChannelHistoryMessageReq.CurMaxMessageID,
				req.GetChannelHistoryMessageReq.Page,
			)
			if err != nil {
				return errors.Wrap(err, "get channel message failed")
			}
			stream.Send(&domain.ChatResponse{
				MessageType: domain.ChannelMessageHistoryResponseMessageType,
				ChannelMessageHistory: &domain.ChannelMessageHistory{
					HistoryMessage: channelHistoryMessage,
					IsEnd:          isEnd,
				},
			})
		case domain.GetHistoryMessage:
			historyMessage, isEnd, err = chat.chatRepo.GetHistoryMessage( // TODO: number offset, page
				ctx,
				userID,
				req.GetHistoryMessageReq.CurMaxMessageID,
				req.GetHistoryMessageReq.Page,
			)
			fmt.Println(historyMessage, "asdifjaisdjfiasdf")
			if err != nil {
				return errors.Wrap(err, "get history message failed")
			}
			stream.Send(&domain.ChatResponse{
				MessageType: domain.FriendOrChannelMessageHistoryResponseMessageType,
				FriendOrChannelMessageHistory: &domain.FriendOrChannelMessageHistory{
					HistoryMessage: historyMessage,
					IsEnd:          isEnd, // TODO: should use isEnd?
				},
			})
		}
	}

}

func (chat *ChatUseCase) AddFriend(ctx context.Context, userID int, friendID int) error {
	if err := chat.chatRepo.CreateAccountFriends(ctx, userID, friendID); err != nil {
		return errors.Wrap(err, "add friend failed")
	}
	if err := chat.chatRepo.SendAddFriendStatusMessage(ctx, userID, friendID); err != nil {
		return errors.Wrap(err, "send add friend message failed")
	}
	return nil
}

func (chat *ChatUseCase) CreateChannel(ctx context.Context, userID int, channelName string) (int64, error) {
	channelID, err := chat.chatRepo.CreateChannel(ctx, userID, channelName)
	if err != nil {
		return 0, errors.Wrap(err, "create channel failed")
	}
	return channelID, nil
}

func (chat *ChatUseCase) JoinChannel(ctx context.Context, userID int, channelID int) error {
	if err := chat.chatRepo.CreateAccountChannels(ctx, userID, channelID); err != nil {
		return errors.Wrap(err, "join channel failed")
	}
	if err := chat.chatRepo.SendJoinChannelStatusMessage(ctx, userID, int(channelID)); err != nil {
		return errors.Wrap(err, "send join friend message failed")
	}
	return nil
}

// api
// * join channel -> db & status topic
// * add friend -> db & status topic

// user struct
// * user in channels <- db & my status
// * user friends <- db & my status

// channel message topic(partition)
// * <- channel message
// * -> send message
// user message topic(partition)
// * <- user message
// * -> send message
// user status topic(partition)
// * <- user status
// * <- my status
// * -> produce status

// TODO: api to get user channels and friends
