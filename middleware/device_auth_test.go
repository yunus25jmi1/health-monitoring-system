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
	r.POST("/telemetry", DeviceAuth("secret-123"), func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/telemetry", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/telemetry", nil)
		req.Header.Set("X-Device-Key", "wrong")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/telemetry", nil)
		req.Header.Set("X-Device-Key", "secret-123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
	})
}
