package services

import (
	"testing"

	"health-go-backend/models"
)

func TestCanTransitionReportStatus(t *testing.T) {
	if !CanTransitionReportStatus(models.ReportStatusPending, models.ReportStatusReviewed) {
		t.Fatalf("expected pending -> reviewed to be allowed")
	}
	if !CanTransitionReportStatus(models.ReportStatusReviewed, models.ReportStatusApproved) {
		t.Fatalf("expected reviewed -> approved to be allowed")
	}

	if CanTransitionReportStatus(models.ReportStatusPending, models.ReportStatusApproved) {
		t.Fatalf("expected pending -> approved to be rejected")
	}
	if CanTransitionReportStatus(models.ReportStatusApproved, models.ReportStatusReviewed) {
		t.Fatalf("expected approved -> reviewed to be rejected")
	}
}
