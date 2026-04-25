package services

import (
	"errors"
	"strings"
	"time"

	"health-go-backend/config"
	"health-go-backend/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrInvalidRole        = errors.New("invalid_role")
	ErrInvalidToken       = errors.New("invalid_token")
)

type TokenClaims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

func HashPassword(plain string) (string, error) {
	trimmed := strings.TrimSpace(plain)
	if len(trimmed) < 8 {
		return "", ErrInvalidCredentials
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(trimmed), 12)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPassword(hashed, plain string) error {
	trimmed := strings.TrimSpace(plain)
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(trimmed)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func GenerateAccessToken(cfg config.Config, user models.User) (string, error) {
	return generateToken(cfg, user, "access", time.Duration(cfg.AccessTokenH)*time.Hour)
}

func GenerateRefreshToken(cfg config.Config, user models.User) (string, error) {
	return generateToken(cfg, user, "refresh", time.Duration(cfg.RefreshTokenD)*24*time.Hour)
}

func generateToken(cfg config.Config, user models.User, tokenType string, ttl time.Duration) (string, error) {
	if !models.IsValidRole(user.Role) {
		return "", ErrInvalidRole
	}

	now := time.Now().UTC()
	claims := TokenClaims{
		UserID: user.ID,
		Role:   user.Role,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "smart-health-user",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

func ParseToken(cfg config.Config, tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
