package repository

import (
	"context"
	"strconv"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/chat/domain"
)

func (suite *TestSuite) TestGetHistoryMessageByFriend() {
	ctx := context.Background()

	for i := 1; i <= 10; i++ {
		_, err := suite.chatRepo.InsertFriendMessage(ctx, 1, 1, "content: a"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertFriendMessage(ctx, 1, 2, "content: b"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
		_, err = suite.chatRepo.InsertFriendMessage(ctx, 2, 1, "content: c"+strconv.Itoa(i))
		assert.Nil(suite.T(), err)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 0, curPage)
			assert.Nil(suite.T(), err)
			if isEnd {
				assert.Equal(suite.T(), 1, len(historyMessage))
			} else {
				assert.Equal(suite.T(), 3, len(historyMessage))
			}

			for _, message := range historyMessage {
				allHistoryMessage = append(allHistoryMessage, message)
			}
		}
		assert.Equal(suite.T(), 5, curPage)

		for i := 0; i < 10; i++ {
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+1), allHistoryMessage[i].Content)
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
			historyMessage, isEnd, err = suite.chatRepo.GetHistoryMessageByFriend(ctx, 1, 1, 2, curPage)
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
		assert.Equal(suite.T(), 4, curPage)

		for i := 0; i < 8; i++ {
			assert.Equal(suite.T(), "content: a"+strconv.Itoa(i+3), allHistoryMessage[i].Content)
		}
	}
}
