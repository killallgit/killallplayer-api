package episodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/stretchr/testify/assert"
)

func TestGetAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupDeps      func() *types.Dependencies
		expectedStatus int
	}{
		{
			name: "no episode service configured",
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{}
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)
			
			deps := tt.setupDeps()
			handler := GetAll(deps)
			
			// Create proper request
			req := httptest.NewRequest("GET", "/episodes", nil)
			c.Request = req
			
			// Register route and execute
			router.GET("/episodes", handler)
			router.ServeHTTP(w, req)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, "error", response["status"])
		})
	}
}