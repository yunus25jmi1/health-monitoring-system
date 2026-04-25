package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"health-go-backend/config"
	"health-go-backend/models"
	"health-go-backend/services"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRouterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Reading{}, &models.Report{}, &models.AsyncJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestProtectedReadingsRouteRequiresDoctorRole(t *testing.T) {
	db := setupRouterTestDB(t)
	cfg := config.Config{
		JWTSecret:     "test-secret",
		RateLimitRPM:  1000,
		AllowedOrigin: []string{"*"},
		DeviceSecret:  "device-key",
		PDFStorage:    t.TempDir(),
		AccessTokenH:  8,
		RefreshTokenD: 7,
	}
	r := NewRouter(cfg, db)

	doctor := models.User{Name: "Doc", Email: "doc@test.local", Password: "hash", Role: models.RoleDoctor}
	patient := models.User{Name: "Pat", Email: "pat@test.local", Password: "hash", Role: models.RolePatient, DeviceKey: "dev-1"}
	otherPatient := models.User{Name: "Other", Email: "other@test.local", Password: "hash", Role: models.RolePatient, DeviceKey: "dev-2"}
	if err := db.Create(&doctor).Error; err != nil {
		t.Fatalf("create doctor: %v", err)
	}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatalf("create patient: %v", err)
	}
	if err := db.Create(&otherPatient).Error; err != nil {
		t.Fatalf("create other patient: %v", err)
	}
	patient.DoctorID = &doctor.ID
	db.Save(&patient)


	doctorToken, err := services.GenerateAccessToken(cfg, doctor)
	if err != nil {
		t.Fatalf("doctor token: %v", err)
	}
	patientToken, err := services.GenerateAccessToken(cfg, patient)
	if err != nil {
		t.Fatalf("patient token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/readings/"+strconv.Itoa(int(otherPatient.ID)), nil)
	req.Header.Set("Authorization", "Bearer "+patientToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for patient viewing others, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/readings/"+strconv.Itoa(int(patient.ID)), nil)
	req.Header.Set("Authorization", "Bearer "+doctorToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for doctor, got %d", w.Code)
	}
}

func TestReadingsPostRequiresDeviceKey(t *testing.T) {
	db := setupRouterTestDB(t)
	cfg := config.Config{
		JWTSecret:     "test-secret",
		RateLimitRPM:  1000,
		AllowedOrigin: []string{"*"},
		DeviceSecret:  "device-key",
		PDFStorage:    t.TempDir(),
	}
	r := NewRouter(cfg, db)

	patient := models.User{Name: "Pat", Email: "pat2@test.local", Password: "hash", Role: models.RolePatient, DeviceKey: "device-key"}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatalf("create patient: %v", err)
	}

	payload := map[string]any{"patient_id": patient.ID, "bpm": 80, "spo2": 98, "temp": 36.7, "ecg_raw": 2000, "glucose_level": 100.0}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/readings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without device key, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/readings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Key", "device-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 with valid device key, got %d body=%s", w.Code, w.Body.String())
	}
}
