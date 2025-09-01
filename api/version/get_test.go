package version

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "successful version request",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"name":        "Podcast Player API",
				"version":     "1.0.0",
				"description": "API for managing and streaming podcasts",
				"status":      "running",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			handler := Get()
			
			// Execute
			handler(c)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			for key, expectedValue := range tt.expectedBody {
				assert.Equal(t, expectedValue, response[key], "Key: %s", key)
			}
		})
	}
}