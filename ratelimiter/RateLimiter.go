package ratelimiter

import (
	"context"
	"errors"
	"time"
)

// ErrRateLimiterQuotaNotEnough is an error indicating
// the request with the given key does not have enough quota.
var ErrRateLimiterQuotaNotEnough error = errors.New("quota not enough")

// RateLimiter limits the rate by the given key
type RateLimiter interface {
	// Request accepts a string as a key and a float64 as
	// the cost of the request. Returns the remaining quota
	// as float64 and reset time.
	Request(ctx context.Context, key string, cost float64) (float64, time.Time, error)
}
