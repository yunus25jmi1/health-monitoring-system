package middleware

import (
	"net/http"
	"strings"

	"health-go-backend/config"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
)

func JWTAuth(cfg config.Config, allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[strings.TrimSpace(role)] = struct{}{}
	}

	return func(c *gin.Context) {
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(authorization, "Bearer ") {
			JSONError(c, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		claims, err := services.ParseToken(cfg, token)
		if err != nil {
			JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		if claims.Type != "access" {
			JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid token type")
			return
		}

		if len(allowed) > 0 {
			if _, ok := allowed[claims.Role]; !ok {
				JSONError(c, http.StatusForbidden, "forbidden", "insufficient role permissions")
				return
			}
		}

		c.Set("auth_user_id", claims.UserID)
		c.Set("auth_role", claims.Role)
		c.Next()
	}
}
