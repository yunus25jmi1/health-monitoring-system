package handlers

import (
	"errors"
	"net/http"
	"strings"

	"health-go-backend/config"
	"health-go-backend/middleware"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	cfg config.Config
	db  *gorm.DB
}

type registerRequest struct {
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	Role        string `json:"role" binding:"required"`
	DoctorToken string `json:"doctor_token"` // Required only for RoleDoctor
	DeviceKey   string `json:"device_key"`   // Optional for RolePatient
	DoctorID    *uint  `json:"doctor_id"`    // Optional for RolePatient
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func NewAuthHandler(cfg config.Config, db *gorm.DB) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "invalid request payload")
		return
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == models.RoleDoctor {
		regToken := strings.TrimSpace(h.cfg.DoctorRegToken)
		if regToken == "" || strings.TrimSpace(req.DoctorToken) != regToken {
			middleware.JSONError(c, http.StatusForbidden, "forbidden", "invalid or missing doctor registration token")
			return
		}
	} else if role != models.RolePatient {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "role must be patient or doctor")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "email is required")
		return
	}

	hashed, err := services.HashPassword(req.Password)
	if err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "password must be at least 8 characters")
		return
	}

	user := models.User{
		Name:      strings.TrimSpace(req.Name),
		Email:     email,
		Password:  hashed,
		Role:      role,
		DeviceKey: strings.TrimSpace(req.DeviceKey),
		DoctorID:  req.DoctorID,
	}
	if err := h.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			middleware.JSONError(c, http.StatusConflict, "conflict", "email is already registered")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to create account")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.JSONError(c, http.StatusBadRequest, "validation_failed", "invalid request payload")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid email or password")
			return
		}
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to process login")
		return
	}

	if err := services.CheckPassword(user.Password, req.Password); err != nil {
		middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	access, err := services.GenerateAccessToken(h.cfg, user)
	if err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to generate access token")
		return
	}
	refresh, err := services.GenerateRefreshToken(h.cfg, user)
	if err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to generate refresh token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"role":          user.Role,
		"user_id":       user.ID,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(token, "Bearer ") {
		middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}

	claims, err := services.ParseToken(h.cfg, strings.TrimSpace(strings.TrimPrefix(token, "Bearer ")))
	if err != nil || claims.Type != "refresh" {
		middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}

	var user models.User
	if err := h.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		middleware.JSONError(c, http.StatusUnauthorized, "unauthorized", "user not found for refresh token")
		return
	}

	access, err := services.GenerateAccessToken(h.cfg, user)
	if err != nil {
		middleware.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to generate access token")
		return
	}

	c.JSON(http.StatusOK, gin.H{"access_token": access})
}
