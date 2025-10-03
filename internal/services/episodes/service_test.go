package episodes

import (
	"context"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing

type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error) {
	args := m.Called(ctx, podcastID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PodcastIndexResponse), args.Error(1)
}

func (m *MockFetcher) GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EpisodeByGUIDResponse), args.Error(1)
}

func (m *MockFetcher) GetEpisodeByID(ctx context.Context, episodeID int64) (*PodcastIndexEpisode, error) {
	args := m.Called(ctx, episodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PodcastIndexEpisode), args.Error(1)
}

func (m *MockFetcher) GetEpisodeMetadata(ctx context.Context, episodeURL string) (*EpisodeMetadata, error) {
	args := m.Called(ctx, episodeURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EpisodeMetadata), args.Error(1)
}

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateEpisode(ctx context.Context, episode *models.Episode) error {
	args := m.Called(ctx, episode)
	return args.Error(0)
}

func (m *MockRepository) UpsertEpisode(ctx context.Context, episode *models.Episode) error {
	args := m.Called(ctx, episode)
	return args.Error(0)
}

func (m *MockRepository) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *MockRepository) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *MockRepository) GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error) {
	args := m.Called(ctx, podcastIndexID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Episode), args.Error(1)
}

func (m *MockRepository) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error) {
	args := m.Called(ctx, podcastID, page, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Episode), args.Get(1).(int64), args.Error(2)
}

func (m *MockRepository) GetEpisodesByPodcastIndexFeedID(ctx context.Context, feedID int64, page, limit int) ([]models.Episode, int64, error) {
	args := m.Called(ctx, feedID, page, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Episode), args.Get(1).(int64), args.Error(2)
}

func (m *MockRepository) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Episode), args.Error(1)
}

func (m *MockRepository) UpdateEpisode(ctx context.Context, episode *models.Episode) error {
	args := m.Called(ctx, episode)
	return args.Error(0)
}

func (m *MockRepository) MarkEpisodeAsPlayed(ctx context.Context, id uint, played bool) error {
	args := m.Called(ctx, id, played)
	return args.Error(0)
}

func (m *MockRepository) UpdatePlaybackPosition(ctx context.Context, id uint, position int) error {
	args := m.Called(ctx, id, position)
	return args.Error(0)
}

func (m *MockRepository) DeleteEpisode(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) GetEpisode(key string) (*models.Episode, bool) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*models.Episode), args.Bool(1)
}

func (m *MockCache) SetEpisode(key string, episode *models.Episode) {
	m.Called(key, episode)
}

func (m *MockCache) GetEpisodeList(key string) ([]models.Episode, int64, bool) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, 0, args.Bool(2)
	}
	return args.Get(0).([]models.Episode), args.Get(1).(int64), args.Bool(2)
}

func (m *MockCache) SetEpisodeList(key string, episodes []models.Episode, total int64) {
	m.Called(key, episodes, total)
}

func (m *MockCache) Invalidate(key string) {
	m.Called(key)
}

func (m *MockCache) InvalidatePattern(pattern string) {
	m.Called(pattern)
}

func (m *MockCache) Clear() {
	m.Called()
}

func (m *MockCache) Stop() {
	m.Called()
}

// Tests

func TestService_GetEpisodeByID_CacheHit(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	expectedEpisode := &models.Episode{
		Title: "Test Episode",
		GUID:  "test-guid",
	}
	expectedEpisode.ID = 1

	// Mock cache hit
	mockCache.On("GetEpisode", "episode:id:1").Return(expectedEpisode, true)

	// Execute
	episode, err := service.GetEpisodeByID(context.Background(), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedEpisode, episode)

	// Verify repository was not called (cache hit)
	mockRepo.AssertNotCalled(t, "GetEpisodeByID")
	mockCache.AssertExpectations(t)
}

