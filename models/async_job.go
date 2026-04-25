package models

import "time"

const (
	JobTypeAIDraft = "ai_draft"
	JobTypePDFGen  = "pdf_generate"

	JobStatusPending = "pending"
	JobStatusDone    = "done"
	JobStatusFailed  = "failed"
)

type AsyncJob struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	JobType     string     `gorm:"size:30;index:idx_job_type_status,priority:1;not null" json:"job_type"`
	Payload     string     `gorm:"type:text;not null" json:"payload"`
	Status      string     `gorm:"size:20;index:idx_job_type_status,priority:2;not null;default:pending" json:"status"`
	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	MaxRetries  int        `gorm:"not null;default:5" json:"max_retries"`
	NextRunAt   time.Time  `gorm:"index;not null" json:"next_run_at"`
	LastError   *string    `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
