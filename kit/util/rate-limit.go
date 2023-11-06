package util

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	redisKit "github.com/superj80820/system-design/kit/redis"
)

type CacheRateLimit struct {
	cache       *redisKit.Cache
	maxRequests int
	expiry      int
}

func CreateCacheRateLimit(cache *redisKit.Cache, maxRequests, expiry int) *CacheRateLimit {
	return &CacheRateLimit{cache: cache, maxRequests: maxRequests, expiry: expiry}
}

func (c *CacheRateLimit) Pass(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error) {
	luaScript := `
		local key = KEYS[1]
		local requests = tonumber(redis.call('GET', key) or '-1')
		local max_requests = tonumber(ARGV[1])
		local expiry = tonumber(ARGV[2])
		if (requests == -1) then
			redis.call('INCR', key)
			redis.call('EXPIRE', key, expiry)
			return {true, 1, expiry}
		end

		local cur_expiry = tonumber(redis.call('TTL', key) or '-1')
		if (requests < max_requests) then
			redis.call('INCR', key)
			return {true, requests, cur_expiry}
		else
			return {false, requests, cur_expiry}
		end
	`
	result, err := c.cache.RunLua(ctx, luaScript, []string{key}, c.maxRequests, c.expiry).Slice()
	if err != nil {
		return false, 0, 0, errors.Wrap(err, "redis run lua script failed")
	}
	pass, err = toBool(result[0])
	if err != nil {
		return false, 0, 0, errors.Wrap(err, "convert result failed")
	}
	curRequests := result[1].(int64)
	curExpiry = int(result[2].(int64))
	return pass, c.maxRequests - int(curRequests), int(curExpiry), nil
}

func toBool(val interface{}) (bool, error) {
	if val == nil {
		return false, nil
	}

	switch val := val.(type) {
	case bool:
		return val, nil
	case int64:
		return val != 0, nil
	case string:
		return strconv.ParseBool(val)
	default:
		return false, errors.New(fmt.Sprintf("unexpected type=%T for Bool", val))
	}
}
