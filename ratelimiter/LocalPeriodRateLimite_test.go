package ratelimiter

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestNewLocalPeriodRateLimiter(t *testing.T) {
	if limiter := NewLocalPeriodRateLimiter(10*time.Second, 10); limiter == nil {
		t.Errorf("wants not nil")
	}
}

func TestLocalPeriodRateLimiterRequests(t *testing.T) {
	limiter := NewLocalPeriodRateLimiter(10*time.Second, 10)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			quota, _, err := limiter.Request(context.Background(), fmt.Sprintf("test:%d", i), 1)
			if quota < 0 {
				t.Errorf("quota should >= 0, got %v", quota)
			}
			if err != nil {
				t.Errorf("err != nil: %v", err)
			}
		}
	}
}

func TestLocalPeriodRateLimiterQuotaNotEnough(t *testing.T) {
	limiter := NewLocalPeriodRateLimiter(10*time.Second, 10)
	_, _, err := limiter.Request(context.Background(), "test", 11)
	if err != ErrRateLimiterQuotaNotEnough {
		t.Errorf("expected %v, got %v", ErrRateLimiterQuotaNotEnough, err)
	}
}

func BenchmarkLocalPeriodRateLimiter(b *testing.B) {
	limiter := NewLocalPeriodRateLimiter(10*time.Second, 10)
	c := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := limiter.Request(c, "test", 1); err != nil && err != ErrRateLimiterQuotaNotEnough {
			b.Errorf("unknown error, got %v", err)
		}
	}
}

func BenchmarkLocalPeriodRateLimiterParallel(b *testing.B) {
	limiter := NewLocalPeriodRateLimiter(10*time.Second, 10)
	c := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := fmt.Sprintf("test %v", rand.Int())
		for pb.Next() {
			if _, _, err := limiter.Request(c, key, 1); err != nil && err != ErrRateLimiterQuotaNotEnough {
				b.Errorf("unknown error, got %v", err)
			}
		}
	})
}
