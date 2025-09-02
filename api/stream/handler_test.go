package stream

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock types for testing
type mockEpisodeService struct {
	mock.Mock
}

func (m *mockEpisodeService) GetEpisodeByPodcastIndexID(ctx context.Context, id int64) (*models.Episode, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *mockEpisodeService) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *mockEpisodeService) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *mockEpisodeService) FetchAndSyncEpisodes(ctx context.Context, podcastID int64, limit int) (*episodes.PodcastIndexResponse, error) {
	args := m.Called(ctx, podcastID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*episodes.PodcastIndexResponse), args.Error(1)
}

func (m *mockEpisodeService) SyncEpisodesToDatabase(ctx context.Context, episodeList []episodes.PodcastIndexEpisode, podcastID uint) (int, error) {
	args := m.Called(ctx, episodeList, podcastID)
	return args.Int(0), args.Error(1)
}

func (m *mockEpisodeService) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error) {
	args := m.Called(ctx, podcastID, page, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Episode), args.Get(1).(int64), args.Error(2)
}

func (m *mockEpisodeService) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Episode), args.Error(1)
}

func (m *mockEpisodeService) UpdatePlaybackState(ctx context.Context, id uint, position int, played bool) error {
	args := m.Called(ctx, id, position, played)
	return args.Error(0)
}

func (m *mockEpisodeService) UpdatePlaybackStateByPodcastIndexID(ctx context.Context, podcastIndexID int64, position int, played bool) error {
	args := m.Called(ctx, podcastIndexID, position, played)
	return args.Error(0)
}

func TestStreamEpisode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		episodeID      string
		setupMock      func(*mockEpisodeService)
		setupServer    func() *httptest.Server
		expectedStatus int
		expectedError  string
		rangeHeader    string
	}{
		{
			name:      "Invalid episode ID",
			episodeID: "invalid",
			setupMock: func(m *mockEpisodeService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid episode ID",
		},
		{
			name:      "Episode not found",
			episodeID: "12345",
			setupMock: func(m *mockEpisodeService) {
				m.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
					Return(nil, errors.New("not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "Episode not found",
		},
		{
			name:      "Episode has no audio URL",
			episodeID: "12345",
			setupMock: func(m *mockEpisodeService) {
				m.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
					Return(&models.Episode{
						PodcastIndexID: 12345,
						AudioURL:       "",
					}, nil)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "Audio not available for this episode",
		},
		{
			name:      "Successful streaming",
			episodeID: "12345",
			setupMock: func(m *mockEpisodeService) {
				m.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
					Return(&models.Episode{
						PodcastIndexID: 12345,
						AudioURL:       "http://test-server/audio.mp3",
					}, nil)
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "audio/mpeg")
					w.Header().Set("Content-Length", "1000")
					w.WriteHeader(http.StatusOK)
					// Write some test data
					data := make([]byte, 1000)
					w.Write(data)
				}))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "Range request",
			episodeID: "12345",
			setupMock: func(m *mockEpisodeService) {
				m.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
					Return(&models.Episode{
						PodcastIndexID: 12345,
						AudioURL:       "http://test-server/audio.mp3",
					}, nil)
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Check if range header was passed through
					if r.Header.Get("Range") == "bytes=0-499" {
						w.Header().Set("Content-Type", "audio/mpeg")
						w.Header().Set("Content-Range", "bytes 0-499/1000")
						w.Header().Set("Content-Length", "500")
						w.WriteHeader(http.StatusPartialContent)
						data := make([]byte, 500)
						w.Write(data)
					} else {
						w.WriteHeader(http.StatusBadRequest)
					}
				}))
			},
			rangeHeader:    "bytes=0-499",
			expectedStatus: http.StatusPartialContent,
		},
		{
			name:      "HTML content instead of audio",
			episodeID: "12345",
			setupMock: func(m *mockEpisodeService) {
				m.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
					Return(&models.Episode{
						PodcastIndexID: 12345,
						AudioURL:       "http://test-server/page.html",
					}, nil)
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("<html><body>Not audio</body></html>"))
				}))
			},
			expectedStatus: http.StatusBadGateway,
			expectedError:  "Audio source returned HTML instead of audio content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockService := new(mockEpisodeService)
			tt.setupMock(mockService)

			// Setup test server if needed
			var testServer *httptest.Server
			if tt.setupServer != nil {
				testServer = tt.setupServer()
				defer testServer.Close()
				
				// Update the mock to use test server URL
				if tt.expectedStatus == http.StatusOK || tt.expectedStatus == http.StatusPartialContent || tt.expectedStatus == http.StatusBadGateway {
					mockService.ExpectedCalls = nil
					mockService.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
						Return(&models.Episode{
							PodcastIndexID: 12345,
							AudioURL:       testServer.URL,
						}, nil)
				}
			}

			// Create dependencies
			deps := &types.Dependencies{
				EpisodeService: mockService,
			}

			// Create test context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/stream/"+tt.episodeID, nil)
			c.Params = []gin.Param{{Key: "id", Value: tt.episodeID}}
			
			if tt.rangeHeader != "" {
				c.Request.Header.Set("Range", tt.rangeHeader)
			}

			// Execute handler
			handler := StreamEpisode(deps)
			handler(c)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}

			// Verify mock expectations
			mockService.AssertExpectations(t)
		})
	}
}

