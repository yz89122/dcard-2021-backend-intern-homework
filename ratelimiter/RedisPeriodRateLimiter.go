package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisPeriodRateLimiter is a rate limiter that periodically
// reset the counter
type RedisPeriodRateLimiter interface {
	Request(ctx context.Context, key string, cost float64) (float64, time.Time, error)
}

// ErrRedisPeriodRateLimiterUnknown represents a unknown error
var ErrRedisPeriodRateLimiterUnknown = errors.New("redis rate limiter unknown error")

// can only be created by constructor
type redisPeriodRateLimiterImpl struct {
	resetPeriod time.Duration
	quota       float64
	redis       redis.UniversalClient
	prefix      string
}

const redisPeriodRateLimiterScript string = `local key = KEYS[1]
local isEnough, cost, quota, pttl = true, ARGV[1], ARGV[2], ARGV[3]
if redis.call("exists", key) == 1 then
	quota = redis.call("get", key)
	pttl = redis.call("pttl", key)
else
	redis.call("set", key, quota, "px", pttl)
end
if tonumber(cost) <= tonumber(quota) then
	quota = redis.call("incrbyfloat", key, "-" .. cost)
else
	isEnough = false
end
return {quota, pttl, tostring(isEnough)}
`

// NewRedisPeriodRateLimiter creates a new RedisPeriodRateLimiter
func NewRedisPeriodRateLimiter(period time.Duration, quota float64, rdb redis.UniversalClient, prefix string) RedisPeriodRateLimiter {
	return &redisPeriodRateLimiterImpl{
		resetPeriod: period,
		quota:       quota,
		redis:       rdb,
		prefix:      prefix,
	}
}

// Request decrement the request quota of the key and return
// the remaining request quota and the time to be reset.
func (limiter *redisPeriodRateLimiterImpl) Request(ctx context.Context, key string, cost float64) (float64, time.Time, error) {
	redisKey := fmt.Sprintf("%s:period-rate-limit:%s", limiter.prefix, key)
	script := redis.NewScript(redisPeriodRateLimiterScript)
	r, err := script.Run(ctx, limiter.redis, []string{redisKey}, cost, limiter.quota, int64(limiter.resetPeriod.Milliseconds())).Result()
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("redis error: %v", err)
	}

	// parse the response
	resultsObject, ok := r.([]interface{})
	if !ok {
		return 0, time.Time{}, ErrRedisPeriodRateLimiterUnknown
	}
	quota, err := redis.NewCmdResult(resultsObject[0], nil).Float64()
	if err != nil {
		return 0, time.Time{}, ErrRedisPeriodRateLimiterUnknown
	}
	pttl, err := redis.NewCmdResult(resultsObject[1], nil).Int64()
	if err != nil {
		return quota, time.Time{}, ErrRedisPeriodRateLimiterUnknown
	}
	isEnough, err := redis.NewCmdResult(resultsObject[2], nil).Bool()
	if err != nil {
		return 0, time.Now().Add(time.Duration(pttl) * time.Millisecond), ErrRedisPeriodRateLimiterUnknown
	}

	if !isEnough {
		return quota, time.Now().Add(time.Duration(pttl) * time.Millisecond), ErrRateLimiterQuotaNotEnough
	}
	return quota, time.Now().Add(time.Duration(pttl) * time.Millisecond), nil
}
