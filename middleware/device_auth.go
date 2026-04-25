package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func DeviceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader("X-Device-Key"))
		if provided == "" {
			JSONError(c, 401, "unauthorized", "missing device key")
			return
		}
		c.Set("device_key", provided)
		c.Next()
	}
}
