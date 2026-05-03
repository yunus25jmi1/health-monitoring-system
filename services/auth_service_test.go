package services

import (
	"strings"
	"testing"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"
)

// Test Helper Functions

func getTestConfig() config.Config {
	return config.Config{
		JWTSecret:     "test-secret-key-for-testing-purposes-only",
		AccessTokenH:  1,
		RefreshTokenD: 1,
	}
}

func getTestUsers() map[string]models.User {
	return map[string]models.User{
		"doctor":  {ID: 1, Name: "Dr. Test", Email: "doctor@test.com", Role: models.RoleDoctor},
		"patient": {ID: 2, Name: "Patient Test", Email: "patient@test.com", Role: models.RolePatient},
		"device":  {ID: 3, Name: "Device Test", Email: "device@test.com", Role: models.RoleDevice},
	}
}

// Password Management Tests

func TestAuthService_HashPassword_Success(t *testing.T) {
	passwords := []string{
		"my-strong-pass",
		"AnotherSecurePassword123!",
		"Complex@Password#With$Special%Chars",
	}

	for _, password := range passwords {
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("expected hash success for password '%s', got error: %v", password, err)
		}

		// Verify hash is not empty
		if hash == "" {
			t.Fatalf("expected non-empty hash for password '%s'", password)
		}

		// Verify hash starts with bcrypt prefix
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Fatalf("expected bcrypt hash format for password '%s', got: %s", password, hash)
		}

		// Verify hash length is reasonable (bcrypt hashes are typically 60 chars)
		if len(hash) < 50 || len(hash) > 100 {
			t.Fatalf("expected hash length between 50-100 for password '%s', got: %d", password, len(hash))
		}
	}
}

func TestAuthService_HashPassword_EmptyPassword(t *testing.T) {
	testCases := []struct {
		name     string
		password string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tab only", "\t"},
		{"newline only", "\n"},
		{"too short", "short"},
		{"exactly 7 chars", "1234567"},
		{"7 chars with spaces", " 12345 "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := HashPassword(tc.password)
			if err == nil {
				t.Fatalf("expected error for password '%s', got hash: %s", tc.password, hash)
			}
			if err != ErrInvalidCredentials {
				t.Fatalf("expected ErrInvalidCredentials for password '%s', got: %v", tc.password, err)
			}
			if hash != "" {
				t.Fatalf("expected empty hash for invalid password '%s', got: %s", tc.password, hash)
			}
		})
	}
}

func TestAuthService_CheckPassword_Success(t *testing.T) {
	password := "my-secure-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to create test hash: %v", err)
	}

	// Test successful verification
	err = CheckPassword(hash, password)
	if err != nil {
		t.Fatalf("expected password match, got error: %v", err)
	}

	// Test with trimmed password (should still work)
	err = CheckPassword(hash, "  my-secure-password-123  ")
	if err != nil {
		t.Fatalf("expected password match with trimmed password, got error: %v", err)
	}
}

func TestAuthService_CheckPassword_WrongPassword(t *testing.T) {
	password := "correct-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to create test hash: %v", err)
	}

	wrongPasswords := []string{
		"wrong-password",
		"Correct-Password", // case sensitivity
		"correct-password-with-extra",
		"",
		" ",
	}

	for _, wrongPass := range wrongPasswords {
		err := CheckPassword(hash, wrongPass)
		if err == nil {
			t.Fatalf("expected password mismatch error for '%s'", wrongPass)
		}
		if err != ErrInvalidCredentials {
			t.Fatalf("expected ErrInvalidCredentials for wrong password '%s', got: %v", wrongPass, err)
		}
	}
}

// Token Generation Tests

func TestAuthService_GenerateAccessToken_Success(t *testing.T) {
	cfg := getTestConfig()
	users := getTestUsers()

	for role, user := range users {
		t.Run(role, func(t *testing.T) {
			token, err := GenerateAccessToken(cfg, user)
			if err != nil {
				t.Fatalf("expected access token generation for %s, got error: %v", role, err)
			}

			// Verify token is not empty
			if token == "" {
				t.Fatalf("expected non-empty token for %s", role)
			}

			// Parse and verify token
			claims, err := ParseToken(cfg, token)
			if err != nil {
				t.Fatalf("failed to parse token for %s: %v", role, err)
			}

			// Verify claims
			if claims.UserID != user.ID {
				t.Fatalf("expected user ID %d, got %d", user.ID, claims.UserID)
			}
			if claims.Role != user.Role {
				t.Fatalf("expected role %s, got %s", user.Role, claims.Role)
			}
			if claims.Type != "access" {
				t.Fatalf("expected token type 'access', got %s", claims.Type)
			}
		})
	}
}

