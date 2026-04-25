package middleware

import "github.com/gin-gonic/gin"

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func JSONError(c *gin.Context, code int, errType, message string) {
	c.AbortWithStatusJSON(code, ErrorResponse{
		Error:   errType,
		Message: message,
		Code:    code,
	})
}
