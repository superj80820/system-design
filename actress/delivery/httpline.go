package http

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type eventType string

const eventTypeMessage eventType = "message"

type messageType string

const (
	messageTypeText  messageType = "text"
	messageTypeImage messageType = "image"
)

type sourceType string

const (
	sourceTypeUser  sourceType = "user"
	sourceTypeGroup sourceType = "group"
)

type lineWebhookRequest struct {
	Destination string `json:"destination"`
	Events      []struct {
		DeliveryContext struct {
			IsRedelivery bool `json:"isRedelivery"`
		} `json:"deliveryContext"`
		Message struct {
			ContentProvider *struct {
				Type string `json:"type"`
			} `json:"contentProvider"`
			ID         string      `json:"id"`
			QuoteToken string      `json:"quoteToken"`
			Text       string      `json:"text"`
			Type       messageType `json:"type"`
		} `json:"message"`
		Mode       string `json:"mode"`
		ReplyToken string `json:"replyToken"`
		Source     struct {
			Type    sourceType `json:"type"`
			GroupID string     `json:"groupId"`
			UserID  string     `json:"userId"`
		} `json:"source"`
		Timestamp      int64     `json:"timestamp"`
		Type           eventType `json:"type"`
		WebhookEventID string    `json:"webhookEventId"`
	} `json:"events"`
}

var (
	DecodeLineWebhookRequest  = httpTransportKit.DecodeJsonRequest[lineWebhookRequest]
	EncodeLineWebhookResponse = httpTransportKit.EncodeOKResponse
)

func MakeLineWebhookEndpoint(actressLineUseCase domain.ActressLineUseCase, logger loggerKit.Logger) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		go func() {
			if err := func() error {

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				req := request.(lineWebhookRequest)
				for _, event := range req.Events {
					if event.Type != eventTypeMessage {
						return nil
					}

					switch event.Message.Type {
					case messageTypeText:
						switch event.Message.Text {
						case "許願":
							if err := actressLineUseCase.GetWish(event.ReplyToken); err != nil {
								return errors.Wrap(err, "get wish failed")
							}
						case "使用說明":
							if err := actressLineUseCase.GetUseInformation(event.ReplyToken); err != nil {
								return errors.Wrap(err, "get information failed")
							}
						case "辨識":
							if event.Source.GroupID == "" { // TODO: maybe use optional or pointer or omitempty
								return nil
							}
							if err := actressLineUseCase.EnableGroupRecognition(ctx, event.Source.GroupID, event.ReplyToken); err != nil {
								return errors.Wrap(err, "get wish failed")
							}
						}
					case messageTypeImage:
						switch event.Source.Type {
						case sourceTypeUser:
							if err := actressLineUseCase.RecognitionByUser(ctx, event.Message.ID, event.ReplyToken); err != nil {
								return errors.Wrap(err, "recognition by user failed")
							}
						case sourceTypeGroup:
							if err := actressLineUseCase.RecognitionByGroup(ctx, event.Source.GroupID, event.Message.ID, event.ReplyToken); err != nil {
								return errors.Wrap(err, "recognition by group failed")
							}
						}
					}
				}
				return nil
			}(); err != nil {
				logger.Error(fmt.Sprintf("%+v", err))
			}
		}()
		return nil, nil
	}
}
