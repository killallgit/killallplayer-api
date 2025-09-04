package episodes

import (
	"fmt"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

// Transformer handles conversion between database models and API responses
type Transformer struct{}

// Ensure Transformer implements EpisodeTransformer interface
var _ EpisodeTransformer = (*Transformer)(nil)

// NewTransformer creates a new transformer instance
func NewTransformer() *Transformer {
	return &Transformer{}
}

// ModelToPodcastIndex converts a database Episode model to Podcast Index API format
func (t *Transformer) ModelToPodcastIndex(episode *models.Episode) PodcastIndexEpisode {
	pie := PodcastIndexEpisode{
		ID:                  episode.PodcastIndexID,
		Title:               episode.Title,
		Link:                episode.Link,
		Description:         episode.Description,
		GUID:                episode.GUID,
		DatePublished:       episode.PublishedAt.Unix(),
		DatePublishedPretty: episode.PublishedAt.Format("January 2, 2006 3:04pm"),
		EnclosureURL:        episode.AudioURL,
		EnclosureType:       episode.EnclosureType,
		EnclosureLength:     episode.EnclosureLength,
		Duration:            episode.Duration,
		Explicit:            episode.Explicit,
		Episode:             episode.EpisodeNumber,
		EpisodeType:         episode.EpisodeType,
		Season:              episode.Season,
		Image:               episode.Image,
		FeedID:              int64(episode.PodcastID),
		FeedTitle:           episode.FeedTitle,
		FeedImage:           episode.FeedImage,
		FeedLanguage:        episode.FeedLanguage,
		FeedItunesID:        episode.FeedItunesID,
		ChaptersURL:         episode.ChaptersURL,
		TranscriptURL:       episode.TranscriptURL,
	}

	// Add date crawled if available
	if !episode.DateCrawled.IsZero() {
		pie.DateCrawled = episode.DateCrawled.Unix()
	}

	// Use internal ID if PodcastIndexID is not set
	if pie.ID == 0 {
		pie.ID = int64(episode.ID)
	}

	return pie
}

// PodcastIndexToModel converts a Podcast Index API episode to database model
func (t *Transformer) PodcastIndexToModel(pie PodcastIndexEpisode, podcastID uint) *models.Episode {
	episode := &models.Episode{
		PodcastID:       podcastID,
		PodcastIndexID:  pie.ID,
		Title:           pie.Title,
		Description:     pie.Description,
		Link:            pie.Link,
		GUID:            pie.GUID,
		AudioURL:        pie.EnclosureURL,
		EnclosureType:   pie.EnclosureType,
		EnclosureLength: pie.EnclosureLength,
		Duration:        pie.Duration,
		Explicit:        pie.Explicit,
		EpisodeNumber:   pie.Episode,
		Season:          pie.Season,
		EpisodeType:     pie.EpisodeType,
		Image:           pie.Image,
		FeedTitle:       pie.FeedTitle,
		FeedImage:       pie.FeedImage,
		FeedLanguage:    pie.FeedLanguage,
		FeedItunesID:    pie.FeedItunesID,
		ChaptersURL:     pie.ChaptersURL,
		TranscriptURL:   pie.TranscriptURL,
	}

	// Convert Unix timestamps to time.Time
	if pie.DatePublished > 0 {
		episode.PublishedAt = time.Unix(pie.DatePublished, 0)
	}
	if pie.DateCrawled > 0 {
		episode.DateCrawled = time.Unix(pie.DateCrawled, 0)
	}

	return episode
}

// CreateErrorResponse creates a Podcast Index compatible error response
func (t *Transformer) CreateErrorResponse(errorMessage string) PodcastIndexErrorResponse {
	return PodcastIndexErrorResponse{
		Status:      "false",
		Description: errorMessage,
	}
}

// CreateSuccessResponse creates a successful response with episodes
func (t *Transformer) CreateSuccessResponse(episodes []models.Episode, description string) PodcastIndexResponse {
	items := make([]PodcastIndexEpisode, 0, len(episodes))
	for _, episode := range episodes {
		items = append(items, t.ModelToPodcastIndex(&episode))
	}

	if description == "" {
		description = fmt.Sprintf("Found %d episodes", len(episodes))
	}

	return PodcastIndexResponse{
		Status:      "true",
		Items:       items,
		Count:       len(items),
		Description: description,
	}
}

// CreateSingleEpisodeResponse creates a response for a single episode
func (t *Transformer) CreateSingleEpisodeResponse(episode *models.Episode) EpisodeByGUIDResponse {
	if episode == nil {
		return EpisodeByGUIDResponse{
			Status:      "false",
			Description: "Episode not found",
		}
	}

	pie := t.ModelToPodcastIndex(episode)
	return EpisodeByGUIDResponse{
		Status:      "true",
		Episode:     &pie,
		Description: "Episode found",
	}
}
