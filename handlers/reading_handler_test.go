package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"health-go-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateReadingPatientNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Reading{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := NewReadingHandler(db, nil, nil)
	r := gin.New()
	r.POST("/readings", h.CreateReading)

	payload := map[string]any{
		"patient_id": 9999,
		"bpm":        80,
		"spo2":       98,
		"temp":       36.8,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/readings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing patient, got %d body=%s", w.Code, w.Body.String())
	}
}