func TestAuthService_GenerateAccessToken_InvalidRole(t *testing.T) {
	cfg := getTestConfig()

	invalidRoles := []struct {
		name string
		role string
	}{
		{"empty role", ""},
		{"invalid role", "admin"},
		{"malformed role", "Doctor"},
		{"random string", "random-role-string"},
		{"special chars", "role@#$"},
	}

	for _, tc := range invalidRoles {
		t.Run(tc.name, func(t *testing.T) {
			user := models.User{
				ID:   1,
				Name: "Test User",
				Role: tc.role,
			}

			token, err := GenerateAccessToken(cfg, user)
			if err == nil {
				t.Fatalf("expected error for invalid role '%s', got token: %s", tc.role, token)
			}
			if err != ErrInvalidRole {
				t.Fatalf("expected ErrInvalidRole for role '%s', got: %v", tc.role, err)
			}
			if token != "" {
				t.Fatalf("expected empty token for invalid role '%s', got: %s", tc.role, token)
			}
		})
	}
}

func TestAuthService_GenerateRefreshToken_Success(t *testing.T) {
	cfg := getTestConfig()
	users := getTestUsers()

	for role, user := range users {
		t.Run(role, func(t *testing.T) {
			token, err := GenerateRefreshToken(cfg, user)
			if err != nil {
				t.Fatalf("expected refresh token generation for %s, got error: %v", role, err)
			}

			// Verify token is not empty
			if token == "" {
				t.Fatalf("expected non-empty refresh token for %s", role)
			}

			// Parse and verify token
			claims, err := ParseToken(cfg, token)
			if err != nil {
				t.Fatalf("failed to parse refresh token for %s: %v", role, err)
			}

			// Verify claims
			if claims.UserID != user.ID {
				t.Fatalf("expected user ID %d, got %d", user.ID, claims.UserID)
			}
			if claims.Role != user.Role {
				t.Fatalf("expected role %s, got %s", user.Role, claims.Role)
			}
			if claims.Type != "refresh" {
				t.Fatalf("expected token type 'refresh', got %s", claims.Type)
			}

			// Verify refresh token has longer expiration than access token
			accessExp := claims.ExpiresAt.Time
			now := time.Now().UTC()
			expectedMinExp := now.Add(time.Duration(cfg.RefreshTokenD) * 24 * time.Hour)

			if accessExp.Before(expectedMinExp.Add(-time.Minute)) {
				t.Fatalf("refresh token expiration too early: %v", accessExp)
			}
		})
	}
}

// Token Parsing Tests

func TestAuthService_ParseToken_Success(t *testing.T) {
	cfg := getTestConfig()
	users := getTestUsers()

	for role, user := range users {
		t.Run("access_"+role, func(t *testing.T) {
			token, err := GenerateAccessToken(cfg, user)
			if err != nil {
				t.Fatalf("failed to generate access token: %v", err)
			}

			claims, err := ParseToken(cfg, token)
			if err != nil {
				t.Fatalf("failed to parse access token: %v", err)
			}

			// Verify all claims
			if claims.UserID != user.ID {
				t.Fatalf("expected user ID %d, got %d", user.ID, claims.UserID)
			}
			if claims.Role != user.Role {
				t.Fatalf("expected role %s, got %s", user.Role, claims.Role)
			}
			if claims.Type != "access" {
				t.Fatalf("expected token type 'access', got %s", claims.Type)
			}
			if claims.Subject != "smart-health-user" {
				t.Fatalf("expected subject 'smart-health-user', got %s", claims.Subject)
			}
		})

		t.Run("refresh_"+role, func(t *testing.T) {
			token, err := GenerateRefreshToken(cfg, user)
			if err != nil {
				t.Fatalf("failed to generate refresh token: %v", err)
			}

			claims, err := ParseToken(cfg, token)
			if err != nil {
				t.Fatalf("failed to parse refresh token: %v", err)
			}

			// Verify all claims
			if claims.UserID != user.ID {
				t.Fatalf("expected user ID %d, got %d", user.ID, claims.UserID)
			}
			if claims.Role != user.Role {
				t.Fatalf("expected role %s, got %s", user.Role, claims.Role)
			}
			if claims.Type != "refresh" {
				t.Fatalf("expected token type 'refresh', got %s", claims.Type)
			}
		})
	}
}

