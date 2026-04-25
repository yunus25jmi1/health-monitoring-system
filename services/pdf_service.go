package services

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"

	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

type PDFService struct {
	cfg config.Config
	db  *gorm.DB
}

type vitalSummary struct {
	HasReadings   bool
	ReadingsCount int
	UrgentCount   int
	BPMMin        int
	BPMMax        int
	BPMAvg        float64
	SPO2Min       int
	SPO2Max       int
	SPO2Avg       float64
	TempMin       float64
	TempMax       float64
	TempAvg       float64
	HasGlucose    bool
	GlucoseMin    float64
	GlucoseMax    float64
	GlucoseAvg    float64
}

type pdfEnrichment struct {
	DoctorName  string
	DoctorEmail string
	Vitals      vitalSummary
}

func NewPDFService(cfg config.Config, db *gorm.DB) *PDFService {
	return &PDFService{cfg: cfg, db: db}
}

func (s *PDFService) GenerateDualCopy(report models.Report) (string, string, error) {
	if err := os.MkdirAll(s.cfg.PDFStorage, 0o755); err != nil {
		return "", "", err
	}

	enrichment, err := s.buildEnrichment(report)
	if err != nil {
		return "", "", err
	}

	baseName := filepath.Join(s.cfg.PDFStorage, fmt.Sprintf("%d", report.ID))
	patientPath := baseName + "_patient.pdf"
	clinicalPath := baseName + "_clinical.pdf"

	if err := s.generateSinglePDF(report, enrichment, patientPath, false); err != nil {
		return "", "", err
	}
	if err := s.generateSinglePDF(report, enrichment, clinicalPath, true); err != nil {
		return "", "", err
	}

	return patientPath, clinicalPath, nil
}

func (s *PDFService) buildEnrichment(report models.Report) (pdfEnrichment, error) {
	result := pdfEnrichment{
		DoctorName:  "Not assigned",
		DoctorEmail: "N/A",
	}

	if s.db == nil {
		return result, nil
	}

	if report.DoctorID != nil {
		var doctor models.User
		err := s.db.Where("id = ? AND role = ?", *report.DoctorID, models.RoleDoctor).First(&doctor).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return result, err
		}
		if err == nil {
			result.DoctorName = doctor.Name
			result.DoctorEmail = doctor.Email
		}
	}

	since := time.Now().UTC().Add(-24 * time.Hour)
	var readings []models.Reading
	if err := s.db.Where("patient_id = ? AND recorded_at >= ?", report.PatientID, since).Order("recorded_at asc").Find(&readings).Error; err != nil {
		return result, err
	}
	if len(readings) == 0 {
		return result, nil
	}

	v := vitalSummary{
		HasReadings:   true,
		ReadingsCount: len(readings),
		BPMMin:        readings[0].BPM,
		BPMMax:        readings[0].BPM,
		SPO2Min:       readings[0].SPO2,
		SPO2Max:       readings[0].SPO2,
		TempMin:       readings[0].Temp,
		TempMax:       readings[0].Temp,
	}

	bpmSum := 0
	spo2Sum := 0
	tempSum := 0.0
	glucoseSum := 0.0
	glucoseCount := 0

	for _, r := range readings {
		if r.IsUrgent {
			v.UrgentCount++
		}

		if r.BPM < v.BPMMin {
			v.BPMMin = r.BPM
		}
		if r.BPM > v.BPMMax {
			v.BPMMax = r.BPM
		}
		bpmSum += r.BPM

		if r.SPO2 < v.SPO2Min {
			v.SPO2Min = r.SPO2
		}
		if r.SPO2 > v.SPO2Max {
			v.SPO2Max = r.SPO2
		}
		spo2Sum += r.SPO2

		if r.Temp < v.TempMin {
			v.TempMin = r.Temp
		}
		if r.Temp > v.TempMax {
			v.TempMax = r.Temp
		}
		tempSum += r.Temp

		if r.GlucoseLevel != nil {
			g := *r.GlucoseLevel
			if !v.HasGlucose || g < v.GlucoseMin {
				v.GlucoseMin = g
			}
			if !v.HasGlucose || g > v.GlucoseMax {
				v.GlucoseMax = g
			}
			v.HasGlucose = true
			glucoseSum += g
			glucoseCount++
		}
	}

	v.BPMAvg = float64(bpmSum) / float64(len(readings))
	v.SPO2Avg = float64(spo2Sum) / float64(len(readings))
	v.TempAvg = tempSum / float64(len(readings))
	if glucoseCount > 0 {
		v.GlucoseAvg = glucoseSum / float64(glucoseCount)
	}

	result.Vitals = v
	return result, nil
}

func (s *PDFService) generateSinglePDF(report models.Report, enrichment pdfEnrichment, outputPath string, watermark bool) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Smart Health Report", false)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Smart Health Monitoring Report")
	pdf.Ln(12)

	if watermark {
		pdf.SetTextColor(220, 220, 220)
		pdf.SetFont("Arial", "B", 28)
		pdf.TransformBegin()
		pdf.TransformRotate(45, 105, 148)
		pdf.Text(35, 148, "CONFIDENTIAL CLINICAL RECORD")
		pdf.TransformEnd()
		pdf.SetTextColor(0, 0, 0)
	}

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Report ID: %d", report.ID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Patient ID: %d", report.PatientID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Status: %s", report.Status))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Doctor: %s (%s)", enrichment.DoctorName, enrichment.DoctorEmail))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Generated At: %s", time.Now().UTC().Format(time.RFC3339)))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "24h Vitals Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	if !enrichment.Vitals.HasReadings {
		pdf.Cell(0, 6, "No readings found in the last 24 hours.")
		pdf.Ln(8)
	} else {
		v := enrichment.Vitals
		pdf.Cell(0, 6, fmt.Sprintf("Readings: %d, Urgent Flags: %d", v.ReadingsCount, v.UrgentCount))
		pdf.Ln(6)
		pdf.Cell(0, 6, fmt.Sprintf("BPM min/max/avg: %d / %d / %.2f", v.BPMMin, v.BPMMax, v.BPMAvg))
		pdf.Ln(6)
		pdf.Cell(0, 6, fmt.Sprintf("SpO2 min/max/avg: %d / %d / %.2f", v.SPO2Min, v.SPO2Max, v.SPO2Avg))
		pdf.Ln(6)
		pdf.Cell(0, 6, fmt.Sprintf("Temp min/max/avg: %.2f / %.2f / %.2f", v.TempMin, v.TempMax, v.TempAvg))
		pdf.Ln(6)
		if v.HasGlucose {
			pdf.Cell(0, 6, fmt.Sprintf("Glucose min/max/avg: %.2f / %.2f / %.2f", v.GlucoseMin, v.GlucoseMax, v.GlucoseAvg))
		} else {
			pdf.Cell(0, 6, "Glucose min/max/avg: unavailable")
		}
		pdf.Ln(8)
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "AI Draft")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6, report.AIDraft, "", "L", false)
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Doctor Notes")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	notes := "No doctor notes provided."
	if report.FinalNote != nil && *report.FinalNote != "" {
		notes = *report.FinalNote
	}
	pdf.MultiCell(0, 6, notes, "", "L", false)

	return pdf.OutputFileAndClose(outputPath)
}
