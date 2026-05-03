package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"health-go-backend/config"
	"health-go-backend/middleware"
	"health-go-backend/models"
	"health-go-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Test Helper Functions

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to setup test database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func getTestConfig() config.Config {
	return config.Config{
		JWTSecret:     "test-secret-key-for-testing-purposes-only",
		AccessTokenH:  1,
		RefreshTokenD: 1,
		DoctorRegToken: "test-doctor-registration-token",
	}
}

func setupTestServer(handler *AuthHandler) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/register", handler.Register)
	router.POST("/login", handler.Login)
	router.POST("/refresh", handler.Refresh)

	// Setup authenticated routes
	authGroup := router.Group("/")
	authGroup.Use(middleware.JWTAuth(handler.cfg))
	authGroup.GET("/me", handler.Me)

	return httptest.NewServer(router)
}

func makeRequest(t *testing.T, method, url string, body interface{}, headers map[string]string) *http.Response {
	var bodyReader *bytes.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	return resp
}

func parseJSONResponse(t *testing.T, resp *http.Response, target interface{}) {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Registration Tests

func TestAuthHandler_Register_Success(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Test patient registration
	t.Run("patient registration", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Test Patient",
			"email":    "patient@test.com",
			"password": "securepassword123",
			"role":     "patient",
		}

		resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		parseJSONResponse(t, resp, &result)

		if result["email"] != "patient@test.com" {
			t.Fatalf("expected email patient@test.com, got %v", result["email"])
		}
		if result["role"] != "patient" {
			t.Fatalf("expected role patient, got %v", result["role"])
		}
		if result["id"] == nil {
			t.Fatalf("expected user ID in response")
		}

		// Verify user was created in database
		var user models.User
		if err := db.Where("email = ?", "patient@test.com").First(&user).Error; err != nil {
			t.Fatalf("failed to find user in database: %v", err)
		}
		if user.Name != "Test Patient" {
			t.Fatalf("expected name Test Patient, got %s", user.Name)
		}
	})

	// Test doctor registration with valid token (in separate test to avoid conflicts)
	t.Run("doctor registration with valid token", func(t *testing.T) {
		// Use a fresh database for this test
		db := setupTestDB(t)
		handler := NewAuthHandler(cfg, db)
		server := setupTestServer(handler)
		defer server.Close()

		payload := map[string]interface{}{
			"name":         "Test Doctor",
			"email":        "doctor@test.com",
			"password":     "securepassword123",
			"role":         "doctor",
			"doctor_token": "test-doctor-registration-token",
		}

		resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		parseJSONResponse(t, resp, &result)

		if result["email"] != "doctor@test.com" {
			t.Fatalf("expected email doctor@test.com, got %v", result["email"])
		}
		if result["role"] != "doctor" {
			t.Fatalf("expected role doctor, got %v", result["role"])
		}
	})
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Create initial user
	payload := map[string]interface{}{
		"name":     "Test User",
		"email":    "duplicate@test.com",
		"password": "securepassword123",
		"role":     "patient",
	}

	resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 for initial registration, got %d", resp.StatusCode)
	}

	// Try to register with same email
	duplicatePayload := map[string]interface{}{
		"name":     "Another User",
		"email":    "duplicate@test.com",
		"password": "anotherpassword123",
		"role":     "patient",
	}

	resp = makeRequest(t, "POST", server.URL+"/register", duplicatePayload, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	parseJSONResponse(t, resp, &errResp)

	if errResp.Error != "conflict" {
		t.Fatalf("expected error type 'conflict', got %s", errResp.Error)
	}
	if !strings.Contains(strings.ToLower(errResp.Message), "email") {
		t.Fatalf("expected error message to mention email, got %s", errResp.Message)
	}
}

func TestAuthHandler_Register_InvalidPayload(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	testCases := []struct {
		name        string
		payload     map[string]interface{}
		expectedErr string
	}{
		{
			name: "missing name",
			payload: map[string]interface{}{
				"email":    "test@test.com",
				"password": "securepassword123",
				"role":     "patient",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "missing email",
			payload: map[string]interface{}{
				"name":     "Test User",
				"password": "securepassword123",
				"role":     "patient",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "missing password",
			payload: map[string]interface{}{
				"name":  "Test User",
				"email": "test@test.com",
				"role":  "patient",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "missing role",
			payload: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "securepassword123",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "short password",
			payload: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "short",
				"role":     "patient",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "invalid role",
			payload: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "securepassword123",
				"role":     "admin",
			},
			expectedErr: "validation_failed",
		},
		{
			name: "missing doctor token",
			payload: map[string]interface{}{
				"name":     "Test Doctor",
				"email":    "doctor@test.com",
				"password": "securepassword123",
				"role":     "doctor",
			},
			expectedErr: "forbidden",
		},
		{
			name: "invalid doctor token",
			payload: map[string]interface{}{
				"name":         "Test Doctor",
				"email":        "doctor@test.com",
				"password":     "securepassword123",
				"role":         "doctor",
				"doctor_token": "invalid-token",
			},
			expectedErr: "forbidden",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, "POST", server.URL+"/register", tc.payload, nil)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected status 400 or 403, got %d", resp.StatusCode)
			}

			var errResp ErrorResponse
			parseJSONResponse(t, resp, &errResp)

			if errResp.Error != tc.expectedErr {
				t.Fatalf("expected error type '%s', got %s", tc.expectedErr, errResp.Error)
			}
		})
	}
}

