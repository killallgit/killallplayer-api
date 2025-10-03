package podcasts

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// MockPodcastRepository is a mock implementation of PodcastRepository
type MockPodcastRepository struct {
	mock.Mock
}

func (m *MockPodcastRepository) GetPodcastByID(ctx context.Context, id uint) (*models.Podcast, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Podcast), args.Error(1)
}

func (m *MockPodcastRepository) GetPodcastByPodcastIndexID(ctx context.Context, piID int64) (*models.Podcast, error) {
	args := m.Called(ctx, piID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Podcast), args.Error(1)
}

func (m *MockPodcastRepository) GetPodcastByFeedURL(ctx context.Context, feedURL string) (*models.Podcast, error) {
	args := m.Called(ctx, feedURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Podcast), args.Error(1)
}

func (m *MockPodcastRepository) GetPodcastByITunesID(ctx context.Context, itunesID int64) (*models.Podcast, error) {
	args := m.Called(ctx, itunesID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Podcast), args.Error(1)
}

func (m *MockPodcastRepository) ListPodcasts(ctx context.Context, page, limit int) ([]models.Podcast, int64, error) {
	args := m.Called(ctx, page, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Podcast), args.Get(1).(int64), args.Error(2)
}

func (m *MockPodcastRepository) SearchPodcasts(ctx context.Context, query string, limit int) ([]models.Podcast, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Podcast), args.Error(1)
}

func (m *MockPodcastRepository) CreatePodcast(ctx context.Context, podcast *models.Podcast) error {
	args := m.Called(ctx, podcast)
	return args.Error(0)
}

func (m *MockPodcastRepository) UpsertPodcast(ctx context.Context, podcast *models.Podcast) error {
	args := m.Called(ctx, podcast)
	return args.Error(0)
}

func (m *MockPodcastRepository) UpdatePodcast(ctx context.Context, podcast *models.Podcast) error {
	args := m.Called(ctx, podcast)
	return args.Error(0)
}

func (m *MockPodcastRepository) UpdateLastFetched(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPodcastRepository) IncrementFetchCount(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Tests

func TestService_GetPodcastByPodcastIndexID_CacheHit(t *testing.T) {
	// Setup
	mockRepo := new(MockPodcastRepository)
	service := NewService(mockRepo, nil)

	now := time.Now()
	expectedPodcast := &models.Podcast{
		PodcastIndexID: 12345,
		Title:          "Test Podcast",
		FeedURL:        "https://example.com/feed.xml",
		LastFetchedAt:  &now,
	}
	expectedPodcast.ID = 1

	// Mock repository returns podcast (cache hit)
	mockRepo.On("GetPodcastByPodcastIndexID", mock.Anything, int64(12345)).Return(expectedPodcast, nil)
	mockRepo.On("UpdateLastFetched", mock.Anything, uint(1)).Return(nil)
	mockRepo.On("IncrementFetchCount", mock.Anything, uint(1)).Return(nil)

	// Execute
	podcast, err := service.GetPodcastByPodcastIndexID(context.Background(), 12345)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedPodcast, podcast)

	// Verify repository was called
	mockRepo.AssertExpectations(t)
}

func TestService_GetPodcastByPodcastIndexID_StaleData(t *testing.T) {
	// Setup
	mockRepo := new(MockPodcastRepository)
	service := NewService(mockRepo, nil)

	// Create stale podcast (older than 24 hours)
	twoWeeksAgo := time.Now().Add(-2 * 7 * 24 * time.Hour)
	stalePodcast := &models.Podcast{
		PodcastIndexID: 12345,
		Title:          "Old Title",
		FeedURL:        "https://example.com/feed.xml",
		LastFetchedAt:  &twoWeeksAgo,
	}
	stalePodcast.ID = 1

	// Mock repository returns stale podcast
	mockRepo.On("GetPodcastByPodcastIndexID", mock.Anything, int64(12345)).Return(stalePodcast, nil)
	mockRepo.On("UpdateLastFetched", mock.Anything, uint(1)).Return(nil)
	mockRepo.On("IncrementFetchCount", mock.Anything, uint(1)).Return(nil)

	// Execute
	podcast, err := service.GetPodcastByPodcastIndexID(context.Background(), 12345)

	// Assert - should return stale data immediately
	require.NoError(t, err)
	assert.Equal(t, stalePodcast, podcast)

	// Verify repository was called
	mockRepo.AssertExpectations(t)

	// Background refresh will happen but we can't easily test it here
	// (it's in a goroutine with a detached context)
}

