package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"health-go-backend/config"
	"health-go-backend/middleware"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ReportHandler struct {
	cfg        config.Config
	db         *gorm.DB
	pdfService *services.PDFService
	jobService *services.AsyncJobService
}

type patchReportRequest struct {
	FinalNotes *string `json:"final_notes"`
	Status     *string `json:"status"`
}

func NewReportHandler(cfg config.Config, db *gorm.DB, pdfService *services.PDFService, jobService *services.AsyncJobService) *ReportHandler {
	return &ReportHandler{cfg: cfg, db: db, pdfService: pdfService, jobService: jobService}
}

type ReportResponse struct {
	models.Report
	PatientName string `json:"patient_name"`
	IsUrgent    bool   `json:"is_urgent"`
}

func (h *ReportHandler) Pending(c *gin.Context) {
	authUserID, _ := c.Get("auth_user_id")
	uid := authUserID.(uint)

	var reports []models.Report
	if err := h.db.Joins("JOIN users ON users.id = reports.patient_id").
		Where("reports.status = ? AND users.doctor_id = ?", models.ReportStatusPending, uid).
		Order("reports.created_at asc").
		Find(&reports).Error; err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch pending reports")
		return
	}

	response := make([]ReportResponse, 0, len(reports))
	for _, r := range reports {
		var patient models.User
		h.db.First(&patient, r.PatientID)

		var latestReading models.Reading
		h.db.Where("patient_id = ?", r.PatientID).Order("recorded_at desc").First(&latestReading)

		response = append(response, ReportResponse{
			Report:      r,
			PatientName: patient.Name,
			IsUrgent:    latestReading.IsUrgent,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *ReportHandler) GetByID(c *gin.Context) {
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil || reportID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "report id must be a positive integer")
		return
	}

	var report models.Report
	if err := h.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "report not found")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch report")
		return
	}

	authRole, _ := c.Get("auth_role")
	authUserID, _ := c.Get("auth_user_id")
	if role, ok := authRole.(string); ok && role == models.RoleDoctor {
		uid := authUserID.(uint)
		var patient models.User
		if err := h.db.First(&patient, report.PatientID).Error; err == nil {
			if patient.DoctorID == nil || *patient.DoctorID != uid {
				middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (h *ReportHandler) Patch(c *gin.Context) {
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil || reportID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "report id must be a positive integer")
		return
	}

	var req patchReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "invalid request payload")
		return
	}

	var report models.Report
	if err := h.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "report not found")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch report")
		return
	}

	authUserID, _ := c.Get("auth_user_id")
	uid := authUserID.(uint)
	var patient models.User
	if err := h.db.First(&patient, report.PatientID).Error; err == nil {
		if patient.DoctorID == nil || *patient.DoctorID != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
			return
		}
	}

	if req.FinalNotes != nil {
		report.FinalNote = req.FinalNotes
	}
	if req.Status != nil {
		toStatus := *req.Status
		if !services.CanTransitionReportStatus(report.Status, toStatus) {
			middleware.JSONError(c, http.StatusConflict, "conflict", "invalid report status transition")
			return
		}

		result := h.db.Model(&report).
			Where("id = ? AND status = ?", report.ID, report.Status).
			Updates(map[string]any{"status": toStatus, "final_note": report.FinalNote})
		if result.Error != nil {
			middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update report")
			return
		}
		if result.RowsAffected == 0 {
			middleware.JSONError(c, http.StatusConflict, "conflict", "report status changed by another request")
			return
		}

		if err := h.db.First(&report, reportID).Error; err != nil {
			middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reload report")
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": report})
		return
	}

	result := h.db.Model(&report).Where("id = ?", report.ID).Updates(map[string]any{"final_note": report.FinalNote})
	if result.Error != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update report")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (h *ReportHandler) Approve(c *gin.Context) {
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil || reportID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "report id must be a positive integer")
		return
	}

	var report models.Report
	if err := h.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "report not found")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch report")
		return
	}

	authUserID, _ := c.Get("auth_user_id")
	uid := authUserID.(uint)
	var patient models.User
	if err := h.db.First(&patient, report.PatientID).Error; err == nil {
		if patient.DoctorID == nil || *patient.DoctorID != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
			return
		}
	}

	if !services.CanTransitionReportStatus(report.Status, models.ReportStatusApproved) {
		middleware.JSONError(c, http.StatusConflict, "conflict", "report must be reviewed before approval")
		return
	}

	now := time.Now().UTC()
	base := filepath.Join(h.cfg.PDFStorage, strconv.Itoa(reportID))
	fromStatus := report.Status
	report.Status = models.ReportStatusApproved
	report.ApprovedAt = &now
	report.PDFPath = &base

	if doctorIDValue, ok := c.Get("auth_user_id"); ok {
		if doctorID, ok := doctorIDValue.(uint); ok {
			report.DoctorID = &doctorID
		}
	}

	updates := map[string]any{
		"status":      models.ReportStatusApproved,
		"approved_at": now,
		"pdf_path":    base,
	}
	if doctorIDValue, ok := c.Get("auth_user_id"); ok {
		if doctorID, ok := doctorIDValue.(uint); ok {
			updates["doctor_id"] = doctorID
			report.DoctorID = &doctorID
		}
	}

	result := h.db.Model(&report).
		Where("id = ? AND status = ?", report.ID, fromStatus).
		Updates(updates)
	if result.Error != nil {
		if strings.Contains(strings.ToLower(result.Error.Error()), "locked") {
			middleware.JSONError(c, http.StatusConflict, "conflict", "report approval is in progress by another request")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to approve report")
		return
	}
	if result.RowsAffected == 0 {
		middleware.JSONError(c, http.StatusConflict, "conflict", "report already approved by another request")
		return
	}

	if err := h.db.First(&report, report.ID).Error; err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reload approved report")
		return
	}

	if h.jobService != nil {
		if err := h.jobService.EnqueuePDFGeneration(report.ID); err != nil {
			middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "report approved but pdf job enqueue failed")
			return
		}
	} else if h.pdfService != nil {
		if _, _, err := h.pdfService.GenerateDualCopy(report); err != nil {
			middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "report approved but pdf generation failed")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "report approved",
		"data":    report,
	})
}

