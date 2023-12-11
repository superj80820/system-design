package usecase

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
)

type ChatUseCase struct {
	chatRepo domain.ChatRepository
}

func CreateChatUseCase(chatRepo domain.ChatRepository) *ChatUseCase {
	return &ChatUseCase{
		chatRepo: chatRepo,
	}
}

func (chat *ChatUseCase) Chat(ctx context.Context, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) error {
	token := httpKit.GetToken(ctx)
	accountID, err := strconv.Atoi(token) // TODO
	if err != nil {
		return errors.Wrap(err, "get user id failed")
	}

	// TODO: think need tx?
	_, err = chat.chatRepo.GetOrCreateUserChatInformation(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "create user chat information failed")
	}

	if err := chat.chatRepo.UpdateOnlineStatus(ctx, accountID, domain.OnlineStatus); err != nil {
		return errors.Wrap(err, "update user online status failed")
	}
	chat.chatRepo.SendUserStatusMessage(ctx, accountID, domain.OnlineStatusType)
	chat.chatRepo.SubscribeUserStatus(ctx, accountID, func(statusMessage *domain.StatusMessage) error {
		stream.Send(&domain.ChatResponse{
			MessageType:   domain.StatusMessageResponseMessageType,
			StatusMessage: statusMessage,
		})

		switch statusMessage.StatusType {
		case domain.AddFriendStatusType:
			chat.chatRepo.SubscribeFriendMessage(
				ctx,
				accountID,
				statusMessage.AddFriendStatus.FriendID,
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
				},
			)
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
			chat.chatRepo.UnSubscribeFriendMessage(ctx, statusMessage.RemoveFriendStatus.FriendID)
			chat.chatRepo.UnSubscribeFriendOnlineStatus(ctx, statusMessage.RemoveFriendStatus.FriendID)
		case domain.AddChannelStatusType:
			chat.chatRepo.SubscribeChannelMessage(ctx, statusMessage.AddChannelStatus.ChannelID, func(cm *domain.ChannelMessage) error {
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
		case domain.RemoveChannelStatusType:
			chat.chatRepo.UnSubscribeFriendMessage(ctx, statusMessage.RemoveChannelStatus.ChannelID)
		}
		return nil
	})

	accountChannels, err := chat.chatRepo.GetAccountChannels(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "get account channels failed")
	}
	stream.Send(&domain.ChatResponse{
		MessageType:  domain.UserChannelsResponseMessageType,
		UserChannels: accountChannels,
	})

	accountFriends, err := chat.chatRepo.GetAccountFriends(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "get account friends failed")
	}
	stream.Send(&domain.ChatResponse{
		MessageType: domain.UserFriendsResponseMessageType,
		UserFriends: accountFriends,
	})
	historyMessage, isEnd, err := chat.chatRepo.GetHistoryMessage(ctx, accountID, 0, 1) // TODO: number offset, page
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

		chat.chatRepo.SubscribeFriendMessage(
			ctx,
			accountID,
			accountFriendInt,
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

	for {
		req, err := stream.Recv()
		if err != nil {
			if err := chat.chatRepo.UpdateOnlineStatus(ctx, accountID, domain.OnlineStatus); err != nil { // TODO: defer?
				return errors.Wrap(err, "update user online status failed")
			}
			chat.chatRepo.SendUserStatusMessage(ctx, accountID, domain.OfflineStatusType)
			return errors.Wrap(err, "receive input failed")
		}
		switch req.Action {
		case domain.SendMessageToFriend:
			// TODO: tx?
			messageID, err := chat.chatRepo.InsertFriendMessage(ctx, int64(accountID), req.SendFriendReq.FriendID, req.SendFriendReq.Message)
			if err != nil {
				return errors.Wrap(err, "insert friend message failed")
			}
			if err := chat.chatRepo.SendFriendMessage(ctx, accountID, int(req.SendFriendReq.FriendID), int(messageID), req.SendFriendReq.Message); err != nil { // TODO: is int64 to int safe? {
				return errors.Wrap(err, "send friend message failed")
			}
		case domain.SendMessageToChannel:
			// TODO: tx?
			messageID, err := chat.chatRepo.InsertChannelMessage(ctx, int64(accountID), req.SendChannelReq.ChannelID, req.SendChannelReq.Message)
			if err != nil {
				return errors.Wrap(err, "insert channel message failed")
			}
			if err := chat.chatRepo.SendChannelMessage(ctx, accountID, int(req.SendChannelReq.ChannelID), int(messageID), req.SendChannelReq.Message); err != nil { // TODO: is int64 to int safe?
				return errors.Wrap(err, "send channel message failed")
			}
		case domain.GetFriendHistoryMessage:
			var friendHistoryMessage []*domain.FriendMessage
			friendHistoryMessage, isEnd, err = chat.chatRepo.GetHistoryMessageByFriend( // TODO: curMaxMessageID?
				ctx,
				accountID,
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
				accountID,
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
