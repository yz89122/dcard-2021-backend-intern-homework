package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/yz89122/dcard-2021-backend-intern-homework/middleware"
	"github.com/yz89122/dcard-2021-backend-intern-homework/ratelimiter"
)

func main() {
	engine := gin.Default()

	router := engine.RouterGroup

	{
		router := router.Group("/local-time-period/")
		rateLimiter := ratelimiter.NewLocalPeriodRateLimiter(1*time.Hour, 1000)
		router.Use(middleware.PeriodRateLimitByIPAddr(rateLimiter))
		router.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
	}

	{
		rdb := redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs: []string{":6379"},
		})
		{
			router := router.Group("/redis-time-period/")
			redisPeriodRatelimiter := ratelimiter.NewRedisPeriodRateLimiter(1*time.Hour, 1000, rdb, "test")
			router.Use(middleware.PeriodRateLimitByIPAddr(redisPeriodRatelimiter))
			router.GET("/ping", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "pong",
				})
			})
		}
		{
			router := router.Group("/redis-token-bucket/")
			redisTokenBucketLimiter := ratelimiter.NewRedisTokenBucketRateLimiter(1*time.Hour, 1000, rdb, "test")
			router.Use(middleware.PeriodRateLimitByIPAddr(redisTokenBucketLimiter))
			router.GET("/ping", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "pong",
				})
			})
		}
	}

	engine.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
