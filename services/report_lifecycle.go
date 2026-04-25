package services

import "health-go-backend/models"

func IsValidReportStatus(status string) bool {
	switch status {
	case models.ReportStatusPending, models.ReportStatusReviewed, models.ReportStatusApproved:
		return true
	default:
		return false
	}
}

func CanTransitionReportStatus(from, to string) bool {
	if !IsValidReportStatus(from) || !IsValidReportStatus(to) {
		return false
	}

	if from == to && from != models.ReportStatusApproved {
		return true
	}

	switch from {
	case models.ReportStatusPending:
		return to == models.ReportStatusReviewed || to == models.ReportStatusApproved
	case models.ReportStatusReviewed:
		return to == models.ReportStatusApproved
	case models.ReportStatusApproved:
		return false
	default:
		return false
	}
}
