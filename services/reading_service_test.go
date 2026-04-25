package services

import (
	"testing"

	"health-go-backend/models"
)

func TestValidateReading(t *testing.T) {
	validGlucose := 120.0
	valid := models.ReadingRequest{
		PatientID:    1,
		BPM:          88,
		SPO2:         97,
		Temp:         36.8,
		GlucoseLevel: &validGlucose,
	}

	if err := ValidateReading(valid); err != nil {
		t.Fatalf("expected valid reading, got error: %v", err)
	}

	invalid := valid
	invalid.BPM = 301
	if err := ValidateReading(invalid); err == nil {
		t.Fatalf("expected error for invalid BPM")
	}

	invalid = valid
	invalid.SPO2 = 101
	if err := ValidateReading(invalid); err == nil {
		t.Fatalf("expected error for invalid SpO2")
	}

	invalid = valid
	invalid.Temp = 46
	if err := ValidateReading(invalid); err == nil {
		t.Fatalf("expected error for invalid temperature")
	}
}

func TestIsUrgentEdgeCases(t *testing.T) {
	glucoseLow := 69.0
	glucoseHigh := 181.0
	glucoseNormal := 100.0

	cases := []struct {
		name string
		req  models.ReadingRequest
		want bool
	}{
		{
			name: "normal values",
			req:  models.ReadingRequest{PatientID: 1, BPM: 80, SPO2: 98, Temp: 36.7, GlucoseLevel: &glucoseNormal},
			want: false,
		},
		{
			name: "bpm high boundary breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 101, SPO2: 98, Temp: 36.7, GlucoseLevel: &glucoseNormal},
			want: true,
		},
		{
			name: "spo2 low breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 85, SPO2: 93, Temp: 36.7, GlucoseLevel: &glucoseNormal},
			want: true,
		},
		{
			name: "fever breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 85, SPO2: 98, Temp: 38.1, GlucoseLevel: &glucoseNormal},
			want: true,
		},
		{
			name: "glucose low breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 85, SPO2: 98, Temp: 36.7, GlucoseLevel: &glucoseLow},
			want: true,
		},
		{
			name: "glucose high breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 85, SPO2: 98, Temp: 36.7, GlucoseLevel: &glucoseHigh},
			want: true,
		},
		{
			name: "hypoglycemic combo breach",
			req:  models.ReadingRequest{PatientID: 1, BPM: 110, SPO2: 98, Temp: 36.7, GlucoseLevel: &glucoseLow},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsUrgent(tc.req)
			if got != tc.want {
				t.Fatalf("IsUrgent() = %v, want %v", got, tc.want)
			}
		})
	}
}
