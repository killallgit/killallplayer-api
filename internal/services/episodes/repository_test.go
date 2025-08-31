package episodes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Episode{}, &models.Podcast{})
	require.NoError(t, err)

	return db
}

func TestRepository_CreateEpisode(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	duration := 3600
	episode := &models.Episode{
		PodcastID:   1,
		Title:       "Test Episode",
		Description: "Test Description",
		AudioURL:    "https://example.com/test.mp3",
		Duration:    &duration,
		PublishedAt: time.Now(),
		GUID:        "test-guid-123",
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)
	assert.NotZero(t, episode.ID)

	// Verify the episode was created
	var retrieved models.Episode
	err = db.First(&retrieved, episode.ID).Error
	require.NoError(t, err)
	assert.Equal(t, episode.Title, retrieved.Title)
	assert.Equal(t, episode.GUID, retrieved.GUID)
}

func TestRepository_UpdateEpisode(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode first
	duration := 3600
	episode := &models.Episode{
		PodcastID:   1,
		Title:       "Original Title",
		Description: "Original Description",
		AudioURL:    "https://example.com/original.mp3",
		Duration:    &duration,
		GUID:        "update-test-guid",
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Update the episode
	episode.Title = "Updated Title"
	episode.Description = "Updated Description"
	newDuration := 7200
	episode.Duration = &newDuration

	err = repo.UpdateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Verify the update
	var retrieved models.Episode
	err = db.First(&retrieved, episode.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title)
	assert.Equal(t, "Updated Description", retrieved.Description)
	require.NotNil(t, retrieved.Duration)
	assert.Equal(t, 7200, *retrieved.Duration)
}

func TestRepository_GetEpisodeByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode
	episode := &models.Episode{
		PodcastID:   1,
		Title:       "Get By ID Test",
		Description: "Test Description",
		AudioURL:    "https://example.com/test.mp3",
		GUID:        "get-by-id-guid",
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Get the episode by ID
	retrieved, err := repo.GetEpisodeByID(context.Background(), episode.ID)
	require.NoError(t, err)
	assert.Equal(t, episode.Title, retrieved.Title)
	assert.Equal(t, episode.GUID, retrieved.GUID)

	// Test non-existent ID
	_, err = repo.GetEpisodeByID(context.Background(), 999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRepository_GetEpisodeByGUID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode
	episode := &models.Episode{
		PodcastID:   1,
		Title:       "Get By GUID Test",
		Description: "Test Description",
		AudioURL:    "https://example.com/test.mp3",
		GUID:        "unique-guid-123",
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Get the episode by GUID
	retrieved, err := repo.GetEpisodeByGUID(context.Background(), "unique-guid-123")
	require.NoError(t, err)
	assert.Equal(t, episode.Title, retrieved.Title)
	assert.Equal(t, episode.ID, retrieved.ID)

	// Test non-existent GUID
	_, err = repo.GetEpisodeByGUID(context.Background(), "non-existent-guid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRepository_GetEpisodesByPodcastID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create multiple episodes for the same podcast
	podcastID := uint(1)
	for i := 1; i <= 5; i++ {
		episode := &models.Episode{
			PodcastID:      podcastID,
			PodcastIndexID: int64(i * 1000), // Unique ID for each episode
			Title:          fmt.Sprintf("Episode %d", i),
			AudioURL:       fmt.Sprintf("https://example.com/episode%d.mp3", i),
			GUID:           fmt.Sprintf("guid-%d", i),
			PublishedAt:    time.Now().Add(time.Duration(-i) * time.Hour),
		}
		err := repo.CreateEpisode(context.Background(), episode)
		require.NoError(t, err)
	}

	// Get first page
	episodes, total, err := repo.GetEpisodesByPodcastID(context.Background(), podcastID, 1, 3)
	require.NoError(t, err)
	assert.Len(t, episodes, 3)
	assert.Equal(t, int64(5), total)

	// Episodes should be ordered by published_at DESC
	assert.Equal(t, "Episode 1", episodes[0].Title)
	assert.Equal(t, "Episode 2", episodes[1].Title)
	assert.Equal(t, "Episode 3", episodes[2].Title)

	// Get second page
	episodes, total, err = repo.GetEpisodesByPodcastID(context.Background(), podcastID, 2, 3)
	require.NoError(t, err)
	assert.Len(t, episodes, 2)
	assert.Equal(t, int64(5), total)
	assert.Equal(t, "Episode 4", episodes[0].Title)
	assert.Equal(t, "Episode 5", episodes[1].Title)
}

func TestRepository_MarkEpisodeAsPlayed(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode
	episode := &models.Episode{
		PodcastID: 1,
		Title:     "Playback Test",
		AudioURL:  "https://example.com/test.mp3",
		GUID:      "playback-guid",
		Played:    false,
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Mark as played
	err = repo.MarkEpisodeAsPlayed(context.Background(), episode.ID, true)
	require.NoError(t, err)

	// Verify
	var retrieved models.Episode
	err = db.First(&retrieved, episode.ID).Error
	require.NoError(t, err)
	assert.True(t, retrieved.Played)

	// Mark as unplayed
	err = repo.MarkEpisodeAsPlayed(context.Background(), episode.ID, false)
	require.NoError(t, err)

	err = db.First(&retrieved, episode.ID).Error
	require.NoError(t, err)
	assert.False(t, retrieved.Played)
}

func TestRepository_UpdatePlaybackPosition(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode
	episode := &models.Episode{
		PodcastID: 1,
		Title:     "Position Test",
		AudioURL:  "https://example.com/test.mp3",
		GUID:      "position-guid",
		Position:  0,
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Update position
	err = repo.UpdatePlaybackPosition(context.Background(), episode.ID, 1234)
	require.NoError(t, err)

	// Verify
	var retrieved models.Episode
	err = db.First(&retrieved, episode.ID).Error
	require.NoError(t, err)
	assert.Equal(t, 1234, retrieved.Position)
}

func TestRepository_UpsertEpisode(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// First upsert (insert)
	episode := &models.Episode{
		PodcastID:   1,
		Title:       "Upsert Test",
		Description: "Original",
		AudioURL:    "https://example.com/test.mp3",
		GUID:        "upsert-guid",
	}

	err := repo.UpsertEpisode(context.Background(), episode)
	require.NoError(t, err)
	originalID := episode.ID

	// Second upsert (update)
	episode2 := &models.Episode{
		PodcastID:   1,
		Title:       "Updated Title",
		Description: "Updated",
		AudioURL:    "https://example.com/updated.mp3",
		GUID:        "upsert-guid", // Same GUID
	}

	err = repo.UpsertEpisode(context.Background(), episode2)
	require.NoError(t, err)
	assert.Equal(t, originalID, episode2.ID) // Should have same ID

	// Verify update
	var retrieved models.Episode
	err = db.First(&retrieved, originalID).Error
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title)
	assert.Equal(t, "Updated", retrieved.Description)
}

func TestRepository_DeleteEpisode(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create an episode
	episode := &models.Episode{
		PodcastID: 1,
		Title:     "Delete Test",
		AudioURL:  "https://example.com/test.mp3",
		GUID:      "delete-guid",
	}

	err := repo.CreateEpisode(context.Background(), episode)
	require.NoError(t, err)

	// Delete the episode
	err = repo.DeleteEpisode(context.Background(), episode.ID)
	require.NoError(t, err)

	// Verify deletion
	var count int64
	db.Model(&models.Episode{}).Where("id = ?", episode.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Try to delete non-existent episode
	err = repo.DeleteEpisode(context.Background(), 999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