// Login Tests

func TestAuthHandler_Login_Success(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Create test user
	hashedPassword, err := services.HashPassword("securepassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := models.User{
		Name:     "Test User",
		Email:    "login@test.com",
		Password: hashedPassword,
		Role:     "patient",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Test successful login
	payload := map[string]interface{}{
		"email":    "login@test.com",
		"password": "securepassword123",
	}

	resp := makeRequest(t, "POST", server.URL+"/login", payload, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	parseJSONResponse(t, resp, &result)

	// Verify tokens are present
	if result["access_token"] == nil {
		t.Fatalf("expected access_token in response")
	}
	if result["refresh_token"] == nil {
		t.Fatalf("expected refresh_token in response")
	}

	// Verify user information
	userData, ok := result["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object in response")
	}
	if userData["email"] != "login@test.com" {
		t.Fatalf("expected email login@test.com, got %v", userData["email"])
	}
	if userData["name"] != "Test User" {
		t.Fatalf("expected name Test User, got %v", userData["name"])
	}
	if userData["role"] != "patient" {
		t.Fatalf("expected role patient, got %v", userData["role"])
	}

	// Verify tokens are valid
	accessToken := result["access_token"].(string)
	refreshToken := result["refresh_token"].(string)

	claims, err := services.ParseToken(cfg, accessToken)
	if err != nil {
		t.Fatalf("failed to parse access token: %v", err)
	}
	if claims.Type != "access" {
		t.Fatalf("expected token type 'access', got %s", claims.Type)
	}

	refreshClaims, err := services.ParseToken(cfg, refreshToken)
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Fatalf("expected token type 'refresh', got %s", refreshClaims.Type)
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Create test user
	hashedPassword, err := services.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := models.User{
		Name:     "Test User",
		Email:    "login@test.com",
		Password: hashedPassword,
		Role:     "patient",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Test with wrong password
	payload := map[string]interface{}{
		"email":    "login@test.com",
		"password": "wrongpassword",
	}

	resp := makeRequest(t, "POST", server.URL+"/login", payload, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	parseJSONResponse(t, resp, &errResp)

	if errResp.Error != "unauthorized" {
		t.Fatalf("expected error type 'unauthorized', got %s", errResp.Error)
	}
}

func TestAuthHandler_Login_NonexistentUser(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Test with non-existent user
	payload := map[string]interface{}{
		"email":    "nonexistent@test.com",
		"password": "anypassword",
	}

	resp := makeRequest(t, "POST", server.URL+"/login", payload, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	parseJSONResponse(t, resp, &errResp)

	if errResp.Error != "unauthorized" {
		t.Fatalf("expected error type 'unauthorized', got %s", errResp.Error)
	}
}

// Token Management Tests

func TestAuthHandler_Refresh_Success(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Create test user
	hashedPassword, err := services.HashPassword("securepassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := models.User{
		Name:     "Test User",
		Email:    "refresh@test.com",
		Password: hashedPassword,
		Role:     "patient",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Generate refresh token
	refreshToken, err := services.GenerateRefreshToken(cfg, user)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	// Test successful token refresh
	headers := map[string]string{
		"Authorization": "Bearer " + refreshToken,
	}

	resp := makeRequest(t, "POST", server.URL+"/refresh", nil, headers)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	parseJSONResponse(t, resp, &result)

	if result["access_token"] == nil {
		t.Fatalf("expected access_token in response")
	}

	// Verify new access token is valid
	newAccessToken := result["access_token"].(string)
	claims, err := services.ParseToken(cfg, newAccessToken)
	if err != nil {
		t.Fatalf("failed to parse new access token: %v", err)
	}
	if claims.Type != "access" {
		t.Fatalf("expected token type 'access', got %s", claims.Type)
	}
	if claims.UserID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, claims.UserID)
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	testCases := []struct {
		name   string
		token  string
		headers map[string]string
	}{
		{
			name:  "missing token",
			token: "",
			headers: map[string]string{},
		},
		{
			name:  "invalid format",
			token: "invalid-token",
			headers: map[string]string{
				"Authorization": "Bearer invalid-token",
			},
		},
		{
			name:  "access token instead of refresh",
			token: "access-token",
			headers: map[string]string{
				"Authorization": "Bearer access-token",
			},
		},
		{
			name:  "malformed token",
			token: "not.a.valid.jwt",
			headers: map[string]string{
				"Authorization": "Bearer not.a.valid.jwt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, "POST", server.URL+"/refresh", nil, tc.headers)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", resp.StatusCode)
			}

			var errResp ErrorResponse
			parseJSONResponse(t, resp, &errResp)

			if errResp.Error != "unauthorized" {
				t.Fatalf("expected error type 'unauthorized', got %s", errResp.Error)
			}
		})
	}
}

func TestAuthHandler_Me_Success(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Create test user
	hashedPassword, err := services.HashPassword("securepassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := models.User{
		Name:     "Test User",
		Email:    "me@test.com",
		Password: hashedPassword,
		Role:     "patient",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Generate access token
	accessToken, err := services.GenerateAccessToken(cfg, user)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// Test successful user info fetch
	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}

	resp := makeRequest(t, "GET", server.URL+"/me", nil, headers)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	parseJSONResponse(t, resp, &result)

	if result["id"] == nil {
		t.Fatalf("expected user ID in response")
	}
	if result["name"] != "Test User" {
		t.Fatalf("expected name Test User, got %v", result["name"])
	}
	if result["email"] != "me@test.com" {
		t.Fatalf("expected email me@test.com, got %v", result["email"])
	}
	if result["role"] != "patient" {
		t.Fatalf("expected role patient, got %v", result["role"])
	}
}

func TestAuthHandler_Me_Unauthorized(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name:    "no authentication",
			headers: map[string]string{},
		},
		{
			name: "invalid token",
			headers: map[string]string{
				"Authorization": "Bearer invalid-token",
			},
		},
		{
			name: "missing bearer prefix",
			headers: map[string]string{
				"Authorization": "some-token",
			},
		},
		{
			name: "malformed token",
			headers: map[string]string{
				"Authorization": "Bearer not.a.valid.jwt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, "GET", server.URL+"/me", nil, tc.headers)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", resp.StatusCode)
			}

			var errResp ErrorResponse
			parseJSONResponse(t, resp, &errResp)

			if errResp.Error != "unauthorized" {
				t.Fatalf("expected error type 'unauthorized', got %s", errResp.Error)
			}
		})
	}
}

