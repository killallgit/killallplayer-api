package audiocache

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// MockRepository is a mock implementation of Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Create(ctx context.Context, cache *models.AudioCache) error {
	args := m.Called(ctx, cache)
	return args.Error(0)
}

func (m *MockRepository) GetByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error) {
	args := m.Called(ctx, podcastIndexEpisodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AudioCache), args.Error(1)
}

func (m *MockRepository) GetBySHA256(ctx context.Context, sha256 string) (*models.AudioCache, error) {
	args := m.Called(ctx, sha256)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AudioCache), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, cache *models.AudioCache) error {
	args := m.Called(ctx, cache)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) GetOlderThan(ctx context.Context, olderThanDays int) ([]models.AudioCache, error) {
	args := m.Called(ctx, olderThanDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AudioCache), args.Error(1)
}

func (m *MockRepository) GetStats(ctx context.Context) (*CacheStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CacheStats), args.Error(1)
}

// MockStorageBackend is a mock implementation of StorageBackend
type MockStorageBackend struct {
	mock.Mock
}

func (m *MockStorageBackend) Save(ctx context.Context, data io.Reader, filename string) (string, error) {
	args := m.Called(ctx, data, filename)
	return args.String(0), args.Error(1)
}

func (m *MockStorageBackend) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorageBackend) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStorageBackend) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorageBackend) GetURL(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

func TestGetCachedAudio_UsesPodcastIndexID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageBackend)
	service := NewService(mockRepo, mockStorage)

	podcastIndexEpisodeID := int64(12345)
	expectedCache := &models.AudioCache{
		ID:                    1,
		PodcastIndexEpisodeID: podcastIndexEpisodeID,
		OriginalPath:          "/cache/original/12345_abc123.mp3",
		ProcessedPath:         "/cache/processed/12345_abc123_16khz.mp3",
		LastUsedAt:            time.Now().Add(-24 * time.Hour),
	}

	// Set up expectations
	mockRepo.On("GetByPodcastIndexEpisodeID", ctx, podcastIndexEpisodeID).Return(expectedCache, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*models.AudioCache")).Return(nil)

	// Act
	result, err := service.GetCachedAudio(ctx, podcastIndexEpisodeID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, podcastIndexEpisodeID, result.PodcastIndexEpisodeID)
	assert.Equal(t, expectedCache.OriginalPath, result.OriginalPath)

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}

func TestGetCachedAudio_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageBackend)
	service := NewService(mockRepo, mockStorage)

	podcastIndexEpisodeID := int64(99999)

	// Set up expectations - return ErrRecordNotFound
	mockRepo.On("GetByPodcastIndexEpisodeID", ctx, podcastIndexEpisodeID).Return(nil, gorm.ErrRecordNotFound)

	// Act
	result, err := service.GetCachedAudio(ctx, podcastIndexEpisodeID)

	// Assert
	assert.NoError(t, err) // Should return nil error for not found
	assert.Nil(t, result)  // Should return nil cache

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}

func TestUpdateLastUsed(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageBackend)
	service := NewService(mockRepo, mockStorage)

	cacheID := uint(1)

	// Set up expectations
	mockRepo.On("Update", ctx, mock.MatchedBy(func(cache *models.AudioCache) bool {
		return cache.ID == cacheID && !cache.LastUsedAt.IsZero()
	})).Return(nil)

	// Act
	err := service.UpdateLastUsed(ctx, cacheID)

	// Assert
	assert.NoError(t, err)

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}

func TestCleanupOldCache_UsesPodcastIndexID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageBackend)
	service := NewService(mockRepo, mockStorage)

	olderThanDays := 7
	oldCaches := []models.AudioCache{
		{
			ID:                    1,
			PodcastIndexEpisodeID: 11111,
			OriginalPath:          "/cache/original/11111_old.mp3",
			ProcessedPath:         "/cache/processed/11111_old_16khz.mp3",
		},
		{
			ID:                    2,
			PodcastIndexEpisodeID: 22222,
			OriginalPath:          "/cache/original/22222_old.mp3",
			ProcessedPath:         "/cache/processed/22222_old_16khz.mp3",
		},
	}

	// Set up expectations
	mockRepo.On("GetOlderThan", ctx, olderThanDays).Return(oldCaches, nil)

	for _, cache := range oldCaches {
		mockStorage.On("Delete", ctx, cache.OriginalPath).Return(nil)
		mockStorage.On("Delete", ctx, cache.ProcessedPath).Return(nil)
		mockRepo.On("Delete", ctx, cache.ID).Return(nil)
	}

	// Act
	err := service.CleanupOldCache(ctx, olderThanDays)

	// Assert
	assert.NoError(t, err)

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestGetCacheStats(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageBackend)
	service := NewService(mockRepo, mockStorage)

	expectedStats := &CacheStats{
		TotalEntries:    100,
		TotalSizeBytes:  1024 * 1024 * 500, // 500 MB
		OriginalSize:    1024 * 1024 * 300, // 300 MB
		ProcessedSize:   1024 * 1024 * 200, // 200 MB
		OldestEntry:     "2024-01-01T00:00:00Z",
		NewestEntry:     "2025-09-25T00:00:00Z",
		AverageDuration: 1800.5, // 30 minutes average
	}

	// Set up expectations
	mockRepo.On("GetStats", ctx).Return(expectedStats, nil)

	// Act
	result, err := service.GetCacheStats(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedStats.TotalEntries, result.TotalEntries)
	assert.Equal(t, expectedStats.TotalSizeBytes, result.TotalSizeBytes)
	assert.Equal(t, expectedStats.AverageDuration, result.AverageDuration)

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}
