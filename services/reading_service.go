package services

import (
	"errors"

	"health-go-backend/models"
)

var errInvalidVitals = errors.New("validation_failed")

func ValidateReading(req models.ReadingRequest) error {
	if req.PatientID == 0 {
		return errors.New("patient_id is required")
	}
	if req.BPM < 0 || req.BPM > 300 {
		return errors.New("bpm out of range")
	}
	if req.SPO2 < 0 || req.SPO2 > 100 {
		return errors.New("spo2 out of range")
	}
	if (req.Temp != 0 && req.Temp < 25) || req.Temp > 45 {
		return errors.New("temperature out of range")
	}
	if req.ECGRaw != nil && *req.ECGRaw < -1 {
		return errors.New("ecg_raw out of range")
	}
	if req.GlucoseLevel != nil && (*req.GlucoseLevel < 0 || *req.GlucoseLevel > 600) {
		return errors.New("glucose_level out of range")
	}
	return nil
}

func IsUrgent(req models.ReadingRequest) bool {
	if req.BPM < 50 || req.BPM > 100 {
		return true
	}
	if req.SPO2 > 0 && req.SPO2 < 94 {
		return true
	}
	if req.Temp > 38.0 {
		return true
	}
	if req.GlucoseLevel != nil {
		g := *req.GlucoseLevel
		if g < 70 || g > 180 {
			return true
		}
		if g < 70 && req.BPM > 100 {
			return true
		}
	}
	return false
}
