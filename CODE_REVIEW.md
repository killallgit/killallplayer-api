# Code Review Report & Improvement Plan

## Current State Analysis

### Metrics
- **Test Coverage**: 38.5% (Critical - needs improvement)
- **Code Lines**: 1,373 production / 529 test
- **Critical Issues**: 4
- **High Priority Issues**: 8
- **Medium Priority Issues**: 6

## Critical Issues to Fix Immediately

### 1. Security: Remove Debug Credential Logging
**File**: `cmd/serve.go:164-165`
```go
// REMOVE THESE LINES:
fmt.Printf("DEBUG: API Key from config: %s\n", apiKey)
fmt.Printf("DEBUG: API Secret length: %d\n", len(apiSecret))
```

### 2. Fix Failing Tests
**File**: `cmd/root.go`
```go
// Add missing persistent flags
rootCmd.PersistentFlags().String("log-level", "info", "Log level")
rootCmd.PersistentFlags().Bool("json-logs", false, "Enable JSON logs")
viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
viper.BindPFlag("json-logs", rootCmd.PersistentFlags().Lookup("json-logs"))
```

### 3. Remove Dead Code
- Delete `pkg/cache/cache.go` (empty file)
- Delete `pkg/ffmpeg/ffmpeg.go` (empty file)
- Delete `internal/services/services.go` (unused)

### 4. Fix Health Endpoint JSON Marshaling
**File**: `cmd/serve.go:200-203`
```go
// Replace manual JSON construction with:
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(health)
```

## High Priority Improvements

### 1. Implement Comprehensive Testing

#### Database Tests (`internal/database/database_test.go`)
```go
package database

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
    // Test successful initialization
    // Test connection failure
    // Test migration execution
}

func TestHealthCheck(t *testing.T) {
    // Test healthy database
    // Test unhealthy database
}
```

#### Handler Tests (`internal/api/handlers/search_test.go`)
```go
package handlers

import (
    "net/http/httptest"
    "testing"
)

func TestSearchHandler_ServeHTTP(t *testing.T) {
    // Test valid search request
    // Test invalid method
    // Test missing query
    // Test API error handling
}
```

### 2. Standardize Error Handling

Create `pkg/errors/errors.go`:
```go
package errors

import "fmt"

type AppError struct {
    Code    string
    Message string
    Err     error
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(code, message string) *AppError {
    return &AppError{Code: code, Message: message}
}

func Wrap(err error, code, message string) *AppError {
    return &AppError{Code: code, Message: message, Err: err}
}
```

### 3. Implement Structured Logging

Create `pkg/logger/logger.go`:
```go
package logger

import (
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

func Init(level string, jsonFormat bool) {
    // Initialize zerolog with proper configuration
    zerolog.SetGlobalLevel(parseLevel(level))
    if !jsonFormat {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }
    Logger = log.Logger
}

func Debug(msg string, fields ...interface{}) {
    Logger.Debug().Fields(fields).Msg(msg)
}

func Info(msg string, fields ...interface{}) {
    Logger.Info().Fields(fields).Msg(msg)
}

func Error(msg string, err error, fields ...interface{}) {
    Logger.Error().Err(err).Fields(fields).Msg(msg)
}
```

### 4. Add Missing Middleware

Create `internal/api/middleware/middleware.go`:
```go
package middleware

import (
    "net/http"
    "time"
    "github.com/rs/cors"
)

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        wrapped := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(wrapped, r)
        // Log request details
    })
}

func RateLimit(rps int) func(http.Handler) http.Handler {
    // Implement rate limiting
}

func CORS(origins []string) func(http.Handler) http.Handler {
    c := cors.New(cors.Options{
        AllowedOrigins: origins,
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type", "Authorization"},
    })
    return c.Handler
}
```

## Medium Priority Improvements

### 1. Refactor Configuration with Interfaces

Create `pkg/config/interface.go`:
```go
package config

type ConfigProvider interface {
    GetString(key string) string
    GetInt(key string) int
    GetBool(key string) bool
    GetDuration(key string) time.Duration
}

type Config struct {
    provider ConfigProvider
}
```

### 2. Implement Dependency Injection

Create `internal/api/deps.go`:
```go
package api

type Dependencies struct {
    DB            database.Database
    PodcastClient podcastindex.Client
    Cache         cache.Cache
    Logger        logger.Logger
}

func NewHandlers(deps *Dependencies) *Handlers {
    return &Handlers{deps: deps}
}
```

### 3. Add Input Validation

Create `internal/api/validation/validation.go`:
```go
package validation

import "github.com/go-playground/validator/v10"

var validate = validator.New()

func ValidateSearchRequest(req *models.SearchRequest) error {
    if err := validate.Struct(req); err != nil {
        return err
    }
    // Additional custom validation
    return nil
}
```

## Testing Strategy

### Unit Test Coverage Goals
- **Target**: 80% coverage minimum
- **Critical Paths**: 100% coverage for:
  - Database operations
  - API handlers
  - Authentication
  - Error handling

### Test Organization
```
tests/
├── unit/           # Unit tests
├── integration/    # Integration tests
├── e2e/           # End-to-end tests
├── fixtures/      # Test data
└── mocks/         # Generated mocks
```

### Testing Checklist
- [ ] Database connection and operations
- [ ] API endpoint happy paths
- [ ] API endpoint error cases
- [ ] Configuration loading
- [ ] Middleware functionality
- [ ] Service integrations
- [ ] WebSocket handling (when implemented)
- [ ] Authentication/authorization
- [ ] Rate limiting
- [ ] Concurrent request handling

## Code Quality Improvements

### 1. Establish Code Standards
- Use `gofmt` and `goimports` consistently
- Enforce via pre-commit hooks
- Add golangci-lint configuration

### 2. Documentation Standards
- Add godoc comments for all exported functions
- Include examples in documentation
- Maintain up-to-date README

### 3. Performance Considerations
- Add connection pooling for database
- Implement request/response caching
- Add metrics collection with Prometheus

## Implementation Priority

### Week 1
1. Fix critical security issues
2. Remove dead code
3. Fix failing tests
4. Standardize error handling

### Week 2
1. Implement structured logging
2. Add comprehensive database tests
3. Add API handler tests
4. Implement missing middleware

### Week 3
1. Refactor configuration management
2. Add input validation
3. Implement dependency injection
4. Increase test coverage to 60%

### Week 4
1. Complete remaining tests (target 80% coverage)
2. Add performance optimizations
3. Documentation updates
4. Final code review

## Monitoring Progress

Use these commands to track improvements:
```bash
# Test coverage
task test:coverage

# Code quality
task lint

# Check for issues
task check

# Run full test suite
task test
```

## Success Metrics
- [ ] Test coverage > 80%
- [ ] All tests passing
- [ ] No critical security issues
- [ ] Consistent error handling
- [ ] Structured logging implemented
- [ ] All middleware functioning
- [ ] Documentation complete
- [ ] Code review approved