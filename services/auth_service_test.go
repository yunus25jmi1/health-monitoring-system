package services

import (
	"testing"

	"health-go-backend/config"
	"health-go-backend/models"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hash, err := HashPassword("my-strong-pass")
	if err != nil {
		t.Fatalf("expected hash success, got error: %v", err)
	}

	if err := CheckPassword(hash, "my-strong-pass"); err != nil {
		t.Fatalf("expected password match, got error: %v", err)
	}

	if err := CheckPassword(hash, "wrong-pass"); err == nil {
		t.Fatalf("expected password mismatch error")
	}
}

func TestGenerateAndParseToken(t *testing.T) {
	cfg := config.Config{JWTSecret: "test-secret", AccessTokenH: 8, RefreshTokenD: 7}
	user := models.User{ID: 10, Role: models.RoleDoctor}

	access, err := GenerateAccessToken(cfg, user)
	if err != nil {
		t.Fatalf("expected access token generation, got error: %v", err)
	}

	claims, err := ParseToken(cfg, access)
	if err != nil {
		t.Fatalf("expected valid token parse, got error: %v", err)
	}
	if claims.UserID != user.ID || claims.Role != user.Role || claims.Type != "access" {
		t.Fatalf("unexpected claims parsed: %+v", claims)
	}
}