func TestService_GetEpisodeByID_CacheMiss(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	expectedEpisode := &models.Episode{
		Title: "Test Episode",
		GUID:  "test-guid",
	}
	expectedEpisode.ID = 1

	// Mock cache miss
	mockCache.On("GetEpisode", "episode:id:1").Return(nil, false)

	// Mock repository call
	mockRepo.On("GetEpisodeByID", mock.Anything, uint(1)).Return(expectedEpisode, nil)

	// Mock cache set
	mockCache.On("SetEpisode", "episode:id:1", expectedEpisode).Return()

	// Execute
	episode, err := service.GetEpisodeByID(context.Background(), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedEpisode, episode)

	// Verify all mocks were called
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_GetEpisodeByID_NotFound(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	// Mock cache miss
	mockCache.On("GetEpisode", "episode:id:999").Return(nil, false)

	// Mock repository returns not found
	mockRepo.On("GetEpisodeByID", mock.Anything, uint(999)).Return(nil, NewNotFoundError("episode", 999))

	// Execute
	episode, err := service.GetEpisodeByID(context.Background(), 999)

	// Assert
	require.Error(t, err)
	assert.Nil(t, episode)
	assert.True(t, IsNotFound(err))

	// Verify cache set was not called on error
	mockCache.AssertNotCalled(t, "SetEpisode")
}

func TestService_FetchAndSyncEpisodes(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	// Create test response
	duration := 3600
	testResponse := &PodcastIndexResponse{
		Status: "true",
		Items: []PodcastIndexEpisode{
			{
				ID:           12345,
				Title:        "Episode 1",
				GUID:         "guid-1",
				EnclosureURL: "https://example.com/ep1.mp3",
				Duration:     &duration,
			},
		},
		Count: 1,
	}

	// Mock fetcher call
	mockFetcher.On("GetEpisodesByPodcastID", mock.Anything, int64(100), 20).Return(testResponse, nil)

	// Mock repository calls for the background sync
	// The sync goroutine will check if episode exists by GUID
	mockRepo.On("GetEpisodeByGUID", mock.Anything, "guid-1").Return(nil, NewNotFoundError("episode", "guid-1"))

	// Since episode doesn't exist, it will create it
	mockRepo.On("CreateEpisode", mock.Anything, mock.AnythingOfType("*models.Episode")).Return(nil)

	// Mock cache invalidation
	mockCache.On("InvalidatePattern", mock.AnythingOfType("string")).Return()

	// Execute
	response, err := service.FetchAndSyncEpisodes(context.Background(), 100, 20)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, testResponse, response)

	// Wait for background sync to complete
	time.Sleep(50 * time.Millisecond)

	// Verify all mocks were called
	mockFetcher.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_FetchAndSyncEpisodes_UpdateExisting(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	// Create test response
	duration := 3600
	testResponse := &PodcastIndexResponse{
		Status: "true",
		Items: []PodcastIndexEpisode{
			{
				ID:           12345,
				Title:        "Updated Episode 1",
				GUID:         "guid-1",
				EnclosureURL: "https://example.com/ep1.mp3",
				Duration:     &duration,
			},
		},
		Count: 1,
	}

	// Create existing episode
	existingEpisode := &models.Episode{
		Title: "Old Episode 1",
		GUID:  "guid-1",
	}
	existingEpisode.ID = 1

	// Mock fetcher call
	mockFetcher.On("GetEpisodesByPodcastID", mock.Anything, int64(100), 20).Return(testResponse, nil)

	// Mock repository calls for the background sync
	// The sync goroutine will check if episode exists by GUID
	mockRepo.On("GetEpisodeByGUID", mock.Anything, "guid-1").Return(existingEpisode, nil)

	// Since episode exists, it will update it
	mockRepo.On("UpdateEpisode", mock.Anything, mock.AnythingOfType("*models.Episode")).Return(nil)

	// Mock cache invalidation
	mockCache.On("InvalidatePattern", mock.AnythingOfType("string")).Return()

	// Execute
	response, err := service.FetchAndSyncEpisodes(context.Background(), 100, 20)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, testResponse, response)

	// Wait for background sync to complete
	time.Sleep(50 * time.Millisecond)

	// Verify all mocks were called
	mockFetcher.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_GetRecentEpisodes_CacheHit(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	mockFetcher := new(MockFetcher)

	service := NewService(mockFetcher, mockRepo, mockCache, nil)

	expectedEpisodes := []models.Episode{
		{Title: "Episode 1"},
		{Title: "Episode 2"},
	}

	// Mock cache hit
	mockCache.On("GetEpisodeList", "episode:recent:limit:10").Return(expectedEpisodes, int64(2), true)

	// Execute
	episodes, err := service.GetRecentEpisodes(context.Background(), 10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedEpisodes, episodes)

	// Verify repository was not called
	mockRepo.AssertNotCalled(t, "GetRecentEpisodes")
}
