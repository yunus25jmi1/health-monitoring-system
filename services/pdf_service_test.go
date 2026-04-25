package services

import (
	"os"
	"path/filepath"
	"testing"

	"health-go-backend/config"
	"health-go-backend/models"
)

func TestGenerateDualCopy(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPDFService(config.Config{PDFStorage: tmpDir}, nil)
	notes := "reviewed by doctor"
	report := models.Report{
		ID:        77,
		PatientID: 5,
		Status:    models.ReportStatusApproved,
		AIDraft:   "Summary\n- stable",
		FinalNote: &notes,
	}

	patientPath, clinicalPath, err := svc.GenerateDualCopy(report)
	if err != nil {
		t.Fatalf("expected dual copy generation, got error: %v", err)
	}

	if filepath.Ext(patientPath) != ".pdf" || filepath.Ext(clinicalPath) != ".pdf" {
		t.Fatalf("expected pdf outputs, got patient=%s clinical=%s", patientPath, clinicalPath)
	}
	if _, err := os.Stat(patientPath); err != nil {
		t.Fatalf("expected patient pdf to exist, got: %v", err)
	}
	if _, err := os.Stat(clinicalPath); err != nil {
		t.Fatalf("expected clinical pdf to exist, got: %v", err)
	}
}

func TestGenerateDualCopyFailsOnInvalidStoragePath(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(badPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	svc := NewPDFService(config.Config{PDFStorage: badPath}, nil)
	report := models.Report{ID: 1, PatientID: 1, Status: models.ReportStatusApproved, AIDraft: "text"}
	if _, _, err := svc.GenerateDualCopy(report); err == nil {
		t.Fatalf("expected error when storage path is not a directory")
	}
}
