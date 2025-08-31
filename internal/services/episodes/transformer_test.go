package episodes

import (
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformer_ModelToPodcastIndex(t *testing.T) {
	transformer := NewTransformer()

	// Create a test episode with all fields
	duration := 3600
	episodeNum := 5
	seasonNum := 2
	episode := &models.Episode{
		PodcastID:       100,
		PodcastIndexID:  12345,
		Title:           "Test Episode",
		Description:     "Test Description",
		AudioURL:        "https://example.com/episode.mp3",
		Duration:        &duration,
		PublishedAt:     time.Unix(1609459200, 0),
		GUID:            "test-guid-123",
		Link:            "https://example.com/episode",
		Image:           "https://example.com/image.jpg",
		Explicit:        1,
		EpisodeNumber:   &episodeNum,
		Season:          &seasonNum,
		EnclosureType:   "audio/mpeg",
		EnclosureLength: 52428800,
		FeedLanguage:    "en",
		ChaptersURL:     "https://example.com/chapters.json",
		TranscriptURL:   "https://example.com/transcript.vtt",
	}
	episode.ID = 1 // Set ID through embedded gorm.Model

	// Transform to Podcast Index format
	piEpisode := transformer.ModelToPodcastIndex(episode)

	// Verify all fields are correctly mapped
	assert.Equal(t, episode.PodcastIndexID, piEpisode.ID)
	assert.Equal(t, episode.Title, piEpisode.Title)
	assert.Equal(t, episode.Description, piEpisode.Description)
	assert.Equal(t, episode.AudioURL, piEpisode.EnclosureURL)
	assert.Equal(t, episode.Duration, piEpisode.Duration)
	assert.Equal(t, episode.PublishedAt.Unix(), piEpisode.DatePublished)
	assert.Equal(t, episode.GUID, piEpisode.GUID)
	assert.Equal(t, episode.Link, piEpisode.Link)
	assert.Equal(t, episode.Image, piEpisode.Image)
	assert.Equal(t, episode.Explicit, piEpisode.Explicit)
	assert.Equal(t, episode.EpisodeNumber, piEpisode.Episode)
	assert.Equal(t, episode.Season, piEpisode.Season)
	assert.Equal(t, episode.EnclosureType, piEpisode.EnclosureType)
	assert.Equal(t, episode.EnclosureLength, piEpisode.EnclosureLength)
	assert.Equal(t, episode.FeedLanguage, piEpisode.FeedLanguage)
	assert.Equal(t, episode.ChaptersURL, piEpisode.ChaptersURL)
	assert.Equal(t, episode.TranscriptURL, piEpisode.TranscriptURL)
	assert.Equal(t, int64(episode.PodcastID), piEpisode.FeedID)
}

func TestTransformer_PodcastIndexToModel(t *testing.T) {
	transformer := NewTransformer()

	// Create a test Podcast Index episode
	duration := 3600
	episodeNum := 5
	seasonNum := 2
	piEpisode := PodcastIndexEpisode{
		ID:              12345,
		Title:           "Test Episode",
		Description:     "Test Description",
		EnclosureURL:    "https://example.com/episode.mp3",
		Duration:        &duration,
		DatePublished:   1609459200,
		GUID:            "test-guid-123",
		Link:            "https://example.com/episode",
		Image:           "https://example.com/image.jpg",
		Explicit:        1,
		Episode:         &episodeNum,
		Season:          &seasonNum,
		EnclosureType:   "audio/mpeg",
		EnclosureLength: 52428800,
		FeedLanguage:    "en",
		ChaptersURL:     "https://example.com/chapters.json",
		TranscriptURL:   "https://example.com/transcript.vtt",
		FeedID:          100,
	}

	// Transform to model format
	episode := transformer.PodcastIndexToModel(piEpisode, 100)

	// Verify all fields are correctly mapped
	assert.Equal(t, uint(100), episode.PodcastID)
	assert.Equal(t, piEpisode.ID, episode.PodcastIndexID)
	assert.Equal(t, piEpisode.Title, episode.Title)
	assert.Equal(t, piEpisode.Description, episode.Description)
	assert.Equal(t, piEpisode.EnclosureURL, episode.AudioURL)
	assert.Equal(t, piEpisode.Duration, episode.Duration)
	assert.Equal(t, time.Unix(piEpisode.DatePublished, 0), episode.PublishedAt)
	assert.Equal(t, piEpisode.GUID, episode.GUID)
	assert.Equal(t, piEpisode.Link, episode.Link)
	assert.Equal(t, piEpisode.Image, episode.Image)
	assert.Equal(t, piEpisode.Explicit, episode.Explicit)
	assert.Equal(t, piEpisode.Episode, episode.EpisodeNumber)
	assert.Equal(t, piEpisode.Season, episode.Season)
	assert.Equal(t, piEpisode.EnclosureType, episode.EnclosureType)
	assert.Equal(t, piEpisode.EnclosureLength, episode.EnclosureLength)
	assert.Equal(t, piEpisode.FeedLanguage, episode.FeedLanguage)
	assert.Equal(t, piEpisode.ChaptersURL, episode.ChaptersURL)
	assert.Equal(t, piEpisode.TranscriptURL, episode.TranscriptURL)
}

func TestTransformer_CreateSuccessResponse(t *testing.T) {
	transformer := NewTransformer()

	// Create test episodes
	duration1 := 3600
	duration2 := 1800
	episodes := []models.Episode{
		{
			PodcastID:      100,
			PodcastIndexID: 12345,
			Title:          "Episode 1",
			AudioURL:       "https://example.com/ep1.mp3",
			Duration:       &duration1,
			GUID:           "guid-1",
		},
		{
			PodcastID:      100,
			PodcastIndexID: 12346,
			Title:          "Episode 2",
			AudioURL:       "https://example.com/ep2.mp3",
			Duration:       &duration2,
			GUID:           "guid-2",
		},
	}
	episodes[0].ID = 1 // Set ID through embedded gorm.Model
	episodes[1].ID = 2

	// Create response
	response := transformer.CreateSuccessResponse(episodes, "Test description")

	// Verify response structure
	assert.Equal(t, "true", response.Status)
	assert.Equal(t, "Test description", response.Description)
	assert.Equal(t, 2, response.Count)
	assert.Len(t, response.Items, 2)

	// Verify episodes are transformed correctly
	assert.Equal(t, int64(12345), response.Items[0].ID)
	assert.Equal(t, "Episode 1", response.Items[0].Title)
	assert.Equal(t, int64(12346), response.Items[1].ID)
	assert.Equal(t, "Episode 2", response.Items[1].Title)
}

func TestTransformer_CreateSingleEpisodeResponse(t *testing.T) {
	transformer := NewTransformer()

	// Create test episode
	duration := 3600
	episode := &models.Episode{
		PodcastID:      100,
		PodcastIndexID: 12345,
		Title:          "Test Episode",
		AudioURL:       "https://example.com/episode.mp3",
		Duration:       &duration,
		GUID:           "test-guid",
	}
	episode.ID = 1 // Set ID through embedded gorm.Model

	// Create response
	response := transformer.CreateSingleEpisodeResponse(episode)

	// Verify response structure
	assert.Equal(t, "true", response.Status)
	assert.Contains(t, response.Description, "Episode found")
	require.NotNil(t, response.Episode)
	assert.Equal(t, int64(12345), response.Episode.ID)
	assert.Equal(t, "Test Episode", response.Episode.Title)
	assert.Equal(t, "test-guid", response.Episode.GUID)
}

func TestTransformer_CreateErrorResponse(t *testing.T) {
	transformer := NewTransformer()

	// Create error response
	response := transformer.CreateErrorResponse("Test error message")

	// Verify error response structure
	assert.Equal(t, "false", response.Status)
	assert.Equal(t, "Test error message", response.Description)
}

func TestTransformer_NilDurationHandling(t *testing.T) {
	transformer := NewTransformer()

	// Create episode with nil duration
	episode := &models.Episode{
		PodcastID:      100,
		PodcastIndexID: 12345,
		Title:          "Episode without duration",
		AudioURL:       "https://example.com/episode.mp3",
		Duration:       nil, // nil duration
		GUID:           "test-guid",
	}
	episode.ID = 1 // Set ID through embedded gorm.Model

	// Transform should handle nil duration gracefully
	piEpisode := transformer.ModelToPodcastIndex(episode)
	assert.Nil(t, piEpisode.Duration)

	// Create response with nil duration episode
	response := transformer.CreateSingleEpisodeResponse(episode)
	assert.Equal(t, "true", response.Status)
	assert.Nil(t, response.Episode.Duration)
}

func TestTransformer_EmptyEpisodesList(t *testing.T) {
	transformer := NewTransformer()

	// Create response with empty episodes list
	response := transformer.CreateSuccessResponse([]models.Episode{}, "No episodes found")

	// Verify response handles empty list correctly
	assert.Equal(t, "true", response.Status)
	assert.Equal(t, "No episodes found", response.Description)
	assert.Equal(t, 0, response.Count)
	assert.Empty(t, response.Items)
}
