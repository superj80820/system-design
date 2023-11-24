package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/chat/chat/repository"
	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
	mqKit "github.com/superj80820/system-design/kit/mq"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type ChatUseCase struct {
	chatRepo            *repository.ChatRepo
	channelMessageTopic *mqKit.MQTopic
	userMessageTopic    *mqKit.MQTopic
	userStatusTopic     *mqKit.MQTopic
}

type ChannelMessage struct {
	*domain.ChannelMessage
}

func (c ChannelMessage) GetKey() string {
	return strconv.Itoa(c.ChannelID)
}

func (c ChannelMessage) Marshal() ([]byte, error) {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return jsonData, nil
}

func MakeChatUseCase(chatRepo *repository.ChatRepo, channelMessageTopic, userMessageTopic, userStatusTopic *mqKit.MQTopic) *ChatUseCase {
	return &ChatUseCase{
		chatRepo:            chatRepo,
		channelMessageTopic: channelMessageTopic,
		userMessageTopic:    userMessageTopic,
		userStatusTopic:     userStatusTopic,
	}
}

// TODO: api to get user channels and friends

func (chat *ChatUseCase) Chat(ctx context.Context, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) error {
	token := httpKit.GetToken(ctx)
	userID, err := strconv.Atoi(token) // TODO
	if err != nil {
		return errors.Wrap(err, "get user id failed")
	}

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return errors.Wrap(err, "get unique id failed")
	}

	// TODO: produce my status
	// TODO: subscribe friend status
	// TODO: subscribe friend message for me
	// TODO: subscribe channel message for me

	var (
		joinObservers []*mqKit.Observer
		channelName   string
	)
	for {
		req, ok := stream.RecvFromIn()
		if !ok {
			fmt.Println("break")
			break
		}
		switch req.Action {
		case domain.SendUser:
			// TODO: subscribe user status
		case domain.SendChannel:
			channelID := chat.chatRepo.GetChannelID(req.SendChannelReq.ChannelName)
			channelMessage := domain.ChannelMessage{
				MessageID: int(uniqueIDGenerate.Generate().GetInt64()),
				ChannelID: channelID,
				Content:   req.SendChannelReq.Message,
				UserID:    userID,
			}

			chat.chatRepo.InsertMessage(&channelMessage)

			if err := chat.channelMessageTopic.Produce(ctx, ChannelMessage{&channelMessage}); err != nil {
				return errors.Wrap(err, "produce message failed")
			}
		case domain.JoinChannel:
			channelName = req.SendChannelReq.ChannelName
			channelID := chat.chatRepo.GetChannelID(req.SendChannelReq.ChannelName)

			cur := chat.chatRepo.GetHistory(req.JoinChannelReq.CurMaxMessageID, channelID)
			for cur.Next(ctx) {
				message, err := cur.Decode()
				if err != nil {
					return errors.Wrap(err, "get history failed")
				}

				stream.SendToOut(&domain.ChatResponse{
					Data:      message.Content,
					UserID:    message.UserID,
					MessageID: message.MessageID,
				})
			}

			// TODO: subscribe channel users status
			joinObserver := chat.channelMessageTopic.Subscribe(channelName, func(message []byte) error {
				chMsg := new(domain.ChannelMessage)
				if err := json.Unmarshal(message, chMsg); err != nil {
					return errors.Wrap(err, "unmarshal error failed")
				}

				if chMsg.ChannelID != channelID {
					return nil
				}

				stream.SendToOut(&domain.ChatResponse{
					Data:      chMsg.Content,
					UserID:    chMsg.UserID,
					MessageID: chMsg.MessageID,
				})

				return nil
			})
			joinObservers = append(joinObservers, joinObserver)
		}
	}

	for _, joinObserver := range joinObservers {
		chat.channelMessageTopic.UnSubscribe(channelName, joinObserver) // TODO: maybe defer
	}

	return nil
}
