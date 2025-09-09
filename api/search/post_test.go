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
	searchFunc func(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error)
}

func (m *mockSearcher) Search(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, limit, fullText)
	}
	return &podcastindex.SearchResponse{}, nil
}

func (m *mockSearcher) GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*podcastindex.SearchResponse, error) {
	// Return empty response for tests
	return &podcastindex.SearchResponse{}, nil
}

func (m *mockSearcher) GetCategories() (*podcastindex.CategoriesResponse, error) {
	// Return empty response for tests
	return &podcastindex.CategoriesResponse{}, nil
}

func (m *mockSearcher) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*podcastindex.EpisodesResponse, error) {
	// Return empty response for tests
	return &podcastindex.EpisodesResponse{}, nil
}

// Additional methods required by PodcastClient interface
func (m *mockSearcher) GetPodcastByFeedURL(ctx context.Context, feedURL string) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockSearcher) GetPodcastByFeedID(ctx context.Context, feedID int64) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockSearcher) GetPodcastByiTunesID(ctx context.Context, itunesID int64) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockSearcher) GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockSearcher) GetEpisodesByiTunesID(ctx context.Context, itunesID int64, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockSearcher) GetRecentEpisodes(ctx context.Context, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockSearcher) GetRandomEpisodes(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockSearcher) GetRecentFeeds(ctx context.Context, limit int) (*podcastindex.RecentFeedsResponse, error) {
	return &podcastindex.RecentFeedsResponse{}, nil
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
						searchFunc: func(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
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
				results, ok := resp["results"].([]interface{})
				require.True(t, ok)
				assert.Len(t, results, 1)

				result := results[0].(map[string]interface{})
				assert.Equal(t, float64(1), result["id"])
				assert.Equal(t, "Tech Podcast", result["title"])
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
						searchFunc: func(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
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
					PodcastClient: nil, // Nil client to test error handling
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
						searchFunc: func(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
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
						searchFunc: func(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
							return &podcastindex.SearchResponse{
								Feeds: []podcastindex.Podcast{},
							}, nil
						},
					},
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				results, ok := resp["results"].([]interface{})
				require.True(t, ok)
				assert.Len(t, results, 0)
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
