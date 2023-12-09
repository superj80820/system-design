package repository

import (
	"context"
	"strconv"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
)

func (suite *TestSuite) TestGetHistoryMessageByChannel() {
	ctx := context.Background()

	for i := 1; i <= 10; i++ {
		_, err := suite.chatRepo.InsertChannelMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertChannelMessage(ctx, 2, 1, "content: b"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertChannelMessage(ctx, 1, 2, "content: c"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByChannel(ctx, 1, 0, curPage)
			assert.Nil(suite.T(), err)
			if isEnd {
				assert.Equal(suite.T(), 2, len(historyMessage))
			} else {
				assert.Equal(suite.T(), 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(suite.T(), 8, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i*2].Content)
			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+1), allHistoryMessage[i*2+1].Content)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByChannel(ctx, 1, 2, curPage)
			assert.Nil(suite.T(), err)
			assert.Equal(suite.T(), 3, len(historyMessage))

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(suite.T(), 7, curPage)

		for i := 0; i < 9; i++ {
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+2), allHistoryMessage[i*2].Content)
			assert.Equal(suite.T(), "content: b"+strconv.Itoa(i+2), allHistoryMessage[i*2+1].Content)
		}
	}
}
