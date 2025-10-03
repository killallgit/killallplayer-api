package episodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	clips "github.com/killallgit/player-api/internal/services/clips"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Simple in-memory clip service for testing
type testClipService struct {
	db *gorm.DB
}

func (s *testClipService) CreateClip(ctx context.Context, params clips.CreateClipParams) (*models.Clip, error) {
	// Not used in these tests
	return nil, fmt.Errorf("not implemented")
}

func (s *testClipService) GetClip(ctx context.Context, uuid string) (*models.Clip, error) {
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		return nil, err
	}
	return &clip, nil
}

func (s *testClipService) GetClipsByEpisodeID(ctx context.Context, episodeID int64) ([]*models.Clip, error) {
	var clips []*models.Clip
	if err := s.db.Where("podcast_index_episode_id = ?", episodeID).
		Order("original_start_time ASC").
		Find(&clips).Error; err != nil {
		return nil, err
	}
	return clips, nil
}

func (s *testClipService) UpdateClipLabel(ctx context.Context, uuid, newLabel string) (*models.Clip, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *testClipService) ApproveClip(ctx context.Context, uuid string) (*models.Clip, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *testClipService) DeleteClip(ctx context.Context, uuid string) error {
	return fmt.Errorf("not implemented")
}

func (s *testClipService) ListClips(ctx context.Context, filters clips.ListClipsFilters) ([]*models.Clip, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *testClipService) ExportDataset(ctx context.Context, exportPath string) error {
	return fmt.Errorf("not implemented")
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Migrate
	err = db.AutoMigrate(&models.Clip{})
	require.NoError(t, err)

	return db
}

func TestGetClips_ReturnsExistingClips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup in-memory database
	db := setupTestDB(t)

	clipService := &testClipService{db: db}

	deps := &types.Dependencies{
		ClipService: clipService,
	}

	// Create test clips
	episodeID := int64(12345)
	filename := "clip_test.wav"
	duration := 15.0
	size := int64(480078)
	confidence := 0.85

	clips := []models.Clip{
		{
			UUID:                  "clip-uuid-1",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
			AutoLabeled:           true,
			LabelConfidence:       &confidence,
			LabelMethod:           "peak_detection",
			ClipFilename:          &filename,
			ClipDuration:          &duration,
			ClipSizeBytes:         &size,
			Extracted:             true,
			Status:                "ready",
		},
		{
			UUID:                  "clip-uuid-2",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     120.0,
			OriginalEndTime:       135.0,
			Label:                 "advertisement",
			AutoLabeled:           true,
			LabelConfidence:       nil,
			LabelMethod:           "peak_detection",
			ClipFilename:          nil,
			ClipDuration:          nil,
			ClipSizeBytes:         nil,
			Extracted:             false,
			Status:                "processing",
		},
	}

	for _, clip := range clips {
		err := db.Create(&clip).Error
		require.NoError(t, err)
	}

	// Create request
	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/clips", episodeID), nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response ClipsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, types.StatusOK, response.Status)
	assert.Equal(t, episodeID, response.EpisodeID)
	assert.Len(t, response.Clips, 2)

	// Verify first clip (extracted)
	assert.Equal(t, "clip-uuid-1", response.Clips[0].UUID)
	assert.Equal(t, 30.0, response.Clips[0].StartTime)
	assert.Equal(t, 45.0, response.Clips[0].EndTime)
	assert.Equal(t, "advertisement", response.Clips[0].Label)
	assert.True(t, response.Clips[0].AutoLabeled)
	assert.True(t, response.Clips[0].Extracted)
	assert.NotNil(t, response.Clips[0].Confidence)
	assert.Equal(t, 0.85, *response.Clips[0].Confidence)

	// Verify second clip (auto-detected, not extracted)
	assert.Equal(t, "clip-uuid-2", response.Clips[1].UUID)
	assert.Equal(t, 120.0, response.Clips[1].StartTime)
	assert.Equal(t, 135.0, response.Clips[1].EndTime)
	assert.False(t, response.Clips[1].Extracted)
	assert.Nil(t, response.Clips[1].Confidence)
}

func TestGetClips_ReturnsEmptyArrayWhenNoClips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup in-memory database
	db := setupTestDB(t)

	clipService := &testClipService{db: db}

	deps := &types.Dependencies{
		ClipService: clipService,
		JobService:  nil, // No job service - should still return empty array
	}

	episodeID := int64(99999)

	// Create request
	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/clips", episodeID), nil)
	router.ServeHTTP(w, req)

	// Assert - should return 202 since no job service available to queue job
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response ClipsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, episodeID, response.EpisodeID)
	assert.Empty(t, response.Clips)
}

