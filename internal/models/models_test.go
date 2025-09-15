package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestUserModel(t *testing.T) {
	user := User{
		Model:        gorm.Model{},
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword123",
		IsActive:     true,
	}

	// Test field values
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hashedpassword123", user.PasswordHash)
	assert.True(t, user.IsActive)
}

func TestEpisodeModel(t *testing.T) {
	publishedAt := time.Now().Add(-24 * time.Hour)
	duration := 3600

	episode := Episode{
		Model:       gorm.Model{},
		PodcastID:   1,
		Title:       "Test Episode",
		Description: "A test episode description",
		AudioURL:    "https://example.com/episode.mp3",
		Duration:    &duration,
		GUID:        "episode-123",
		PublishedAt: publishedAt,
	}

	// Test field values
	assert.Equal(t, "Test Episode", episode.Title)
	assert.Equal(t, "A test episode description", episode.Description)
	assert.Equal(t, "https://example.com/episode.mp3", episode.AudioURL)
	assert.Equal(t, &duration, episode.Duration)
	assert.Equal(t, "episode-123", episode.GUID)
	assert.Equal(t, publishedAt, episode.PublishedAt)
	assert.Equal(t, uint(1), episode.PodcastID)
}

func TestPodcastModel(t *testing.T) {
	podcast := Podcast{
		Model:       gorm.Model{},
		Title:       "Test Podcast",
		Description: "A test podcast description",
		Author:      "Test Author",
		FeedURL:     "https://example.com/feed.xml",
		ImageURL:    "https://example.com/image.jpg",
		Language:    "en",
		Category:    "Technology",
	}

	// Test field values
	assert.Equal(t, "Test Podcast", podcast.Title)
	assert.Equal(t, "A test podcast description", podcast.Description)
	assert.Equal(t, "Test Author", podcast.Author)
	assert.Equal(t, "https://example.com/feed.xml", podcast.FeedURL)
	assert.Equal(t, "https://example.com/image.jpg", podcast.ImageURL)
	assert.Equal(t, "en", podcast.Language)
	assert.Equal(t, "Technology", podcast.Category)
}

func TestSubscriptionModel(t *testing.T) {
	subscription := Subscription{
		Model:     gorm.Model{},
		UserID:    1,
		PodcastID: 2,
	}

	// Test field values
	assert.Equal(t, uint(1), subscription.UserID)
	assert.Equal(t, uint(2), subscription.PodcastID)
}
