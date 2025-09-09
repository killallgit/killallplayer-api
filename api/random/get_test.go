package random

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock client for testing
type mockRandomClient struct {
	getRandomFunc func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error)
}

func (m *mockRandomClient) GetRandomEpisodes(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
	if m.getRandomFunc != nil {
		return m.getRandomFunc(ctx, max, lang, notCategories)
	}
	return &podcastindex.EpisodesResponse{
		Status:      "true",
		Items:       []podcastindex.Episode{},
		Count:       0,
		Description: "Random episodes",
	}, nil
}

// Implement other required methods for PodcastClient interface
func (m *mockRandomClient) Search(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error) {
	return &podcastindex.SearchResponse{}, nil
}

func (m *mockRandomClient) GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*podcastindex.SearchResponse, error) {
	return &podcastindex.SearchResponse{}, nil
}

func (m *mockRandomClient) GetCategories() (*podcastindex.CategoriesResponse, error) {
	return &podcastindex.CategoriesResponse{}, nil
}

func (m *mockRandomClient) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockRandomClient) GetPodcastByFeedURL(ctx context.Context, feedURL string) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockRandomClient) GetPodcastByFeedID(ctx context.Context, feedID int64) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockRandomClient) GetPodcastByiTunesID(ctx context.Context, itunesID int64) (*podcastindex.PodcastResponse, error) {
	return &podcastindex.PodcastResponse{}, nil
}

func (m *mockRandomClient) GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockRandomClient) GetEpisodesByiTunesID(ctx context.Context, itunesID int64, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockRandomClient) GetRecentEpisodes(ctx context.Context, limit int) (*podcastindex.EpisodesResponse, error) {
	return &podcastindex.EpisodesResponse{}, nil
}

func (m *mockRandomClient) GetRecentFeeds(ctx context.Context, limit int) (*podcastindex.RecentFeedsResponse, error) {
	return &podcastindex.RecentFeedsResponse{}, nil
}

func TestGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		mockFunc       func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error)
		expectedStatus int
		expectedMax    int
		expectedLang   string
		expectedNotCat []string
	}{
		{
			name:        "Default parameters",
			queryParams: map[string]string{},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, 10, max)
				assert.Equal(t, "en", lang)
				assert.Empty(t, notCategories)
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items: []podcastindex.Episode{
						{ID: 1, Title: "Episode 1"},
						{ID: 2, Title: "Episode 2"},
					},
					Count: 2,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Custom limit",
			queryParams: map[string]string{
				"limit": "5",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, 5, max)
				assert.Equal(t, "en", lang)
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Limit exceeds maximum",
			queryParams: map[string]string{
				"limit": "200",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, 100, max) // Should be capped at 100
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid limit defaults to 10",
			queryParams: map[string]string{
				"limit": "invalid",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, 10, max) // Should default to 10
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Custom language",
			queryParams: map[string]string{
				"lang": "es",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, "es", lang)
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Exclude categories",
			queryParams: map[string]string{
				"notcat": "News,Politics,Religion",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, []string{"News", "Politics", "Religion"}, notCategories)
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Exclude categories with spaces",
			queryParams: map[string]string{
				"notcat": "News, Politics , Religion",
			},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				assert.Equal(t, []string{"News", "Politics", "Religion"}, notCategories)
				return &podcastindex.EpisodesResponse{
					Status: "true",
					Items:  []podcastindex.Episode{},
					Count:  0,
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "API error",
			queryParams: map[string]string{},
			mockFunc: func(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error) {
				return nil, assert.AnError
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &mockRandomClient{
				getRandomFunc: tt.mockFunc,
			}

			// Create dependencies
			deps := &types.Dependencies{
				PodcastClient: mockClient,
			}

			// Create test router
			router := gin.New()
			router.GET("/random", Get(deps))

			// Build URL with query parameters
			url := "/random"
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for key, value := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += key + "=" + value
					first = false
				}
			}

			// Create request
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// If successful, check response structure
			if tt.expectedStatus == http.StatusOK {
				var response podcastindex.EpisodesResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "true", response.Status)
			}
		})
	}
}