func TestGetClips_InvalidEpisodeID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deps := &types.Dependencies{}

	tests := []struct {
		name       string
		episodeID  string
		wantStatus int
	}{
		{"negative ID", "-1", http.StatusBadRequest},
		{"non-numeric ID", "abc", http.StatusBadRequest},
		{"zero ID", "0", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%s/clips", tt.episodeID), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response types.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, types.StatusError, response.Status)
		})
	}
}

func TestGetClips_ClipServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	deps := &types.Dependencies{
		ClipService: nil, // Service not available
	}

	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/episodes/12345/clips", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, types.StatusError, response.Status)
	assert.Contains(t, response.Message, "not available")
}

func TestGetClips_OrderedByStartTime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup in-memory database
	db := setupTestDB(t)

	clipService := &testClipService{db: db}

	deps := &types.Dependencies{
		ClipService: clipService,
	}

	episodeID := int64(12345)

	// Create clips in non-sequential order by start time
	clips := []models.Clip{
		{
			UUID:                  "clip-2",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     120.0,
			OriginalEndTime:       135.0,
			Label:                 "music",
			AutoLabeled:           true,
			Status:                "ready",
		},
		{
			UUID:                  "clip-1",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
			AutoLabeled:           true,
			Status:                "ready",
		},
		{
			UUID:                  "clip-3",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     60.0,
			OriginalEndTime:       75.0,
			Label:                 "advertisement",
			AutoLabeled:           true,
			Status:                "ready",
		},
	}

	for _, clip := range clips {
		err := db.Create(&clip).Error
		require.NoError(t, err)
	}

	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/clips", episodeID), nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response ClipsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify clips are ordered by start time
	assert.Len(t, response.Clips, 3)
	assert.Equal(t, 30.0, response.Clips[0].StartTime, "First clip should start at 30s")
	assert.Equal(t, 60.0, response.Clips[1].StartTime, "Second clip should start at 60s")
	assert.Equal(t, 120.0, response.Clips[2].StartTime, "Third clip should start at 120s")
}

func TestGetClips_OnlyReturnsClipsForSpecifiedEpisode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup in-memory database
	db := setupTestDB(t)

	clipService := &testClipService{db: db}

	deps := &types.Dependencies{
		ClipService: clipService,
	}

	episode1 := int64(11111)
	episode2 := int64(22222)

	// Create clips for both episodes
	clips := []models.Clip{
		{
			UUID:                  "episode1-clip1",
			PodcastIndexEpisodeID: episode1,
			SourceEpisodeURL:      "https://example.com/episode1.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
		},
		{
			UUID:                  "episode1-clip2",
			PodcastIndexEpisodeID: episode1,
			SourceEpisodeURL:      "https://example.com/episode1.mp3",
			OriginalStartTime:     60.0,
			OriginalEndTime:       75.0,
			Label:                 "music",
		},
		{
			UUID:                  "episode2-clip1",
			PodcastIndexEpisodeID: episode2,
			SourceEpisodeURL:      "https://example.com/episode2.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
		},
	}

	for _, clip := range clips {
		err := db.Create(&clip).Error
		require.NoError(t, err)
	}

	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	// Request clips for episode 1
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/clips", episode1), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ClipsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should only return clips for episode 1
	assert.Len(t, response.Clips, 2)
	assert.Equal(t, "episode1-clip1", response.Clips[0].UUID)
	assert.Equal(t, "episode1-clip2", response.Clips[1].UUID)
}

func TestGetClips_HandlesBothExtractedAndNonExtractedClips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup in-memory database
	db := setupTestDB(t)

	clipService := &testClipService{db: db}

	deps := &types.Dependencies{
		ClipService: clipService,
	}

	episodeID := int64(33333)
	filename := "clip_test.wav"
	duration := 15.0
	size := int64(480078)

	// Create both types of clips
	clips := []models.Clip{
		{
			UUID:                  "extracted-clip",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
			AutoLabeled:           false,
			ClipFilename:          &filename,
			ClipDuration:          &duration,
			ClipSizeBytes:         &size,
			Extracted:             true,
			Status:                "ready",
		},
		{
			UUID:                  "auto-detected-clip",
			PodcastIndexEpisodeID: episodeID,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     60.0,
			OriginalEndTime:       75.0,
			Label:                 "music",
			AutoLabeled:           true,
			ClipFilename:          nil,
			ClipDuration:          nil,
			ClipSizeBytes:         nil,
			Extracted:             false,
			Status:                "processing",
		},
	}

	for _, clip := range clips {
		err := db.Create(&clip).Error
		require.NoError(t, err)
	}

	router := gin.New()
	router.GET("/api/v1/episodes/:id/clips", GetClips(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/clips", episodeID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ClipsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Clips, 2)

	// Verify extracted clip
	assert.True(t, response.Clips[0].Extracted)
	assert.False(t, response.Clips[0].AutoLabeled)

	// Verify auto-detected clip
	assert.False(t, response.Clips[1].Extracted)
	assert.True(t, response.Clips[1].AutoLabeled)
}
