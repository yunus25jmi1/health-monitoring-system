package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"

	"gorm.io/gorm"
)

type AIService struct {
	cfg    config.Config
	db     *gorm.DB
	client *http.Client
}

type providerError struct {
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

func NewAIService(cfg config.Config, db *gorm.DB) *AIService {
	timeout := time.Duration(cfg.LLMTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &AIService{
		cfg: cfg,
		db:  db,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *AIService) DraftReportForPatient(patientID uint) error {
	prompt, err := s.buildPrompt(patientID)
	if err != nil {
		return err
	}

	draft, attempts, err := s.GenerateDraft(prompt)
	if err != nil {
		joined := make([]string, 0, len(attempts))
		for _, a := range attempts {
			joined = append(joined, fmt.Sprintf("%s:%s", a.Provider, a.Reason))
		}
		draft = "AI provider unavailable. Deferred generation. Attempts: " + strings.Join(joined, ", ")
	}

	report := models.Report{
		PatientID: patientID,
		AIDraft:   draft,
		Status:    models.ReportStatusPending,
	}
	if err := s.db.Create(&report).Error; err != nil {
		return err
	}
	return nil
}

func (s *AIService) GenerateDraft(prompt string) (string, []providerError, error) {
	attempts := make([]providerError, 0, 3)

	if draft, err := s.callNIM(prompt); err == nil {
		log.Printf("ai_provider_success provider=nim attempts=%d", len(attempts)+1)
		return draft, attempts, nil
	} else {
		log.Printf("ai_provider_failure provider=nim error=%q", err.Error())
		attempts = append(attempts, providerError{Provider: "nim", Reason: err.Error()})
	}

	if draft, err := s.callGemini(prompt); err == nil {
		log.Printf("ai_provider_success provider=gemini attempts=%d", len(attempts)+1)
		return draft, attempts, nil
	} else {
		log.Printf("ai_provider_failure provider=gemini error=%q", err.Error())
		attempts = append(attempts, providerError{Provider: "gemini", Reason: err.Error()})
	}

	if draft, err := s.callOpenRouter(prompt); err == nil {
		log.Printf("ai_provider_success provider=openrouter attempts=%d", len(attempts)+1)
		return draft, attempts, nil
	} else {
		log.Printf("ai_provider_failure provider=openrouter error=%q", err.Error())
		attempts = append(attempts, providerError{Provider: "openrouter", Reason: err.Error()})
	}

	return "", attempts, errors.New("all providers failed")
}

func (s *AIService) buildPrompt(patientID uint) (string, error) {
	var readings []models.Reading
	since := time.Now().UTC().Add(-24 * time.Hour)
	if err := s.db.Where("patient_id = ? AND recorded_at >= ?", patientID, since).Order("recorded_at desc").Find(&readings).Error; err != nil {
		return "", err
	}
	if len(readings) == 0 {
		return "", errors.New("no readings found in last 24h")
	}

	bpmMin, bpmMax, bpmSum := readings[0].BPM, readings[0].BPM, 0
	spo2Min, spo2Max, spo2Sum := readings[0].SPO2, readings[0].SPO2, 0
	tempMin, tempMax, tempSum := readings[0].Temp, readings[0].Temp, 0.0
	urgentCount := 0
	glucoseCount := 0
	glucoseMin := 0.0
	glucoseMax := 0.0
	glucoseSum := 0.0

	for i, r := range readings {
		if r.IsUrgent {
			urgentCount++
		}
		if r.BPM < bpmMin {
			bpmMin = r.BPM
		}
		if r.BPM > bpmMax {
			bpmMax = r.BPM
		}
		bpmSum += r.BPM

		if r.SPO2 < spo2Min {
			spo2Min = r.SPO2
		}
		if r.SPO2 > spo2Max {
			spo2Max = r.SPO2
		}
		spo2Sum += r.SPO2

		if r.Temp < tempMin {
			tempMin = r.Temp
		}
		if r.Temp > tempMax {
			tempMax = r.Temp
		}
		tempSum += r.Temp

		if r.GlucoseLevel != nil {
			g := *r.GlucoseLevel
			if glucoseCount == 0 || g < glucoseMin {
				glucoseMin = g
			}
			if glucoseCount == 0 || g > glucoseMax {
				glucoseMax = g
			}
			glucoseCount++
			glucoseSum += g
		}

		if i == 0 {
			_ = r
		}
	}

	prompt := fmt.Sprintf("Patient ID: %d\nReadings in 24h: %d\nUrgent flags: %d\nBPM min/max/avg: %d/%d/%.2f\nSpO2 min/max/avg: %d/%d/%.2f\nTemp min/max/avg: %.2f/%.2f/%.2f\n", patientID, len(readings), urgentCount, bpmMin, bpmMax, float64(bpmSum)/float64(len(readings)), spo2Min, spo2Max, float64(spo2Sum)/float64(len(readings)), tempMin, tempMax, tempSum/float64(len(readings)))

	if glucoseCount > 0 {
		prompt += fmt.Sprintf("Glucose min/max/avg: %.2f/%.2f/%.2f\n", glucoseMin, glucoseMax, glucoseSum/float64(glucoseCount))
	} else {
		prompt += "Glucose min/max/avg: unavailable\n"
	}
	prompt += "Generate concise Markdown with sections: Summary, Vitals Analysis, Glucose Trend, Clinical Observations, Recommended Actions."

	return prompt, nil
}

func (s *AIService) callNIM(prompt string) (string, error) {
	if strings.TrimSpace(s.cfg.NIMAPIKey) == "" {
		return "", errors.New("missing key")
	}
	payload := map[string]any{
		"model":    s.cfg.NIMModel,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	return s.callJSONEndpoint(context.Background(), s.cfg.NIMAPIURL, s.cfg.NIMAPIKey, payload)
}

func (s *AIService) callGemini(prompt string) (string, error) {
	if strings.TrimSpace(s.cfg.GeminiAPIKey) == "" {
		return "", errors.New("missing key")
	}
	url := strings.TrimRight(s.cfg.GeminiAPIURL, "/") + "/" + s.cfg.GeminiModel + ":generateContent?key=" + s.cfg.GeminiAPIKey
	payload := map[string]any{
		"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}},
	}
	return s.callJSONEndpoint(context.Background(), url, "", payload)
}

func (s *AIService) callOpenRouter(prompt string) (string, error) {
	if strings.TrimSpace(s.cfg.OpenRouterKey) == "" {
		return "", errors.New("missing key")
	}
	payload := map[string]any{
		"model":    s.cfg.OpenRouterModel,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	return s.callJSONEndpoint(context.Background(), s.cfg.OpenRouterURL, s.cfg.OpenRouterKey, payload)
}

func (s *AIService) callJSONEndpoint(ctx context.Context, url, apiKey string, payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	text := extractText(respBody)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("empty response")
	}
	return text, nil
}

func extractText(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if val, ok := payload["text"].(string); ok {
		return val
	}
	if choices, ok := payload["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
		}
	}
	if cands, ok := payload["candidates"].([]any); ok && len(cands) > 0 {
		if cand, ok := cands[0].(map[string]any); ok {
			if content, ok := cand["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok && len(parts) > 0 {
					if first, ok := parts[0].(map[string]any); ok {
						if text, ok := first["text"].(string); ok {
							return text
						}
					}
				}
			}
		}
	}
	return ""
}
