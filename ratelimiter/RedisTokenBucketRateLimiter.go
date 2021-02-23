package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisTokenBucketRateLimiter is a rate limiter that periodically
// reset the counter
type RedisTokenBucketRateLimiter interface {
	Request(ctx context.Context, key string, cost float64) (float64, time.Time, error)
}

// ErrRedisTokenBucketRateLimiterUnknown represents a unknown error
var ErrRedisTokenBucketRateLimiterUnknown = errors.New("redis rate limiter unknown error")

// can only be created by constructor
type redisTokenBucketRateLimiterImpl struct {
	period time.Duration
	quota  float64
	redis  redis.UniversalClient
	prefix string
}

const redisTokenBucketRateLimiterScript string = `local key = KEYS[1]
local cost, maxQuota, period = tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3])
local now = redis.call("time")[1]
local quota, last, isEnough = maxQuota, now, true
if redis.call("exists", key) == 1 then
	local response = redis.call("hmget", key, "quota", "last")
	quota, last = tonumber(response[1]), tonumber(response[2])
else
	redis.call("hmset", key, "quota", quota, "last", now)
end
redis.call("expire", key, period)
quota = quota + (now - last) * (maxQuota / period)
if maxQuota < quota then
	quota = maxQuota
end
if tonumber(cost) <= tonumber(quota) then
	quota = quota - cost
else
	isEnough = false
end
redis.call("hmset", key, "quota", quota, "last", now)
return {quota, tostring(isEnough)}
`

// NewRedisTokenBucketRateLimiter creates a new RedisTokenBucketRateLimiter.
// Accepts a prefix for prepending on redis key.
func NewRedisTokenBucketRateLimiter(period time.Duration, quota float64, rdb redis.UniversalClient, prefix string) RedisTokenBucketRateLimiter {
	return &redisTokenBucketRateLimiterImpl{
		period: period,
		quota:  quota,
		redis:  rdb,
		prefix: prefix,
	}
}

// Request decrement the request quota of the key and return
// the remaining request quota and the time to be reset.
func (limiter *redisTokenBucketRateLimiterImpl) Request(ctx context.Context, key string, cost float64) (float64, time.Time, error) {
	redisKey := fmt.Sprintf("%s:token-bucket-rate-limit:%s", limiter.prefix, key)
	script := redis.NewScript(redisTokenBucketRateLimiterScript)
	r, err := script.Run(ctx, limiter.redis, []string{redisKey}, cost, limiter.quota, int64(limiter.period.Seconds())).Result()
	resetAt := time.Now().Add(limiter.period)
	if err != nil {
		return 0, resetAt, fmt.Errorf("redis error: %v", err)
	}

	// parse the response
	resultsObject, ok := r.([]interface{})
	if !ok {
		return 0, resetAt, ErrRedisTokenBucketRateLimiterUnknown
	}
	quota, err := redis.NewCmdResult(resultsObject[0], nil).Float64()
	if err != nil {
		return 0, resetAt, ErrRedisTokenBucketRateLimiterUnknown
	}
	isEnough, err := redis.NewCmdResult(resultsObject[1], nil).Bool()
	if err != nil {
		return 0, resetAt, ErrRedisTokenBucketRateLimiterUnknown
	}

	if !isEnough {
		return quota, resetAt, ErrRateLimiterQuotaNotEnough
	}
	return quota, resetAt, nil
}
