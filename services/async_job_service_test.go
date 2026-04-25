package services

import (
	"testing"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAsyncJobRetryAndDoneFlow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AsyncJob{}, &models.Report{}, &models.User{}, &models.Reading{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewAsyncJobService(db, &AIService{cfg: config.Config{}}, NewPDFService(config.Config{PDFStorage: t.TempDir()}, db))
	if err := svc.EnqueuePDFGeneration(9999); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := svc.ProcessDueJobs(10); err != nil {
		t.Fatalf("process: %v", err)
	}

	var job models.AsyncJob
	if err := db.First(&job).Error; err != nil {
		t.Fatalf("fetch job: %v", err)
	}
	if job.Attempts < 1 {
		t.Fatalf("expected attempts to increment, got %d", job.Attempts)
	}
	if !job.NextRunAt.After(time.Now().UTC().Add(-1 * time.Second)) {
		t.Fatalf("expected next run to be moved forward")
	}
}
