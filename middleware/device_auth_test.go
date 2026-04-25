package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDeviceAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/telemetry", DeviceAuth(), func(c *gin.Context) {
		key, _ := c.Get("device_key")
		c.JSON(http.StatusCreated, gin.H{"ok": true, "key": key})
	})

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/telemetry", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/telemetry", nil)
		req.Header.Set("X-Device-Key", "any-key")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
	})
}
