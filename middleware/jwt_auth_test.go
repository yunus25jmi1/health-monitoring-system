package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"health-go-backend/config"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
)

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{JWTSecret: "jwt-test", AccessTokenH: 8, RefreshTokenD: 7}
	doctor := models.User{ID: 1, Role: models.RoleDoctor}
	patient := models.User{ID: 2, Role: models.RolePatient}

	doctorToken, err := services.GenerateAccessToken(cfg, doctor)
	if err != nil {
		t.Fatalf("failed generating doctor token: %v", err)
	}
	patientToken, err := services.GenerateAccessToken(cfg, patient)
	if err != nil {
		t.Fatalf("failed generating patient token: %v", err)
	}

	r := gin.New()
	r.GET("/doctor", JWTAuth(cfg, models.RoleDoctor), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	cases := []struct {
		name   string
		token  string
		status int
	}{
		{name: "missing token", token: "", status: http.StatusUnauthorized},
		{name: "patient forbidden", token: patientToken, status: http.StatusForbidden},
		{name: "doctor allowed", token: doctorToken, status: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/doctor", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, w.Code)
			}
		})
	}
}
