package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupDeps      func() *types.Dependencies
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "healthy with database",
			setupDeps: func() *types.Dependencies {
				db, err := database.Initialize(":memory:", false)
				require.NoError(t, err)
				return &types.Dependencies{
					DB: db,
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status":  "healthy",
				"version": "1.0.0",
				"database": map[string]interface{}{
					"status":    "connected",
					"connected": true,
				},
			},
		},
		{
			name: "healthy without database",
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status":  "healthy",
				"version": "1.0.0",
				"database": map[string]interface{}{
					"status":    "not configured",
					"connected": false,
				},
			},
		},
		{
			name: "unhealthy with closed database",
			setupDeps: func() *types.Dependencies {
				db, err := database.Initialize(":memory:", false)
				require.NoError(t, err)
				
				// Close the database connection
				sqlDB, _ := db.DB.DB()
				sqlDB.Close()
				
				return &types.Dependencies{
					DB: db,
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody: map[string]interface{}{
				"status":  "unhealthy",
				"version": "1.0.0",
				"database": map[string]interface{}{
					"status":    "error",
					"connected": false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			deps := tt.setupDeps()
			handler := Get(deps)
			
			// Execute
			handler(c)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectedBody["status"], response["status"])
			assert.Equal(t, tt.expectedBody["version"], response["version"])
			
			if dbStatus, ok := response["database"].(map[string]interface{}); ok {
				expectedDB := tt.expectedBody["database"].(map[string]interface{})
				assert.Equal(t, expectedDB["connected"], dbStatus["connected"])
				assert.Contains(t, dbStatus["status"].(string), expectedDB["status"].(string))
			}
			
			// Cleanup
			if deps.DB != nil && deps.DB.DB != nil {
				if sqlDB, err := deps.DB.DB.DB(); err == nil {
					sqlDB.Close()
				}
			}
		})
	}
}

func TestGetDatabaseStatus(t *testing.T) {
	tests := []struct {
		name     string
		setupDeps func() *types.Dependencies
		expected map[string]interface{}
	}{
		{
			name: "nil database",
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{}
			},
			expected: map[string]interface{}{
				"status": "not configured",
			},
		},
		{
			name: "healthy database",
			setupDeps: func() *types.Dependencies {
				db, err := database.Initialize(":memory:", false)
				require.NoError(t, err)
				return &types.Dependencies{DB: db}
			},
			expected: map[string]interface{}{
				"status": "connected",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := tt.setupDeps()
			status := getDatabaseStatus(deps)
			
			assert.Contains(t, status["status"].(string), tt.expected["status"].(string))
			
			// Cleanup
			if deps.DB != nil && deps.DB.DB != nil {
				if sqlDB, err := deps.DB.DB.DB(); err == nil {
					sqlDB.Close()
				}
			}
		})
	}
}