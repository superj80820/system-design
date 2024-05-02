package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	redisContainer "github.com/superj80820/system-design/kit/testing/redis/container"
)

func TestActressLineRepo(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redisContainer.CreateRedis(ctx)
	assert.Nil(t, err)
	defer redisContainer.Terminate(ctx)

	redisCache, err := redisKit.CreateCache(redisContainer.GetURI(), "", 0)
	assert.Nil(t, err)

	actressLineRepo := CreateActressLineRepo(redisCache)

	exists, err := actressLineRepo.IsEnableGroupRecognition(ctx, "groupID")
	assert.Nil(t, err)
	assert.False(t, exists)

	assert.Nil(t, actressLineRepo.EnableGroupRecognition(ctx, "groupID"))

	exists, err = actressLineRepo.IsEnableGroupRecognition(ctx, "groupID")
	assert.Nil(t, err)
	assert.True(t, exists)

	assert.Nil(t, actressLineRepo.DisableGroupRecognition(ctx, "groupID"))

	exists, err = actressLineRepo.IsEnableGroupRecognition(ctx, "groupID")
	assert.Nil(t, err)
	assert.False(t, exists)

}
