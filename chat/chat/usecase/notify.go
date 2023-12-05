package usecase

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
)

func channelMessageNotify(channelID int, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) func(message []byte) error {
	return func(message []byte) error {
		chMsg := new(domain.ChannelMessage)
		if err := json.Unmarshal(message, chMsg); err != nil {
			return errors.Wrap(err, "unmarshal error failed")
		}

		if chMsg.ChannelID != channelID {
			return nil
		}

		stream.Send(&domain.ChatResponse{
			Data:      chMsg.Content,
			UserID:    chMsg.UserID,
			MessageID: chMsg.MessageID,
		})

		return nil
	}
}

func friendStatusNotify(friendAccountID int, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) func(message []byte) error {
	return func(message []byte) error {
		statusMessage := new(domain.StatusMessage)
		json.Unmarshal(message, statusMessage) // TODO: error handle

		if statusMessage.UserID != friendAccountID ||
			(statusMessage.StatusType != domain.OnlineStatusType &&
				statusMessage.StatusType != domain.OfflineStatusType) {
			return nil
		}

		switch statusMessage.StatusType {
		case domain.OnlineStatusType:
			stream.Send(&domain.ChatResponse{}) // TODO: response
		case domain.OfflineStatusType:
			stream.Send(&domain.ChatResponse{}) // TODO: response
		}
		return nil
	}
}

func friendMessageNotify(friendAccountID int, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) func(message []byte) error {
	return func(message []byte) error {
		friendMessage := new(domain.FriendMessage)
		json.Unmarshal(message, friendMessage) // TODO: error handle

		if friendMessage.UserID != friendAccountID {
			return nil
		}

		stream.Send(&domain.ChatResponse{
			Data:      friendMessage.Content,
			UserID:    friendMessage.UserID,
			MessageID: friendMessage.MessageID,
		})
		return nil
	}
}
