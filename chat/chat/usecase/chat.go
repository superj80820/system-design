package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	"github.com/superj80820/system-design/chat/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
	mqKit "github.com/superj80820/system-design/kit/mq"
	mqReaderManagerKit "github.com/superj80820/system-design/kit/mq/reader_manager"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type ChatUseCase struct {
	chatRepo domain.ChatRepository

	channelMessageTopic *mqKit.MQTopic // TODO: to domain
	accountMessageTopic *mqKit.MQTopic
	accountStatusTopic  *mqKit.MQTopic
}

func MakeChatUseCase(chatRepo domain.ChatRepository, channelMessageTopic, userMessageTopic, userStatusTopic *mqKit.MQTopic) *ChatUseCase {
	return &ChatUseCase{
		chatRepo:            chatRepo,
		channelMessageTopic: channelMessageTopic,
		accountMessageTopic: userMessageTopic,
		accountStatusTopic:  userStatusTopic,
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

func (chat *ChatUseCase) Chat(ctx context.Context, stream endpoint.Stream[domain.ChatRequest, domain.ChatResponse]) error {
	token := httpKit.GetToken(ctx)
	accountID, err := strconv.Atoi(token) // TODO
	if err != nil {
		return errors.Wrap(err, "get user id failed")
	}

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return errors.Wrap(err, "get unique id failed")
	}

	// TODO: think need tx?
	if _, err := chat.chatRepo.GetOrCreateUserChatInformation(ctx, accountID); err != nil {
		return errors.Wrap(err, "create user chat information failed")
	}

	chat.chatRepo.Online(ctx, accountID)
	chat.accountStatusTopic.Produce(ctx, &StatusMessage{&domain.StatusMessage{ // TODO: refactor
		StatusType: domain.OnlineStatusType,
	}})
	defer func() {
		chat.chatRepo.Offline(ctx, accountID)
		chat.accountStatusTopic.Produce(ctx, &StatusMessage{&domain.StatusMessage{ // TODO: refactor
			StatusType: domain.OfflineStatusType,
		}})
	}()

	addAccountMessageObserverCh, removeAccountMessageObserverCh := observerManger(ctx, chat.accountMessageTopic)
	addChannelMessageObserverCh, removeChannelMessageObserverCh := observerManger(ctx, chat.channelMessageTopic)
	addAccountStatusObserverCh, removeAccountStatusObserverCh := observerManger(ctx, chat.accountStatusTopic)

	accountStatusObserver := chat.accountStatusTopic.Subscribe(strconv.Itoa(accountID), func(message []byte) error {
		statusMessage := new(domain.StatusMessage)
		if err := json.Unmarshal(message, statusMessage); err != nil {
			return errors.Wrap(err, "unmarshal status message failed")
		}

		if statusMessage.UserID != accountID {
			return nil
		}

		stream.Send(&domain.ChatResponse{}) // TODO: response

		switch statusMessage.StatusType {
		case domain.AddFriendStatusType:
			addFriendStatus := new(domain.AddFriendStatus)
			json.Unmarshal([]byte(statusMessage.Content), addFriendStatus) // TODO: need byte? // TODO: error handle

			friendMessageObserver := chat.accountMessageTopic.Subscribe(
				strconv.Itoa(addFriendStatus.FriendUserID),
				friendMessageNotify(addFriendStatus.FriendUserID, stream),
			)
			addAccountMessageObserverCh <- friendMessageObserver

			friendStatusObserver := chat.accountStatusTopic.Subscribe(
				strconv.Itoa(addFriendStatus.FriendUserID),
				friendStatusNotify(addFriendStatus.FriendUserID, stream),
			)
			addAccountStatusObserverCh <- friendStatusObserver
		case domain.RemoveFriendStatusType:
			removeFriendStatus := new(domain.RemoveFriendStatus)
			json.Unmarshal([]byte(statusMessage.Content), removeFriendStatus) // TODO: need byte? // TODO: error handle

			removeAccountMessageObserverCh <- strconv.Itoa(removeFriendStatus.FriendUserID)
			removeAccountStatusObserverCh <- strconv.Itoa(removeFriendStatus.FriendUserID)
		case domain.AddChannelStatusType:
			addChannelStatus := new(domain.AddChannelStatus)
			json.Unmarshal([]byte(statusMessage.Content), addChannelStatus) // TODO: need byte? // TODO: error handle

			channelMessageObserver := chat.channelMessageTopic.Subscribe(
				strconv.Itoa(addChannelStatus.ChannelID),
				channelMessageNotify(addChannelStatus.ChannelID, stream),
			)
			addChannelMessageObserverCh <- channelMessageObserver
		case domain.RemoveChannelStatusType:
			removeChannelStatus := new(domain.RemoveChannelStatus)
			json.Unmarshal([]byte(statusMessage.Content), removeChannelStatus) // TODO: need byte? // TODO: error handle

			removeChannelMessageObserverCh <- strconv.Itoa(removeChannelStatus.ChannelID)
		}

		return nil
	})
	defer chat.accountStatusTopic.UnSubscribe(accountStatusObserver)

	accountChannels, err := chat.chatRepo.GetAccountChannels(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "get account channels failed")
	}
	accountFriends, err := chat.chatRepo.GetAccountFriends(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "get account friends failed")
	}
	historyMessage, _, err := chat.chatRepo.GetHistoryMessage(ctx, accountID, 0, 1) // TODO: number offset, page
	if err != nil {
		return errors.Wrap(err, "get history message failed")
	}

	accountChannelsMarshal, err := json.Marshal(accountChannels)
	if err != nil {
		return errors.Wrap(err, "get account channel failed")
	}
	stream.Send(&domain.ChatResponse{
		Data: string(accountChannelsMarshal), // TODO
	})
	accountFriendsMarshal, err := json.Marshal(accountFriends)
	if err != nil {
		return errors.Wrap(err, "get friend channel failed")
	}
	stream.Send(&domain.ChatResponse{
		Data: string(accountFriendsMarshal), // TODO
	})
	historyMessageMarshal, err := json.Marshal(historyMessage)
	if err != nil {
		return errors.Wrap(err, "get history message failed")
	}
	stream.Send(&domain.ChatResponse{
		Data: string(historyMessageMarshal), // TODO
	})

	for _, accountChannel := range accountChannels {
		channelMessageObserver := chat.channelMessageTopic.Subscribe(
			strconv.Itoa(accountChannel.ID),
			channelMessageNotify(accountChannel.ID, stream),
		)
		addChannelMessageObserverCh <- channelMessageObserver
	}
	for _, accountFriend := range accountFriends {
		friendMessageObserver := chat.accountMessageTopic.Subscribe(
			strconv.Itoa(accountFriend.ID),
			friendMessageNotify(accountFriend.ID, stream),
		)
		addAccountMessageObserverCh <- friendMessageObserver

		friendStatusObserver := chat.accountStatusTopic.Subscribe(
			strconv.Itoa(accountFriend.ID),
			friendStatusNotify(accountFriend.ID, stream),
		)
		addAccountStatusObserverCh <- friendStatusObserver
	}

	for {
		req, err := stream.Recv()
		if err != nil {
			return errors.Wrap(err, "receive input failed")
		}
		switch req.Action {
		case domain.SendMessageToFriend:
			friendMessage := domain.FriendMessage{
				MessageID: int(uniqueIDGenerate.Generate().GetInt64()),
				Content:   req.SendChannelReq.Message,
				UserID:    accountID,
			}

			if err := chat.chatRepo.InsertFriendMessage(ctx, &friendMessage); err != nil {
				return errors.Wrap(err, "insert friend message failed")
			}

			if err := chat.channelMessageTopic.Produce(ctx, ChannelMessage{&domain.ChannelMessage{ // TODO refactor
				MessageID: int(uniqueIDGenerate.Generate().GetInt64()),
				Content:   req.SendChannelReq.Message,
				UserID:    accountID,
			}}); err != nil {
				return errors.Wrap(err, "produce message failed")
			}
		case domain.SendMessageToChannel:
			channelInfo, err := chat.chatRepo.GetChannelByName(req.SendChannelReq.ChannelName)
			if err != nil {
				return errors.Wrap(err, "get channel failed")
			}
			channelMessage := domain.ChannelMessage{
				MessageID: int(uniqueIDGenerate.Generate().GetInt64()),
				ChannelID: channelInfo.ID,
				Content:   req.SendChannelReq.Message,
				UserID:    accountID,
			}

			if err := chat.chatRepo.InsertChannelMessage(ctx, &channelMessage); err != nil {
				return errors.Wrap(err, "insert channel message failed")
			}

			if err := chat.channelMessageTopic.Produce(ctx, ChannelMessage{&domain.ChannelMessage{
				MessageID: int(uniqueIDGenerate.Generate().GetInt64()),
				ChannelID: channelInfo.ID,
				Content:   req.SendChannelReq.Message,
				UserID:    accountID,
			}}); err != nil {
				return errors.Wrap(err, "produce message failed")
			}
		case domain.GetFriendHistoryMessage:
			var friendHistoryMessage []*domain.FriendMessage
			friendHistoryMessage, _, err = chat.chatRepo.GetHistoryMessageByFriend( // TODO: curMaxMessageID?
				ctx,
				accountID,
				req.GetFriendHistoryMessage.FriendID,
				req.GetFriendHistoryMessage.CurMaxMessageID,
				req.GetFriendHistoryMessage.Page,
			)
			if err != nil {
				return errors.Wrap(err, "get friend message failed")
			}
			friendHistoryMessageMarshal, err := json.Marshal(friendHistoryMessage)
			if err != nil {
				return errors.Wrap(err, "marshal friend history failed")
			}
			stream.Send(&domain.ChatResponse{
				Data: string(friendHistoryMessageMarshal), // TODO
			})
		case domain.GetChannelHistoryMessage:
			var channelHistoryMessage []*domain.ChannelMessage
			channelHistoryMessage, _, err = chat.chatRepo.GetHistoryMessageByChannel(
				ctx,
				req.GetChannelHistoryMessage.ChannelID,
				req.GetChannelHistoryMessage.CurMaxMessageID,
				req.GetChannelHistoryMessage.Page,
			)
			if err != nil {
				return errors.Wrap(err, "get channel message failed")
			}
			channelHistoryMessageMarshal, err := json.Marshal(channelHistoryMessage)
			if err != nil {
				return errors.Wrap(err, "marshal channel history failed")
			}
			stream.Send(&domain.ChatResponse{
				Data: string(channelHistoryMessageMarshal) + "TODO: isEnd", // TODO
			})
		case domain.GetHistoryMessage:
			historyMessage, _, err = chat.chatRepo.GetHistoryMessage( // TODO: number offset, page
				ctx,
				accountID,
				req.GetHistoryMessage.CurMaxMessageID,
				req.GetHistoryMessage.Page,
			)
			if err != nil {
				return errors.Wrap(err, "get history message failed")
			}
			historyMessageMarshal, err := json.Marshal(historyMessage)
			if err != nil {
				return errors.Wrap(err, "marshal history message failed")
			}
			stream.Send(&domain.ChatResponse{
				Data: string(historyMessageMarshal) + "TODO: isEnd", // TODO
			})

		}
	}
}

func observerManger(ctx context.Context, mqTopic *mqKit.MQTopic) (
	chan *mqReaderManagerKit.Observer,
	chan string,
) { // TODO: name and variable // TODO: think to lib?
	observerHashTable := make(map[string]*mqReaderManagerKit.Observer)
	addObserverCh := make(chan *mqReaderManagerKit.Observer)
	removeObserverCh := make(chan string)
	go func() {
		for {
			select {
			case accountMessageObserver := <-addObserverCh:
				observerHashTable[accountMessageObserver.GetKey()] = accountMessageObserver
			case removeAccountMessageObserverKey := <-removeObserverCh:
				if observer, ok := observerHashTable[removeAccountMessageObserverKey]; ok {
					fmt.Println("TODO: should not be")
					mqTopic.UnSubscribe(observer)
					delete(observerHashTable, removeAccountMessageObserverKey)
				}
			case <-ctx.Done(): // TODO: check return
				for key, observer := range observerHashTable {
					mqTopic.UnSubscribe(observer)
					delete(observerHashTable, key) // TODO: right?
				}
				return
			}
		}
	}()
	return addObserverCh, removeObserverCh
}