func TestStreamingWithFlusher(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mock service
	mockService := new(mockEpisodeService)
	mockService.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
		Return(&models.Episode{
			PodcastIndexID: 12345,
			AudioURL:       "http://test-server/audio.mp3",
		}, nil)

	// Create a test server that sends data slowly
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		
		// Send data in chunks with delays to test flushing
		for i := 0; i < 5; i++ {
			data := bytes.Repeat([]byte{byte(i)}, 100)
			w.Write(data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer testServer.Close()

	// Update mock with test server URL
	mockService.ExpectedCalls = nil
	mockService.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
		Return(&models.Episode{
			PodcastIndexID: 12345,
			AudioURL:       testServer.URL,
		}, nil)

	// Create dependencies
	deps := &types.Dependencies{
		EpisodeService: mockService,
	}

	// Create test context with custom response writer that supports flushing
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/stream/12345", nil)
	c.Params = []gin.Param{{Key: "id", Value: "12345"}}

	// Execute handler
	handler := StreamEpisode(deps)
	handler(c)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	// Should have received 500 bytes (5 chunks of 100 bytes)
	assert.Equal(t, 500, w.Body.Len())

	mockService.AssertExpectations(t)
}

func TestClientDisconnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mock service
	mockService := new(mockEpisodeService)
	mockService.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
		Return(&models.Episode{
			PodcastIndexID: 12345,
			AudioURL:       "http://test-server/audio.mp3",
		}, nil)

	// Create a test server that sends a lot of data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		
		// Try to send a lot of data
		for i := 0; i < 1000; i++ {
			data := make([]byte, 1024)
			_, err := w.Write(data)
			if err != nil {
				// Client disconnected
				return
			}
		}
	}))
	defer testServer.Close()

	// Update mock with test server URL
	mockService.ExpectedCalls = nil
	mockService.On("GetEpisodeByPodcastIndexID", mock.Anything, int64(12345)).
		Return(&models.Episode{
			PodcastIndexID: 12345,
			AudioURL:       testServer.URL,
		}, nil)

	// Create dependencies
	deps := &types.Dependencies{
		EpisodeService: mockService,
	}

	// Create test context with context that will be cancelled (simulating client disconnect)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	c.Request = httptest.NewRequest("GET", "/stream/12345", nil).WithContext(ctx)
	c.Params = []gin.Param{{Key: "id", Value: "12345"}}

	// Execute handler
	handler := StreamEpisode(deps)
	handler(c)

	// The handler should handle the disconnection gracefully
	// We don't check specific status as the client disconnected
	mockService.AssertExpectations(t)
}