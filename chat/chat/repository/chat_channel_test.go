package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
)

func testGetHistoryMessageByChannel(t *testing.T, ctx context.Context, chatRepo *ChatRepo) {
	for i := 1; i <= 10; i++ {
		err := chatRepo.InsertChannelMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 2, 1, "content: b"+strconv.Itoa(i))
		assert.Nil(t, err)
		err = chatRepo.InsertChannelMessage(ctx, 1, 2, "content: c"+strconv.Itoa(i))
		assert.Nil(t, err)
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.ChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.ChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByChannel(ctx, 1, 0, curPage)
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
		assert.Equal(t, 8, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].Content)
		}
	}

	{
		var (
			isEnd             bool
			allHistoryMessage []*domain.ChannelMessage
		)
		curPage := 1
		for ; !isEnd; curPage++ {
			var (
				historyMessage []*domain.ChannelMessage
				err            error
			)
			historyMessage, isEnd, err = chatRepo.GetHistoryMessageByChannel(ctx, 1, 2, curPage)
			assert.Nil(t, err)
			assert.Equal(t, 3, len(historyMessage))

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(t, 7, curPage)

		for i := 0; i < 9; i++ {
			assert.Equal(t, "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].Content)
			assert.Equal(t, "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].Content)
		}
	}
}
