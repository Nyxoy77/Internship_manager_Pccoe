package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type loginBucket struct {
	count       int
	windowStart time.Time
}

// LoginRateLimitMiddleware applies a simple fixed-window limit per client IP.
func LoginRateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := map[string]*loginBucket{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[ip]
		if !ok || now.Sub(b.windowStart) >= window {
			b = &loginBucket{count: 0, windowStart: now}
			buckets[ip] = b
		}
		if b.count >= limit {
			retryAfter := int(window.Seconds() - now.Sub(b.windowStart).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			mu.Unlock()
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts, please try again later"})
			c.Abort()
			return
		}
		b.count++
		mu.Unlock()

		c.Next()
	}
}
