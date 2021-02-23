package ratelimiter

import (
	"context"
	"sync"
	"time"
)

// LocalPeriodRateLimiter is a rate limiter that periodically
// reset the counter and only works in local
type LocalPeriodRateLimiter interface {
	Request(ctx context.Context, key string, cost float64) (float64, time.Time, error)
}

// can only be created by constructor
type localPeriodRateLimiterImpl struct {
	period           time.Duration
	quota            float64
	quotaMap         map[string]*localPeriodRateLimiterQuota
	mutex            sync.RWMutex
	nextPruneAt      time.Time
	nextPruneAtMutex sync.Mutex
	pruneQueue       []*localPeriodRateLimiterPruneJob
	pruneQueueMutex  sync.Mutex
}

type localPeriodRateLimiterQuota struct {
	mutex     sync.Mutex
	value     float64
	expiresAt time.Time
}

type localPeriodRateLimiterPruneJob struct {
	key       string
	expiredAt time.Time
}

// NewLocalPeriodRateLimiter creates a new LocalPeriodRateLimiter
func NewLocalPeriodRateLimiter(period time.Duration, quota float64) LocalPeriodRateLimiter {
	return &localPeriodRateLimiterImpl{
		period:      period,
		quota:       quota,
		quotaMap:    make(map[string]*localPeriodRateLimiterQuota),
		nextPruneAt: time.Now().Add(period),
		pruneQueue:  make([]*localPeriodRateLimiterPruneJob, 0, 8),
	}
}

func (limiter *localPeriodRateLimiterImpl) getQuota(key string) *localPeriodRateLimiterQuota {
	limiter.mutex.RLock()
	defer limiter.mutex.RUnlock()
	return limiter.quotaMap[key] // return nil(zero value) on not found
}

func (limiter *localPeriodRateLimiterImpl) appendToPruneQueue(key string, expiresAt time.Time) {
	limiter.pruneQueueMutex.Lock()
	defer limiter.pruneQueueMutex.Unlock()
	limiter.pruneQueue = append(limiter.pruneQueue, &localPeriodRateLimiterPruneJob{
		key:       key,
		expiredAt: expiresAt,
	})
}

func (limiter *localPeriodRateLimiterImpl) tryRequest(key string, quota *localPeriodRateLimiterQuota, cost float64) (float64, time.Time, error) {
	// lock the quota object to make sure there's only one
	// goroutine is mutating the quota object
	quota.mutex.Lock()
	defer quota.mutex.Unlock()

	// check the expire time
	if now := time.Now(); now.After(quota.expiresAt) {
		quota.value = limiter.quota
		expiresAt := now.Add(limiter.period)
		quota.expiresAt = expiresAt
		limiter.appendToPruneQueue(key, expiresAt)
	}

	// check quota
	if quota.value < cost {
		return quota.value, quota.expiresAt, ErrRateLimiterQuotaNotEnough
	}
	quota.value -= cost
	return quota.value, quota.expiresAt, nil
}

func (limiter *localPeriodRateLimiterImpl) createQuotaForKey(key string) *localPeriodRateLimiterQuota {
	quota := &localPeriodRateLimiterQuota{
		value:     limiter.quota,
		expiresAt: time.Now().Add(limiter.period),
	}
	limiter.appendToPruneQueue(key, quota.expiresAt)
	limiter.quotaMap[key] = quota
	return quota
}

// Request increment the request count of the key and return
// the remaining request count and the time to be reset.
func (limiter *localPeriodRateLimiterImpl) Request(_ context.Context, key string, cost float64) (float64, time.Time, error) {
	limiter.tryPruneKeys()

	if quota := limiter.getQuota(key); quota != nil { // getQuota() requires the read lock
		return limiter.tryRequest(key, quota, cost)
	}

	quota := func() *localPeriodRateLimiterQuota {
		// wrap in func in order to release the lock ASAP
		limiter.mutex.Lock()
		defer limiter.mutex.Unlock()
		if quota := limiter.quotaMap[key]; quota != nil {
			// if other writer already created for the key
			return quota
		}
		return limiter.createQuotaForKey(key)
	}()

	return limiter.tryRequest(key, quota, cost)
}

func (limiter *localPeriodRateLimiterImpl) pruneKeys() {
	index := 0
	now := time.Now()

	// we're going to remove keys from the map
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	// we're going to pop from the queue
	limiter.pruneQueueMutex.Lock()
	defer limiter.pruneQueueMutex.Unlock()

	for _, job := range limiter.pruneQueue {
		if now.Before(job.expiredAt) {
			break
		}
		index++
		if now.After(limiter.quotaMap[job.key].expiresAt) {
			delete(limiter.quotaMap, job.key)
		}
	}
	// pop from the queue
	// the underlying slice will be updated by the append() function
	limiter.pruneQueue = limiter.pruneQueue[index:]
}

func (limiter *localPeriodRateLimiterImpl) shouldPrune() bool {
	if now := time.Now(); now.After(limiter.nextPruneAt) {
		// no need to lock if we just read the data
		// less locking to improve performance
		return func() bool {
			limiter.nextPruneAtMutex.Lock()
			defer limiter.nextPruneAtMutex.Unlock()
			if now.After(limiter.nextPruneAt) {
				// if it's the first goroutine do the pruning
				limiter.nextPruneAt = now.Add(limiter.period)
				return true
			}
			return false
		}()
	}
	return false
}

func (limiter *localPeriodRateLimiterImpl) tryPruneKeys() {
	if limiter.shouldPrune() { // Note: shouldPrune() will update limiter.nextPruneAt
		limiter.pruneKeys()
	}
}
