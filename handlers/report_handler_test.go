package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Report{}, &models.Reading{}, &models.AsyncJob{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestStreamPDF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupReportTestDB(t)

	tmpDir := t.TempDir()
	report := models.Report{ID: 9, PatientID: 3, Status: models.ReportStatusApproved, AIDraft: "ok"}
	base := filepath.Join(tmpDir, "9")
	report.PDFPath = &base
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("failed to create report: %v", err)
	}
	if err := os.WriteFile(base+"_patient.pdf", []byte("pdf"), 0o644); err != nil {
		t.Fatalf("failed to seed patient pdf: %v", err)
	}

	jobService := services.NewAsyncJobService(db, nil, nil)
	h := NewReportHandler(config.Config{PDFStorage: tmpDir}, db, services.NewPDFService(config.Config{PDFStorage: tmpDir}, db), jobService)
	r := gin.New()
	r.GET("/reports/:id/pdf", func(c *gin.Context) {
		c.Set("auth_role", models.RoleDoctor)
		c.Set("auth_user_id", uint(1))
		h.StreamPDF(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/reports/9/pdf?copy=patient", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got == "" {
		t.Fatalf("expected content type header")
	}
}

func TestApproveConflictAlreadyApproved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupReportTestDB(t)
	tmpDir := t.TempDir()

	doctor := models.User{Name: "Dr Test", Email: "doctor@test.local", Password: "hash", Role: models.RoleDoctor}
	if err := db.Create(&doctor).Error; err != nil {
		t.Fatalf("failed to create doctor: %v", err)
	}

	now := nowUTC()
	report := models.Report{
		PatientID:  3,
		DoctorID:   &doctor.ID,
		Status:     models.ReportStatusApproved,
		AIDraft:    "already approved",
		ApprovedAt: &now,
	}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	h := NewReportHandler(config.Config{PDFStorage: tmpDir}, db, services.NewPDFService(config.Config{PDFStorage: tmpDir}, db), nil)
	r := gin.New()
	r.POST("/reports/:id/approve", func(c *gin.Context) {
		c.Set("auth_role", models.RoleDoctor)
		c.Set("auth_user_id", doctor.ID)
		h.Approve(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/reports/"+itoa(int(report.ID))+"/approve", bytes.NewBuffer(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStreamPDFFileNotReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupReportTestDB(t)
	tmpDir := t.TempDir()

	report := models.Report{PatientID: 3, Status: models.ReportStatusApproved, AIDraft: "ok"}
	base := filepath.Join(tmpDir, "11")
	report.PDFPath = &base
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	h := NewReportHandler(config.Config{PDFStorage: tmpDir}, db, services.NewPDFService(config.Config{PDFStorage: tmpDir}, db), nil)
	r := gin.New()
	r.GET("/reports/:id/pdf", func(c *gin.Context) {
		c.Set("auth_role", models.RoleDoctor)
		c.Set("auth_user_id", uint(1))
		h.StreamPDF(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/reports/"+itoa(int(report.ID))+"/pdf?copy=clinical", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 not_found, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestApproveConcurrentRaceOneWins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupReportTestDB(t)
	tmpDir := t.TempDir()

	doctor := models.User{Name: "Dr Race", Email: "race@test.local", Password: "hash", Role: models.RoleDoctor}
	if err := db.Create(&doctor).Error; err != nil {
		t.Fatalf("failed to create doctor: %v", err)
	}
	report := models.Report{PatientID: 4, Status: models.ReportStatusReviewed, AIDraft: "ready"}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	h := NewReportHandler(config.Config{PDFStorage: tmpDir}, db, services.NewPDFService(config.Config{PDFStorage: tmpDir}, db), nil)
	r := gin.New()
	r.POST("/reports/:id/approve", func(c *gin.Context) {
		c.Set("auth_role", models.RoleDoctor)
		c.Set("auth_user_id", doctor.ID)
		h.Approve(c)
	})

	statuses := make([]int, 0, 2)
	bodies := make([]string, 0, 2)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/reports/"+itoa(int(report.ID))+"/approve", bytes.NewBuffer(nil))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			mu.Lock()
			statuses = append(statuses, w.Code)
			bodies = append(bodies, w.Body.String())
			mu.Unlock()
		}()
	}
	wg.Wait()

	count200 := 0
	count409 := 0
	for _, code := range statuses {
		if code == http.StatusOK {
			count200++
		}
		if code == http.StatusConflict {
			count409++
		}
	}
	if count200 != 1 || count409 != 1 {
		t.Fatalf("expected one success and one conflict, got statuses=%v bodies=%v", statuses, bodies)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
