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
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "invalid request payload: "+err.Error())
		return
	}

	if err := services.ValidateReading(req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "sensor values out of bounds: "+err.Error())
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

	deviceKey, _ := c.Get("device_key")
	if patient.DeviceKey == "" || patient.DeviceKey != deviceKey.(string) {
		middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid device key for this patient")
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
		// DEDUPLICATION: Only trigger AI if there isn't already a pending report for this patient
		var count int64
		h.db.Model(&models.Report{}).Where("patient_id = ? AND status = ?", reading.PatientID, models.ReportStatusPending).Count(&count)

		if count == 0 {
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

	var patient models.User
	if err := h.db.First(&patient, patientID).Error; err != nil {
		middleware.JSONError(c, http.StatusNotFound, "not_found", "patient not found")
		return
	}

	authRole, _ := c.Get("auth_role")
	authUserID, _ := c.Get("auth_user_id")
	if role, ok := authRole.(string); ok && role == models.RoleDoctor {
		uid := authUserID.(uint)
		if patient.DoctorID == nil || *patient.DoctorID != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
			return
		}
	} else if role, ok := authRole.(string); ok && role == models.RolePatient {
		uid := authUserID.(uint)
		if uint(patientID) != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you can only view your own readings")
			return
		}
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

	var patient models.User
	if err := h.db.First(&patient, patientID).Error; err != nil {
		middleware.JSONError(c, http.StatusNotFound, "not_found", "patient not found")
		return
	}

	authRole, _ := c.Get("auth_role")
	authUserID, _ := c.Get("auth_user_id")
	if role, ok := authRole.(string); ok && role == models.RoleDoctor {
		uid := authUserID.(uint)
		if patient.DoctorID == nil || *patient.DoctorID != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
			return
		}
	} else if role, ok := authRole.(string); ok && role == models.RolePatient {
		uid := authUserID.(uint)
		if uint(patientID) != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you can only view your own readings")
			return
		}
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
