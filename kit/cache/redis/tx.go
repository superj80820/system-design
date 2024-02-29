package redis

import (
	"context"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

type CacheTX struct {
	redisTxPipeline goRedis.Pipeliner
}

func (tx *CacheTX) Exec(ctx context.Context) error {
	_, err := tx.redisTxPipeline.Exec(ctx) // TODO: york
	return err
}

func (tx *CacheTX) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) { // TODO: 參考redis的cmdable func實作
	cacheSet(ctx, tx.redisTxPipeline, key, value, expiration)
}

func (tx *CacheTX) SetBF(ctx context.Context, key string, value interface{}) {
	tx.redisTxPipeline.Do(ctx, "BF.ADD", key, value)
}
