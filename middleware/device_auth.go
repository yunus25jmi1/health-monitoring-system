package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func DeviceAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader("X-Device-Key"))
		if provided == "" || provided != strings.TrimSpace(secret) {
			JSONError(c, 401, "unauthorized", "invalid or missing device key")
			return
		}
		c.Next()
	}
}