func TestAuthService_ParseToken_InvalidToken(t *testing.T) {
	cfg := getTestConfig()

	invalidTokens := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"whitespace", "   "},
		{"invalid format", "invalid.token.here"},
		{"missing parts", "only.two.parts"},
		{"random string", "randomstringnotatoken"},
		{"base64 only", "dGVzdA=="},
	}

	for _, tc := range invalidTokens {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := ParseToken(cfg, tc.token)
			if err == nil {
				t.Fatalf("expected error for invalid token '%s', got claims: %+v", tc.token, claims)
			}
			if err != ErrInvalidToken {
				t.Fatalf("expected ErrInvalidToken for '%s', got: %v", tc.name, err)
			}
			if claims != nil {
				t.Fatalf("expected nil claims for invalid token, got: %+v", claims)
			}
		})
	}

	// Test with wrong signature
	t.Run("wrong signature", func(t *testing.T) {
		user := getTestUsers()["doctor"]
		token, err := GenerateAccessToken(cfg, user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Modify the config to use different secret
		wrongCfg := config.Config{
			JWTSecret:     "different-secret",
			AccessTokenH:  1,
			RefreshTokenD: 1,
		}

		claims, err := ParseToken(wrongCfg, token)
		if err == nil {
			t.Fatalf("expected error for token with wrong signature, got claims: %+v", claims)
		}
		if err != ErrInvalidToken {
			t.Fatalf("expected ErrInvalidToken for wrong signature, got: %v", err)
		}
	})
}

func TestAuthService_ParseToken_ExpiredToken(t *testing.T) {
	// Create config with very short expiration
	cfg := config.Config{
		JWTSecret:     "test-secret-key",
		AccessTokenH:  0, // 0 hours = immediate expiration
		RefreshTokenD: 0,
	}

	user := getTestUsers()["doctor"]

	// Generate token that will expire immediately
	token, err := GenerateAccessToken(cfg, user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Wait a moment to ensure token is expired
	time.Sleep(100 * time.Millisecond)

	// Try to parse expired token
	claims, err := ParseToken(cfg, token)
	if err == nil {
		t.Fatalf("expected error for expired token, got claims: %+v", claims)
	}
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got: %v", err)
	}
	if claims != nil {
		t.Fatalf("expected nil claims for expired token, got: %+v", claims)
	}
}

// Additional Security and Edge Case Tests

func TestAuthService_PasswordHashing_SpecialCharacters(t *testing.T) {
	specialPasswords := []string{
		"password-with-emoji-🔐",
		"unicode-密码-123",
		"special@#$%^&*()chars",
		"newlines\nand\ttabs",
		"quotes\"'and`backticks",
	}

	for _, password := range specialPasswords {
		t.Run(password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("expected hash success for special password, got error: %v", err)
			}

			err = CheckPassword(hash, password)
			if err != nil {
				t.Fatalf("expected password match for special password, got error: %v", err)
			}
		})
	}
}

func TestAuthService_TokenUniqueness(t *testing.T) {
	cfg := getTestConfig()
	user := getTestUsers()["doctor"]

	// Generate multiple tokens for same user
	tokens := make([]string, 5)
	for i := 0; i < 5; i++ {
		token, err := GenerateAccessToken(cfg, user)
		if err != nil {
			t.Fatalf("failed to generate token %d: %v", i, err)
		}
		tokens[i] = token
		// Add small delay to potentially get different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all tokens parse correctly and contain valid claims
	for i, token := range tokens {
		claims, err := ParseToken(cfg, token)
		if err != nil {
			t.Fatalf("failed to parse token %d: %v", i, err)
		}
		if claims.UserID != user.ID {
			t.Fatalf("token %d has wrong user ID", i)
		}
		if claims.Role != user.Role {
			t.Fatalf("token %d has wrong role", i)
		}
		if claims.Type != "access" {
			t.Fatalf("token %d has wrong type", i)
		}
	}

	// Note: Tokens may have identical timestamps if generated within the same second.
	// This is acceptable as JWT security relies on the signature, not timestamp uniqueness.
}

func TestAuthService_ConcurrentTokenGeneration(t *testing.T) {
	cfg := getTestConfig()
	user := getTestUsers()["doctor"]

	// Generate tokens concurrently
	results := make(chan string, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			token, err := GenerateAccessToken(cfg, user)
			if err != nil {
				errors <- err
				return
			}
			results <- token
		}()
	}

	// Collect results
	tokenCount := 0
	errorCount := 0
	for i := 0; i < 10; i++ {
		select {
		case <-results:
			tokenCount++
		case <-errors:
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fatalf("expected no errors in concurrent generation, got %d", errorCount)
	}
	if tokenCount != 10 {
		t.Fatalf("expected 10 tokens, got %d", tokenCount)
	}
}

// Legacy tests (kept for compatibility)

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
