package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func RateLimit(perMinute int) gin.HandlerFunc {
	if perMinute <= 0 {
		perMinute = 100
	}
	var clients sync.Map

	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiterIface, _ := clients.LoadOrStore(ip, rate.NewLimiter(rate.Every(time.Minute/time.Duration(perMinute)), perMinute))
		limiter := limiterIface.(*rate.Limiter)

		if !limiter.Allow() {
			c.Header("Retry-After", "60")
			JSONError(c, http.StatusTooManyRequests, "rate_limited", fmt.Sprintf("request limit exceeded (%d/min)", perMinute))
			return
		}
		c.Next()
	}
}
