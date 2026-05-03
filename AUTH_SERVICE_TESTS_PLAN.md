# Auth Service Tests Implementation Plan

## Overview
Implement comprehensive tests for the AuthService to ensure proper authentication and token handling as specified in issue #1.

## Current State Analysis
- Existing file: `services/auth_service_test.go` with basic tests
- Current tests: `TestPasswordHashAndCompare`, `TestGenerateAndParseToken`
- Required tests: 10 specific test cases covering all auth service functionality

## Implementation Plan

### Phase 1: Test Structure Setup
1. **Create test helper functions**
   - Setup test configuration
   - Create test user fixtures
   - Helper for token validation

### Phase 2: Password Management Tests
2. **TestAuthService_HashPassword_Success**
   - Test successful password hashing with valid password
   - Verify hash format and length
   - Ensure different passwords produce different hashes

3. **TestAuthService_HashPassword_EmptyPassword**
   - Test with empty string
   - Test with whitespace only
   - Test with password < 8 characters
   - Verify proper error handling

4. **TestAuthService_CheckPassword_Success**
   - Test successful password verification
   - Test with hashed password
   - Verify no error returned

5. **TestAuthService_CheckPassword_WrongPassword**
   - Test with incorrect password
   - Test with empty password
   - Verify ErrInvalidCredentials returned

### Phase 3: Token Generation Tests
6. **TestAuthService_GenerateAccessToken_Success**
   - Test successful access token generation
   - Verify token structure and claims
   - Test with different user roles

7. **TestAuthService_GenerateAccessToken_InvalidRole**
   - Test with invalid role
   - Verify ErrInvalidRole returned
   - Test edge cases (empty role, malformed role)

8. **TestAuthService_GenerateRefreshToken_Success**
   - Test successful refresh token generation
   - Verify token type is "refresh"
   - Verify longer expiration than access token

### Phase 4: Token Parsing Tests
9. **TestAuthService_ParseToken_Success**
   - Test successful token parsing
   - Verify all claims are extracted correctly
   - Test with both access and refresh tokens

10. **TestAuthService_ParseToken_InvalidToken**
    - Test with malformed token
    - Test with wrong signature
    - Test with empty token
    - Verify ErrInvalidToken returned

11. **TestAuthService_ParseToken_ExpiredToken**
    - Test with expired token
    - Verify proper error handling
    - Test edge cases around expiration

### Phase 5: Edge Cases and Security Tests
12. **Additional security tests**
    - Test token tampering detection
    - Test with different JWT secrets
    - Test concurrent token generation
    - Test password hashing with special characters

## Implementation Details

### Test Configuration
```go
testConfig := config.Config{
    JWTSecret:     "test-secret-key-for-testing",
    AccessTokenH:  1,
    RefreshTokenD: 1,
}
```

### Test User Fixtures
```go
testUsers := map[string]models.User{
    "doctor":  {ID: 1, Name: "Dr. Test", Email: "doctor@test.com", Role: models.RoleDoctor},
    "patient": {ID: 2, Name: "Patient Test", Email: "patient@test.com", Role: models.RolePatient},
    "device":  {ID: 3, Name: "Device Test", Email: "device@test.com", Role: models.RoleDevice},
}
```

### Test Coverage Goals
- All public functions tested
- Success paths covered
- Error paths covered
- Edge cases covered
- Security scenarios covered

## Success Criteria
- All 10 required test cases implemented
- All tests pass
- Code coverage > 80% for auth_service.go
- Proper error handling verified
- Security scenarios tested

## Dependencies
- services/auth_service.go (already exists)
- config/config.go (already exists)
- models/user.go (already exists)
- No interface/mock dependencies needed (auth service uses only pure functions)

## Notes
- Auth service functions are pure functions with no external dependencies
- No database or external API calls
- Tests can be unit tests without mocking
- Focus on comprehensive edge case coverage
- Ensure security best practices in test scenarios