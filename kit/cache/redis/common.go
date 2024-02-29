package redis

import (
	"context"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

func cacheSet(ctx context.Context, cmd goRedis.Cmdable, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd {
	return cmd.Set(ctx, key, value, expiration)
}