func TestService_ShouldRefresh(t *testing.T) {
	service := &Service{
		refreshAfter: 24 * time.Hour,
	}

	tests := []struct {
		name     string
		podcast  *models.Podcast
		expected bool
	}{
		{
			name: "nil last fetched - should refresh",
			podcast: &models.Podcast{
				LastFetchedAt: nil,
			},
			expected: true,
		},
		{
			name: "fresh data - should not refresh",
			podcast: &models.Podcast{
				LastFetchedAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "stale data - should refresh",
			podcast: &models.Podcast{
				LastFetchedAt: func() *time.Time { t := time.Now().Add(-48 * time.Hour); return &t }(),
			},
			expected: true,
		},
		{
			name: "exactly at threshold - should refresh",
			podcast: &models.Podcast{
				LastFetchedAt: func() *time.Time { t := time.Now().Add(-24 * time.Hour); return &t }(),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.ShouldRefresh(tt.podcast)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_UpdatePodcastMetrics(t *testing.T) {
	// Setup
	mockRepo := new(MockPodcastRepository)
	service := NewService(mockRepo, nil)

	existingPodcast := &models.Podcast{
		PodcastIndexID: 12345,
		Title:          "Test Podcast",
		EpisodeCount:   0,
	}
	existingPodcast.ID = 1

	// Mock repository calls
	mockRepo.On("GetPodcastByID", mock.Anything, uint(1)).Return(existingPodcast, nil)
	mockRepo.On("UpdatePodcast", mock.Anything, mock.MatchedBy(func(p *models.Podcast) bool {
		return p.EpisodeCount == 50
	})).Return(nil)

	// Execute
	err := service.UpdatePodcastMetrics(context.Background(), 1, 50)

	// Assert
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_transformFromPodcastIndex(t *testing.T) {
	service := &Service{}

	// Test with comprehensive data
	categories := map[string]string{
		"1": "Technology",
		"2": "News",
	}
	piFeed := &podcastindex.Podcast{
		ID:               12345,
		Title:            "Test Podcast",
		Author:           "Test Author",
		OwnerName:        "Test Owner",
		Description:      "Test Description",
		URL:              "https://example.com/feed.xml",
		OriginalURL:      "https://example.com/original.xml",
		Link:             "https://example.com",
		Image:            "https://example.com/image.jpg",
		Artwork:          "https://example.com/artwork.jpg",
		ITunesID:         67890,
		Language:         "en",
		Categories:       categories,
		EpisodeCount:     100,
		LastUpdateTime:   time.Now().Unix(),
		LastCrawlTime:    time.Now().Unix(),
		LastParseTime:    time.Now().Unix(),
		LastGoodHTTPCode: 200,
		ImageURLHash:     123456789,
		Locked:           0,
	}

	// Execute
	podcast, err := service.transformFromPodcastIndex(piFeed)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(12345), podcast.PodcastIndexID)
	assert.Equal(t, "Test Podcast", podcast.Title)
	assert.Equal(t, "Test Author", podcast.Author)
	assert.Equal(t, "Test Owner", podcast.OwnerName)
	assert.Equal(t, "https://example.com/feed.xml", podcast.FeedURL)
	assert.NotNil(t, podcast.ITunesID)
	assert.Equal(t, int64(67890), *podcast.ITunesID)
	assert.Equal(t, "en", podcast.Language)
	assert.Equal(t, 100, podcast.EpisodeCount)

	// Verify categories JSON
	var cats map[string]string
	err = json.Unmarshal(podcast.Categories, &cats)
	require.NoError(t, err)
	assert.Len(t, cats, 2)
}

func TestService_transformFromPodcastIndex_NoITunesID(t *testing.T) {
	service := &Service{}

	piFeed := &podcastindex.Podcast{
		ID:       12345,
		Title:    "Test Podcast",
		URL:      "https://example.com/feed.xml",
		ITunesID: 0, // No iTunes ID
	}

	// Execute
	podcast, err := service.transformFromPodcastIndex(piFeed)

	// Assert
	require.NoError(t, err)
	assert.Nil(t, podcast.ITunesID)
}

func TestService_transformFromPodcastIndex_EmptyCategories(t *testing.T) {
	service := &Service{}

	piFeed := &podcastindex.Podcast{
		ID:         12345,
		Title:      "Test Podcast",
		URL:        "https://example.com/feed.xml",
		Categories: map[string]string{}, // Empty
	}

	// Execute
	podcast, err := service.transformFromPodcastIndex(piFeed)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, datatypes.JSON("{}"), podcast.Categories)
}

func TestService_GetByID(t *testing.T) {
	// Setup
	mockRepo := new(MockPodcastRepository)
	service := NewService(mockRepo, nil)

	expectedPodcast := &models.Podcast{
		PodcastIndexID: 12345,
		Title:          "Test Podcast",
	}
	expectedPodcast.ID = 1

	// Mock repository
	mockRepo.On("GetPodcastByID", mock.Anything, uint(1)).Return(expectedPodcast, nil)

	// Execute
	podcast, err := service.GetByID(context.Background(), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedPodcast, podcast)
	mockRepo.AssertExpectations(t)
}
