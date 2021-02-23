package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yz89122/dcard-2021-backend-intern-homework/ratelimiter"
)

// PeriodRateLimitByIPAddr limit the request rate of each IP address
// by the given limiter
func PeriodRateLimitByIPAddr(limiter ratelimiter.RateLimiter) gin.HandlerFunc {
	const rateLimitResetHeader string = "X-RateLimit-Reset"
	const rateLimitRemainingHeader string = "X-RateLimit-Remaining"
	return func(c *gin.Context) {
		remaining, resetAt, err := limiter.Request(c, c.ClientIP(), 1)
		if err == ratelimiter.ErrRateLimiterQuotaNotEnough || remaining < 0 {
			c.Header(rateLimitResetHeader, resetAt.UTC().Format(http.TimeFormat))
			c.Header(rateLimitRemainingHeader, fmt.Sprintf("%d", 0))
			c.AbortWithStatus(429)
			return
		}
		if err != nil {
			fmt.Println(err) // TODO: should log by logger
			c.AbortWithStatus(500)
			return
		}
		c.Header(rateLimitResetHeader, resetAt.UTC().Format(http.TimeFormat))
		c.Header(rateLimitRemainingHeader, fmt.Sprintf("%d", int64(remaining)))
		c.Next()
	}
}
