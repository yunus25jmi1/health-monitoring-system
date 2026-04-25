package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"health-go-backend/middleware"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ReadingHandler struct {
	db         *gorm.DB
	aiService  *services.AIService
	jobService *services.AsyncJobService
}

func NewReadingHandler(db *gorm.DB, aiService *services.AIService, jobService *services.AsyncJobService) *ReadingHandler {
	return &ReadingHandler{db: db, aiService: aiService, jobService: jobService}
}

func (h *ReadingHandler) CreateReading(c *gin.Context) {
	var req models.ReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "invalid request payload")
		return
	}

	if err := services.ValidateReading(req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "sensor values out of expected bounds")
		return
	}

	var patient models.User
	if err := h.db.Where("id = ? AND role = ?", req.PatientID, models.RolePatient).First(&patient).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "patient not found")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to verify patient")
		return
	}

	reading := models.Reading{
		PatientID:    req.PatientID,
		BPM:          req.BPM,
		SPO2:         req.SPO2,
		Temp:         req.Temp,
		ECGRaw:       req.ECGRaw,
		GlucoseLevel: req.GlucoseLevel,
		IsUrgent:     services.IsUrgent(req),
	}

	if err := h.db.Create(&reading).Error; err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to store reading")
		return
	}

	if reading.IsUrgent {
		if h.jobService != nil {
			if err := h.jobService.EnqueueAIDraft(reading.PatientID); err != nil {
				middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to enqueue ai draft job")
				return
			}
		} else if h.aiService != nil {
			if err := h.aiService.DraftReportForPatient(reading.PatientID); err != nil {
				middleware.JSONError(c, http.StatusInternalServerError, "ai_unavailable", "urgent reading stored but ai draft failed")
				return
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          reading.ID,
		"is_urgent":   reading.IsUrgent,
		"recorded_at": reading.RecordedAt,
		"message":     "Reading stored successfully",
	})
}

func (h *ReadingHandler) ListByPatient(c *gin.Context) {
	patientID, err := strconv.Atoi(c.Param("patient_id"))
	if err != nil || patientID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "patient_id must be a positive integer")
		return
	}

	var readings []models.Reading
	if err := h.db.Where("patient_id = ?", patientID).Order("recorded_at desc").Find(&readings).Error; err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch readings")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": readings})
}

func (h *ReadingHandler) LatestByPatient(c *gin.Context) {
	patientID, err := strconv.Atoi(c.Param("patient_id"))
	if err != nil || patientID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "patient_id must be a positive integer")
		return
	}

	var reading models.Reading
	if err := h.db.Where("patient_id = ?", patientID).Order("recorded_at desc").First(&reading).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "no readings found for patient")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch latest reading")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": reading})
}
