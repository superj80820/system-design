package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
)

func testGetHistoryMessageByFriend(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	for i := 1; i <= 10; i++ {
		err := chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertFriendMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
		assert.Nil(t, err)
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 0, curPage)
			assert.Nil(t, err)
			if isEnd {
				assert.Equal(t, 1, len(historyMessage))
			} else {
				assert.Equal(t, 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 5, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+1), allHistoryMessage[i].Content)
		}
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.FriendMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.FriendMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 2, curPage)
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
		assert.Equal(t, 4, curPage)

		for i := 0; i < 8; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+3), allHistoryMessage[i].Content)
		}
	}
}