func (h *ReportHandler) StreamPDF(c *gin.Context) {
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil || reportID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "report id must be a positive integer")
		return
	}

	var report models.Report
	if err := h.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "report not found")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch report")
		return
	}

	if report.Status != models.ReportStatusApproved {
		middleware.JSONError(c, http.StatusConflict, "conflict", "report PDF is available only after approval")
		return
	}

	authRole, _ := c.Get("auth_role")
	authUserID, _ := c.Get("auth_user_id")
	if role, ok := authRole.(string); ok && role == models.RolePatient {
		uid, ok := authUserID.(uint)
		if !ok || uid != report.PatientID {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "patients can only access their own reports")
			return
		}
	} else if role, ok := authRole.(string); ok && role == models.RoleDoctor {
		uid := authUserID.(uint)
		var patient models.User
		if err := h.db.First(&patient, report.PatientID).Error; err == nil {
			if patient.DoctorID == nil || *patient.DoctorID != uid {
				middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
				return
			}
		}
	}

	copyType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("copy", "patient")))
	if copyType != "patient" && copyType != "clinical" {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "copy must be patient or clinical")
		return
	}

	basePath := filepath.Join(h.cfg.PDFStorage, strconv.Itoa(reportID))
	if report.PDFPath != nil && strings.TrimSpace(*report.PDFPath) != "" {
		basePath = strings.TrimSpace(*report.PDFPath)
	}

	filePath := basePath + "_" + copyType + ".pdf"
	if _, err := os.Stat(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			middleware.JSONError(c, http.StatusNotFound, "not_found", "pdf file not generated yet")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to access pdf file")
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=report_"+strconv.Itoa(reportID)+"_"+copyType+".pdf")
	c.File(filePath)
}

func (h *ReportHandler) ListByPatient(c *gin.Context) {
	patientID, err := strconv.Atoi(c.Param("patient_id"))
	if err != nil || patientID <= 0 {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "patient_id must be a positive integer")
		return
	}

	authRole, _ := c.Get("auth_role")
	authUserID, _ := c.Get("auth_user_id")
	uid := authUserID.(uint)
	var patient models.User
	if err := h.db.First(&patient, patientID).Error; err != nil {
		middleware.JSONError(c, http.StatusNotFound, "not_found", "patient not found")
		return
	}
	if role, ok := authRole.(string); ok && role == models.RoleDoctor {
		if patient.DoctorID == nil || *patient.DoctorID != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you are not the assigned doctor for this patient")
			return
		}
	} else if role, ok := authRole.(string); ok && role == models.RolePatient {
		if uint(patientID) != uid {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "you can only view your own reports")
			return
		}
	}

	var reports []models.Report
	if err := h.db.Where("patient_id = ?", patientID).Order("created_at desc").Find(&reports).Error; err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to fetch reports")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": reports})
}
