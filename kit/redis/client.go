package redis

import (
	"context"
	"time"

	"github.com/pkg/errors"
	goRedis "github.com/redis/go-redis/v9"
)

type Cache struct {
	redisClient *goRedis.Client
}

type redisClient interface {
	Do(ctx context.Context, args ...interface{}) *goRedis.Cmd
}

type Cmd struct {
	*goRedis.Cmd
}

type (
	StringSliceCmd = goRedis.StringSliceCmd
	ZRangeBy       = goRedis.ZRangeBy
)

func (cache *Cache) TxPipeline() *CacheTX {
	return &CacheTX{
		redisTxPipeline: cache.redisClient.TxPipeline(),
	}
}

func (cache *Cache) LRange(ctx context.Context, key string, start int64, stop int64) *StringSliceCmd {
	return cache.redisClient.LRange(ctx, key, start, stop)
}

func (cache *Cache) RunLua(ctx context.Context, script string, keys []string, args ...interface{}) *Cmd {
	luaScript := goRedis.NewScript(script)
	cmd := Cmd{luaScript.Run(ctx, cache.redisClient, keys, args...)}
	return &cmd
}

func (cache *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error { // TODO: tx
	return cacheSet(ctx, cache.redisClient, key, value, expiration).Err()
}

func (cache *Cache) Del(ctx context.Context, keys ...string) error { // TODO: tx
	return cache.redisClient.Del(ctx, keys...).Err()
}

func (cache *Cache) ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) *StringSliceCmd {
	return cache.redisClient.ZRangeByScore(ctx, key, opt)
}

func (cache *Cache) Get(ctx context.Context, key string) (val string, exists bool, err error) {
	val, err = cache.redisClient.Get(ctx, key).Result()
	if err == goRedis.Nil {
		return "", false, nil
	} else if err != nil {
		return "", false, errors.Wrap(err, "get redis failed")
	}
	return val, true, nil
}

func (cache *Cache) SetBF(ctx context.Context, key string, value interface{}) error {
	inserted, err := cache.redisClient.Do(ctx, "BF.ADD", key, value).Bool()
	if err != nil {
		return errors.Wrap(err, "bloom filter add failed")
	}
	if !inserted {
		return errors.New("bloom filter add failed")
	}
	return nil
}

func (cache *Cache) MaybeExistsBF(ctx context.Context, key string, values ...interface{}) (bool, error) {
	values = append(values, nil, nil) // TODO: args
	copy(values[2:], values)          // TODO: check correct
	values[0], values[1] = "BF.EXISTS", key
	maybeExists, err := cache.redisClient.Do(ctx, values...).Bool()
	if err != nil {
		return false, errors.Wrap(err, "bloom filter add failed")
	}
	return maybeExists, nil
}

func CreateCache(address, password string, dbSelect int) (*Cache, error) {
	redisClient := goRedis.NewClient(&goRedis.Options{
		Addr:     address,
		Password: password,
		DB:       dbSelect,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return nil, errors.Wrap(err, "redis connect failed")
	}
	return &Cache{redisClient: redisClient}, nil
}
