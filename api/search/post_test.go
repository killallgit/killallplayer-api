package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock searcher for testing
type mockSearcher struct {
	searchFunc func(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error)
}

func (m *mockSearcher) Search(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, limit)
	}
	return &podcastindex.SearchResponse{}, nil
}

func TestPost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           interface{}
		setupDeps      func() *types.Dependencies
		expectedStatus int
		expectedBody   map[string]interface{}
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful search",
			body: models.SearchRequest{
				Query: "technology",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{
						searchFunc: func(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
							return &podcastindex.SearchResponse{
								Feeds: []podcastindex.Podcast{
									{
										ID:          1,
										Title:       "Tech Podcast",
										Author:      "John Doe",
										Description: "A podcast about technology",
										Image:       "https://example.com/image.jpg",
										URL:         "https://example.com/feed.xml",
									},
								},
							}, nil
						},
					},
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				podcasts, ok := resp["podcasts"].([]interface{})
				require.True(t, ok)
				assert.Len(t, podcasts, 1)

				podcast := podcasts[0].(map[string]interface{})
				assert.Equal(t, "1", podcast["id"])
				assert.Equal(t, "Tech Podcast", podcast["title"])
			},
		},
		{
			name: "empty query",
			body: models.SearchRequest{
				Query: "",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Search query is required",
			},
		},
		{
			name: "invalid JSON",
			body: "invalid json",
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{},
				}
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "error", resp["status"])
				assert.Equal(t, "Invalid request format", resp["message"])
				assert.NotEmpty(t, resp["details"])
			},
		},
		{
			name: "limit too high",
			body: models.SearchRequest{
				Query: "test",
				Limit: 101,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Limit must be between 1 and 100",
			},
		},
		{
			name: "limit too low",
			body: models.SearchRequest{
				Query: "test",
				Limit: -1,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Limit must be between 1 and 100",
			},
		},
		{
			name: "default limit when not provided",
			body: models.SearchRequest{
				Query: "test",
				Limit: 0,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{
						searchFunc: func(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
							assert.Equal(t, 10, limit) // Should use default of 10
							return &podcastindex.SearchResponse{
								Feeds: []podcastindex.Podcast{},
							}, nil
						},
					},
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "podcast client not configured",
			body: models.SearchRequest{
				Query: "test",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: nil,
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Search service not available",
			},
		},
		{
			name: "podcast client wrong type",
			body: models.SearchRequest{
				Query: "test",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: "not a searcher", // Wrong type
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Search service not available",
			},
		},
		{
			name: "search service error",
			body: models.SearchRequest{
				Query: "test",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{
						searchFunc: func(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
							return nil, errors.New("API error")
						},
					},
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Failed to search podcasts",
			},
		},
		{
			name: "empty results",
			body: models.SearchRequest{
				Query: "nonexistent",
				Limit: 5,
			},
			setupDeps: func() *types.Dependencies {
				return &types.Dependencies{
					PodcastClient: &mockSearcher{
						searchFunc: func(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
							return &podcastindex.SearchResponse{
								Feeds: []podcastindex.Podcast{},
							}, nil
						},
					},
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				podcasts, ok := resp["podcasts"].([]interface{})
				require.True(t, ok)
				assert.Len(t, podcasts, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)

			deps := tt.setupDeps()
			handler := Post(deps)

			// Prepare request
			var body []byte
			var err error
			if str, ok := tt.body.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			// Register route and execute
			router.POST("/api/v1/search", handler)
			router.ServeHTTP(w, c.Request)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedBody != nil {
				for key, value := range tt.expectedBody {
					assert.Equal(t, value, response[key], "Key: %s", key)
				}
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}
		})
	}
}
