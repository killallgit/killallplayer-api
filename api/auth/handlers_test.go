package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	authService "github.com/killallgit/player-api/internal/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestHandler_Me(t *testing.T) {
	router := setupTestRouter()

	// Create handler without auth service for this specific test
	handler := &Handler{}
	router.GET("/me", handler.Me)

	t.Run("valid user claims", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/me", nil)

		// Create a context with valid claims
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		claims := &authService.Claims{
			Sub:   "user-123",
			Email: "test@example.com",
			AppMetadata: authService.AppMetadata{
				Permissions: []string{"podcasts:read", "podcasts:write"},
				Role:        "user",
			},
		}
		c.Set("claims", claims)

		handler.Me(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response authService.UserInfo
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "user-123", response.ID)
		assert.Equal(t, "test@example.com", response.Email)
		assert.Equal(t, []string{"podcasts:read", "podcasts:write"}, response.Permissions)
		assert.Equal(t, "user", response.Role)
	})

	t.Run("missing claims", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/me", nil)

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		// No claims set

		handler.Me(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Unauthorized", response["error"])
	})
}

func TestAuthMiddleware(t *testing.T) {
	// Create a test auth service with dev mode
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`)) // Empty JWKS for testing
	}))
	defer server.Close()

	authSvc, err := authService.NewService(server.URL)
	require.NoError(t, err)

	devToken := "valid-dev-token"
	authSvc.SetDevAuth(true, devToken)

	handler := NewHandler(authSvc)
	router := setupTestRouter()

	// Test route with auth middleware
	router.GET("/protected", handler.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected resource"})
	})

	t.Run("valid dev token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+devToken)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "protected resource", response["message"])
	})

	t.Run("missing Authorization header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Authorization header required", response["error"])
	})

	t.Run("invalid Authorization format", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat token")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Invalid authorization header format", response["error"])
	})

	t.Run("invalid token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Invalid or expired token", response["error"])
	})

	t.Run("empty Bearer token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer ")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Empty token gets treated as invalid token by the auth service
		assert.Equal(t, "Invalid or expired token", response["error"])
	})
}

func TestNewHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	}))
	defer server.Close()

	authSvc, err := authService.NewService(server.URL)
	require.NoError(t, err)

	handler := NewHandler(authSvc)

	assert.NotNil(t, handler)
	assert.Equal(t, authSvc, handler.authService)
}

func TestHandler_SetDevAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	}))
	defer server.Close()

	authSvc, err := authService.NewService(server.URL)
	require.NoError(t, err)

	handler := NewHandler(authSvc)

	// Test enabling dev auth
	handler.SetDevAuth(true, "test-token")
	assert.True(t, handler.devAuthEnabled)
	assert.Equal(t, "test-token", handler.devAuthToken)

	// Test disabling dev auth
	handler.SetDevAuth(false, "")
	assert.False(t, handler.devAuthEnabled)
	assert.Equal(t, "", handler.devAuthToken)
}
