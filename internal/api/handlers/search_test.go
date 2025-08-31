package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPodcastSearcher is a mock implementation of the PodcastSearcher interface
type MockPodcastSearcher struct {
	mock.Mock
}

func (m *MockPodcastSearcher) Search(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*podcastindex.SearchResponse), args.Error(1)
}

func TestSearchHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*MockPodcastSearcher)
		expectedStatus int
		expectedError  string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful search",
			requestBody: models.SearchRequest{
				Query: "technology",
				Limit: 5,
			},
			setupMock: func(m *MockPodcastSearcher) {
				m.On("Search", mock.Anything, "technology", 5).Return(&podcastindex.SearchResponse{
					Status: "true",
					Count:  2,
					Feeds: []podcastindex.Podcast{
						{
							ID:          1,
							Title:       "Tech Podcast",
							Description: "A podcast about technology",
							Author:      "Tech Author",
							Image:       "https://example.com/image.jpg",
							URL:         "https://example.com/feed.xml",
						},
						{
							ID:          2,
							Title:       "Another Tech Show",
							Description: "Another technology podcast",
							Author:      "Another Author",
							Image:       "https://example.com/image2.jpg",
							URL:         "https://example.com/feed2.xml",
						},
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				podcasts := resp["podcasts"].([]interface{})
				assert.Len(t, podcasts, 2)
				
				first := podcasts[0].(map[string]interface{})
				assert.Equal(t, "1", first["id"])
				assert.Equal(t, "Tech Podcast", first["title"])
			},
		},
		{
			name:           "invalid request body",
			requestBody:    "invalid json",
			setupMock:      func(m *MockPodcastSearcher) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request format",
		},
		{
			name: "empty query",
			requestBody: models.SearchRequest{
				Query: "",
				Limit: 5,
			},
			setupMock:      func(m *MockPodcastSearcher) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Query parameter is required",
		},
		{
			name: "invalid limit - too high",
			requestBody: models.SearchRequest{
				Query: "test",
				Limit: 1000,
			},
			setupMock:      func(m *MockPodcastSearcher) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Limit must be between 1 and 100",
		},
		{
			name: "zero limit uses default",
			requestBody: models.SearchRequest{
				Query: "test",
				Limit: 0,
			},
			setupMock: func(m *MockPodcastSearcher) {
				m.On("Search", mock.Anything, "test", 10).Return(&podcastindex.SearchResponse{
					Status: "true",
					Count:  1,
					Feeds: []podcastindex.Podcast{
						{
							ID:    1,
							Title: "Test Podcast",
						},
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				podcasts := resp["podcasts"].([]interface{})
				assert.Len(t, podcasts, 1)
			},
		},
		{
			name: "podcast index error",
			requestBody: models.SearchRequest{
				Query: "test",
				Limit: 10,
			},
			setupMock: func(m *MockPodcastSearcher) {
				m.On("Search", mock.Anything, "test", 10).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to search podcasts",
		},
		{
			name: "default limit when not specified",
			requestBody: map[string]interface{}{
				"query": "test",
			},
			setupMock: func(m *MockPodcastSearcher) {
				m.On("Search", mock.Anything, "test", 10).Return(&podcastindex.SearchResponse{
					Status: "true",
					Count:  0,
					Feeds:  []podcastindex.Podcast{},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				podcasts := resp["podcasts"].([]interface{})
				assert.Len(t, podcasts, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := new(MockPodcastSearcher)
			tt.setupMock(mockClient)

			// Create handler with mock
			handler := &SearchHandler{
				podcastClient: mockClient,
			}

			// Prepare request body
			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				assert.NoError(t, err)
			}

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create gin context and call handler
			c, _ := gin.CreateTestContext(rr)
			c.Request = req
			handler.HandleSearch(c)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check error message if expected
			if tt.expectedError != "" {
				assert.Equal(t, "error", response["status"])
				assert.Contains(t, response["message"], tt.expectedError)
			}

			// Additional response checks
			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
		})
	}
}

func TestNewSearchHandler(t *testing.T) {
	// Test with valid configuration
	client := &podcastindex.Client{}
	handler := NewSearchHandler(client)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.podcastClient)
}