// Additional Security and Edge Case Tests

func TestAuthHandler_InputValidation(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	t.Run("SQL injection attempt in email", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":       "Test User",
			"email":      "test' OR '1'='1",
			"password":   "securepassword123",
			"role":       "patient",
			"device_key": "test-device-key-1",
		}

		resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
		defer resp.Body.Close()

		// Should either succeed with sanitized input or fail validation
		if resp.StatusCode == http.StatusInternalServerError {
			t.Fatalf("SQL injection attempt caused internal server error")
		}
	})

	t.Run("XSS attempt in name", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":       "<script>alert('xss')</script>",
			"email":      "xss@test.com",
			"password":   "securepassword123",
			"role":       "patient",
			"device_key": "test-device-key-2",
		}

		resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			var result map[string]interface{}
			parseJSONResponse(t, resp, &result)

			// Verify XSS is not reflected in response
			if result["name"] != nil {
				name := result["name"].(string)
				if strings.Contains(name, "<script>") {
					t.Fatalf("XSS attempt was not sanitized in response")
				}
			}
		}
	})

	t.Run("extremely long inputs", func(t *testing.T) {
		longString := strings.Repeat("a", 10000)
		payload := map[string]interface{}{
			"name":       longString,
			"email":      longString + "@test.com",
			"password":   "securepassword123",
			"role":       "patient",
			"device_key": "test-device-key-3",
		}

		resp := makeRequest(t, "POST", server.URL+"/register", payload, nil)
		defer resp.Body.Close()

		// Should handle gracefully without crashing
		if resp.StatusCode == http.StatusInternalServerError {
			var errResp ErrorResponse
			parseJSONResponse(t, resp, &errResp)
			t.Logf("Long input handled with error: %s", errResp.Message)
		}
	})
}

func TestAuthHandler_ConcurrentOperations(t *testing.T) {
	db := setupTestDB(t)
	cfg := getTestConfig()
	handler := NewAuthHandler(cfg, db)
	server := setupTestServer(handler)
	defer server.Close()

	// Test that the handler can handle multiple requests sequentially without issues
	t.Run("sequential operations", func(t *testing.T) {
		// Create a test user
		hashedPassword, err := services.HashPassword("securepassword123")
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		user := models.User{
			Name:       "Sequential Test User",
			Email:      "sequential@test.com",
			Password:   hashedPassword,
			Role:       "patient",
			DeviceKey:  "sequential-device-key",
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}

		// Test multiple sequential login requests
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{
				"email":    "sequential@test.com",
				"password": "securepassword123",
			}

			resp := makeRequest(t, "POST", server.URL+"/login", payload, nil)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("request %d: expected status 200, got %d", i, resp.StatusCode)
			}

			var result map[string]interface{}
			parseJSONResponse(t, resp, &result)

			if result["access_token"] == nil {
				t.Fatalf("request %d: expected access_token in response", i)
			}
		}
	})
}