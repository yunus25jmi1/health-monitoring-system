package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"health-go-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AsyncJobService struct {
	db         *gorm.DB
	aiService  *AIService
	pdfService *PDFService
}

type aiDraftPayload struct {
	PatientID uint `json:"patient_id"`
}

type pdfGenPayload struct {
	ReportID uint `json:"report_id"`
}

func NewAsyncJobService(db *gorm.DB, aiService *AIService, pdfService *PDFService) *AsyncJobService {
	return &AsyncJobService{db: db, aiService: aiService, pdfService: pdfService}
}

func (s *AsyncJobService) EnqueueAIDraft(patientID uint) error {
	payload, _ := json.Marshal(aiDraftPayload{PatientID: patientID})
	return s.enqueue(models.JobTypeAIDraft, string(payload))
}

func (s *AsyncJobService) EnqueuePDFGeneration(reportID uint) error {
	payload, _ := json.Marshal(pdfGenPayload{ReportID: reportID})
	return s.enqueue(models.JobTypePDFGen, string(payload))
}

func (s *AsyncJobService) enqueue(jobType, payload string) error {
	job := models.AsyncJob{
		JobType:    jobType,
		Payload:    payload,
		Status:     models.JobStatusPending,
		NextRunAt:  time.Now().UTC(),
		Attempts:   0,
		MaxRetries: 5,
	}
	return s.db.Create(&job).Error
}

func (s *AsyncJobService) ProcessDueJobs(limit int) error {
	if limit <= 0 {
		limit = 20
	}

	now := time.Now().UTC()
	var jobs []models.AsyncJob
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ? AND next_run_at <= ?", models.JobStatusPending, now).
		Order("next_run_at asc").
		Limit(limit).
		Find(&jobs).Error; err != nil {
		return err
	}

	for _, job := range jobs {
		err := s.processJob(job)
		if err != nil {
			log.Printf("async job %d (%s) failed: %v", job.ID, job.JobType, err)
		}
	}
	return nil
}

func (s *AsyncJobService) processJob(job models.AsyncJob) error {
	var runErr error

	switch job.JobType {
	case models.JobTypeAIDraft:
		var payload aiDraftPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			runErr = err
			break
		}
		runErr = s.aiService.DraftReportForPatient(payload.PatientID)
	case models.JobTypePDFGen:
		var payload pdfGenPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			runErr = err
			break
		}
		var report models.Report
		if err := s.db.First(&report, payload.ReportID).Error; err != nil {
			runErr = err
			break
		}
		_, _, runErr = s.pdfService.GenerateDualCopy(report)
	default:
		runErr = fmt.Errorf("unknown job type: %s", job.JobType)
	}

	now := time.Now().UTC()
	if runErr == nil {
		job.Status = models.JobStatusDone
		job.CompletedAt = &now
		job.LastError = nil
		return s.db.Save(&job).Error
	}

	job.Attempts++
	errText := runErr.Error()
	job.LastError = &errText
	if job.Attempts >= job.MaxRetries {
		job.Status = models.JobStatusFailed
		return s.db.Save(&job).Error
	}

	backoff := time.Duration(1<<minInt(job.Attempts, 6)) * time.Second
	job.NextRunAt = now.Add(backoff)
	job.Status = models.JobStatusPending
	return s.db.Save(&job).Error
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
