package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort      string
	DBDSN           string
	DeviceSecret    string
	JWTSecret       string
	AccessTokenH    int
	RefreshTokenD   int
	RateLimitRPM    int
	LLMTimeoutSec   int
	NIMAPIURL       string
	NIMAPIKey       string
	NIMModel        string
	GeminiAPIURL    string
	GeminiAPIKey    string
	GeminiModel     string
	OpenRouterURL   string
	OpenRouterKey   string
	OpenRouterModel string
	AllowedOrigin   []string
	PDFStorage      string
}

func LoadConfig() Config {
	_ = godotenv.Load()

	return Config{
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		DBDSN:           getEnv("DB_DSN", "host=localhost user=postgres password=postgres dbname=health_monitor port=5432 sslmode=disable TimeZone=UTC"),
		DeviceSecret:    getEnv("DEVICE_SECRET_KEY", "change-me"),
		JWTSecret:       getEnv("JWT_SECRET", "dev-jwt-secret"),
		AccessTokenH:    getEnvInt("JWT_ACCESS_HOURS", 8),
		RefreshTokenD:   getEnvInt("JWT_REFRESH_DAYS", 7),
		RateLimitRPM:    getEnvInt("RATE_LIMIT_RPM", 100),
		LLMTimeoutSec:   getEnvInt("LLM_TIMEOUT_SECONDS", 20),
		NIMAPIURL:       getEnv("NIM_API_URL", "https://integrate.api.nvidia.com/v1/chat/completions"),
		NIMAPIKey:       getEnv("NIM_API_KEY", ""),
		NIMModel:        getEnv("NIM_MODEL", "meta/llama-3.1-8b-instruct"),
		GeminiAPIURL:    getEnv("GEMINI_API_URL", "https://generativelanguage.googleapis.com/v1beta/models"),
		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
		GeminiModel:     getEnv("GEMINI_MODEL", "gemini-1.5-flash"),
		OpenRouterURL:   getEnv("OPENROUTER_API_URL", "https://openrouter.ai/api/v1/chat/completions"),
		OpenRouterKey:   getEnv("OPENROUTER_API_KEY", ""),
		OpenRouterModel: getEnv("OPENROUTER_MODEL", "openai/gpt-4o-mini"),
		AllowedOrigin:   splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
		PDFStorage:      getEnv("PDF_STORAGE_PATH", "./storage/reports"),
	}
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func splitCSV(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}
