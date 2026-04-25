package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"health-go-backend/config"
)

func TestGenerateDraftProviderFallbackOrder(t *testing.T) {
	nim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer nim.Close()

	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer gemini.Close()

	openrouter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"fallback success"}`))
	}))
	defer openrouter.Close()

	svc := &AIService{
		cfg: config.Config{
			LLMTimeoutSec:   5,
			NIMAPIURL:       nim.URL,
			NIMAPIKey:       "nim-key",
			NIMModel:        "nim-model",
			GeminiAPIURL:    gemini.URL,
			GeminiAPIKey:    "gemini-key",
			GeminiModel:     "gemini-model",
			OpenRouterURL:   openrouter.URL,
			OpenRouterKey:   "or-key",
			OpenRouterModel: "or-model",
		},
		client: &http.Client{},
	}

	draft, attempts, err := svc.GenerateDraft("prompt")
	if err != nil {
		t.Fatalf("expected success via openrouter fallback, got error: %v", err)
	}
	if draft != "fallback success" {
		t.Fatalf("expected fallback content, got: %s", draft)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected two failed attempts before success, got: %d", len(attempts))
	}
	if attempts[0].Provider != "nim" || attempts[1].Provider != "gemini" {
		t.Fatalf("unexpected provider order: %+v", attempts)
	}
}

func TestGenerateDraftTimeoutFallback(t *testing.T) {
	nim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"late"}`))
	}))
	defer nim.Close()

	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"gemini-fallback"}`))
	}))
	defer gemini.Close()

	svc := &AIService{
		cfg: config.Config{
			NIMAPIURL:       nim.URL,
			NIMAPIKey:       "nim-key",
			NIMModel:        "nim-model",
			GeminiAPIURL:    gemini.URL,
			GeminiAPIKey:    "gemini-key",
			GeminiModel:     "gemini-model",
			OpenRouterURL:   nim.URL,
			OpenRouterKey:   "or-key",
			OpenRouterModel: "or-model",
		},
		client: &http.Client{Timeout: 50 * time.Millisecond},
	}

	draft, attempts, err := svc.GenerateDraft("prompt")
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if draft != "gemini-fallback" {
		t.Fatalf("expected gemini fallback text, got: %s", draft)
	}
	if len(attempts) != 1 || attempts[0].Provider != "nim" {
		t.Fatalf("expected one failed nim attempt before fallback, got: %+v", attempts)
	}
}

func TestGenerateDraftAllProvidersFail(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failSrv.Close()

	svc := &AIService{
		cfg: config.Config{
			NIMAPIURL:       failSrv.URL,
			NIMAPIKey:       "nim-key",
			NIMModel:        "nim-model",
			GeminiAPIURL:    failSrv.URL,
			GeminiAPIKey:    "gemini-key",
			GeminiModel:     "gemini-model",
			OpenRouterURL:   failSrv.URL,
			OpenRouterKey:   "or-key",
			OpenRouterModel: "or-model",
		},
		client: &http.Client{},
	}

	_, attempts, err := svc.GenerateDraft("prompt")
	if err == nil {
		t.Fatalf("expected all providers failure")
	}
	if len(attempts) != 3 {
		t.Fatalf("expected 3 failure attempts, got: %d", len(attempts))
	}
}
