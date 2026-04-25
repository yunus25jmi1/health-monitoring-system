package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ReportStatusPending   = "pending"
	ReportStatusReviewed  = "reviewed"
	ReportStatusApproved  = "approved"
	ReportStatusDismissed = "dismissed"
)

type Report struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	PatientID  uint       `gorm:"index:idx_reports_patient_created,priority:1;not null" json:"patient_id"`
	DoctorID   *uint      `gorm:"index" json:"doctor_id,omitempty"`
	AIDraft    string     `gorm:"type:text;not null" json:"ai_draft"`
	FinalNote  *string    `gorm:"type:text" json:"final_notes,omitempty"`
	Status     string     `gorm:"size:20;index:idx_reports_status_created,priority:1;default:pending;not null" json:"status"`
	PDFPath    *string    `gorm:"type:text" json:"pdf_path,omitempty"`
	CreatedAt  time.Time  `gorm:"index:idx_reports_status_created,priority:2;index:idx_reports_patient_created,priority:2" json:"created_at"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
}

func IsValidReportStatus(status string) bool {
	switch status {
	case ReportStatusPending, ReportStatusReviewed, ReportStatusApproved, ReportStatusDismissed:
		return true
	default:
		return false
	}
}

func (r *Report) BeforeSave(tx *gorm.DB) error {
	r.Status = strings.ToLower(strings.TrimSpace(r.Status))
	if !IsValidReportStatus(r.Status) {
		return errors.New("invalid report status")
	}
	return nil
}
