package models

import "time"

type Reading struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	PatientID    uint       `gorm:"index:idx_readings_patient_recorded,priority:1;not null" json:"patient_id"`
	BPM          int        `gorm:"not null" json:"bpm"`
	SPO2         int        `gorm:"not null" json:"spo2"`
	Temp         float64    `gorm:"type:decimal(5,2);not null" json:"temp"`
	ECGRaw       *int       `json:"ecg_raw,omitempty"`
	GlucoseLevel *float64   `gorm:"type:decimal(6,2)" json:"glucose_level,omitempty"`
	IsUrgent     bool       `gorm:"default:false" json:"is_urgent"`
	RecordedAt   *time.Time `gorm:"index:idx_readings_patient_recorded,priority:2;autoCreateTime" json:"recorded_at"`
}

type ReadingRequest struct {
	PatientID    uint     `json:"patient_id" binding:"required"`
	BPM          int      `json:"bpm" binding:"required"`
	SPO2         int      `json:"spo2" binding:"required"`
	Temp         float64  `json:"temp" binding:"required"`
	ECGRaw       *int     `json:"ecg_raw"`
	GlucoseLevel *float64 `json:"glucose_level"`
}
