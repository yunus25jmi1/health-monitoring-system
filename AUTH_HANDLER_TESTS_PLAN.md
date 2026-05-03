# Auth Handler Tests Implementation Plan

## Overview
Implement comprehensive tests for the AuthHandler to ensure proper authentication flow and token management as specified in issue #2.

## Current State Analysis
- Existing file: `handlers/auth_handler.go` with Register, Login, Refresh, and Me methods
- Current tests: None for auth handler
- Required tests: 10 specific test cases covering all auth handler functionality
- Dependencies: services package, middleware, GORM database, Gin framework

## Implementation Plan

### Phase 1: Test Infrastructure Setup
1. **Create test helper functions**
   - Setup test database (in-memory SQLite)
   - Setup test configuration
   - Create test HTTP server with Gin
   - Helper for making HTTP requests
   - Helper for parsing JSON responses

### Phase 2: Registration Tests
2. **TestAuthHandler_Register_Success**
   - Test successful patient registration
   - Test successful doctor registration with valid token
   - Verify response contains user data
   - Verify user is created in database

3. **TestAuthHandler_Register_DuplicateEmail**
   - Test with existing email
   - Verify 409 Conflict response
   - Verify proper error message

4. **TestAuthHandler_Register_InvalidPayload**
   - Test with missing required fields
   - Test with invalid email format
   - Test with short password
   - Test with invalid role
   - Test with missing doctor token for doctor role
   - Test with invalid doctor token

### Phase 3: Login Tests
5. **TestAuthHandler_Login_Success**
   - Test successful login with valid credentials
   - Verify response contains access and refresh tokens
   - Verify response contains user information
   - Test with different user roles

6. **TestAuthHandler_Login_InvalidCredentials**
   - Test with wrong password
   - Verify 401 Unauthorized response
   - Verify proper error message

7. **TestAuthHandler_Login_NonexistentUser**
   - Test with non-existent email
   - Verify 401 Unauthorized response
   - Verify proper error message

### Phase 4: Token Management Tests
8. **TestAuthHandler_Refresh_Success**
   - Test successful token refresh with valid refresh token
   - Verify new access token is generated
   - Verify access token is valid

9. **TestAuthHandler_Refresh_InvalidToken**
   - Test with invalid token format
   - Test with access token instead of refresh token
   - Test with expired token
   - Test with tampered token
   - Verify 401 Unauthorized response

10. **TestAuthHandler_Me_Success**
    - Test successful user info fetch with valid authentication
    - Verify response contains user data
    - Test with different user roles

11. **TestAuthHandler_Me_Unauthorized**
    - Test without authentication
    - Test with invalid token
    - Test with expired token
    - Verify 401 Unauthorized response

### Phase 5: Edge Cases and Security Tests
12. **Additional security tests**
    - Test SQL injection attempts
    - Test XSS attempts in user input
    - Test with extremely long inputs
    - Test with special characters in inputs
    - Test concurrent registration attempts
    - Test rate limiting (if implemented)

## Implementation Details

### Test Database Setup
```go
func setupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to setup test database: %v", err)
    }
    
    // Run migrations
    db.AutoMigrate(&models.User{})
    
    return db
}
```

### Test Configuration
```go
func getTestConfig() config.Config {
    return config.Config{
        JWTSecret:     "test-secret-key-for-testing",
        AccessTokenH:  1,
        RefreshTokenD: 1,
        DoctorRegToken: "test-doctor-token",
    }
}
```

### Test HTTP Server Setup
```go
func setupTestServer(handler *handlers.AuthHandler) *httptest.Server {
    router := gin.Default()
    router.POST("/register", handler.Register)
    router.POST("/login", handler.Login)
    router.POST("/refresh", handler.Refresh)
    router.GET("/me", middleware.AuthMiddleware(handler.cfg, handler.db), handler.Me)
    
    return httptest.NewServer(router)
}
```

### Test Helper Functions
```go
func makeRequest(t *testing.T, method, path string, body interface{}, headers map[string]string) *http.Response {
    // Implementation for making HTTP requests
}

func parseResponse(t *testing.T, resp *http.Response, target interface{}) {
    // Implementation for parsing JSON responses
}
```

## Test Coverage Goals
- All public handler methods tested
- Success paths covered
- Error paths covered
- Edge cases covered
- Security scenarios covered
- Input validation tested
- Database operations tested

## Success Criteria
- All 10 required test cases implemented
- All tests pass
- Code coverage > 80% for auth_handler.go
- Proper error handling verified
- Security scenarios tested
- Input validation confirmed

## Dependencies
- handlers/auth_handler.go (already exists)
- services/auth_service.go (already exists)
- middleware/errors.go (already exists)
- models/user.go (already exists)
- GORM for database mocking
- httptest for HTTP testing
- Gin for HTTP framework

## Notes
- AuthHandler uses direct database access, so we'll use in-memory SQLite for testing
- No interface abstraction currently exists for database operations
- Tests will be integration-style tests with real database operations
- Focus on comprehensive edge case coverage
- Ensure security best practices in test scenarios
- Test both success and failure paths thoroughly

## Security Considerations
- Verify error messages don't expose sensitive information
- Test input sanitization and validation
- Verify proper authentication checks
- Test authorization for different user roles
- Verify token security and validation
- Test for common web vulnerabilities