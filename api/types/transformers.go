package types

import (
	"encoding/json"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/itunes"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// FromPodcastIndex transforms a Podcast Index podcast to our simplified Podcast type
func FromPodcastIndex(p *podcastindex.Podcast) *Podcast {
	if p == nil {
		return nil
	}

	// Convert categories map to array
	categories := make([]string, 0, len(p.Categories))
	for _, category := range p.Categories {
		if category != "" {
			categories = append(categories, category)
		}
	}

	return &Podcast{
		ID:           int64(p.ID),
		Title:        p.Title,
		Author:       p.Author,
		Description:  p.Description,
		Link:         p.Link,
		Image:        p.Image,
		FeedURL:      p.URL,
		ITunesID:     int64(p.ITunesID),
		Language:     p.Language,
		Categories:   categories,
		EpisodeCount: p.EpisodeCount,
		LastUpdated:  p.LastUpdateTime,
	}
}

// FromPodcastIndexList transforms a list of Podcast Index podcasts
func FromPodcastIndexList(podcasts []podcastindex.Podcast) []Podcast {
	result := make([]Podcast, 0, len(podcasts))
	for _, p := range podcasts {
		if transformed := FromPodcastIndex(&p); transformed != nil {
			result = append(result, *transformed)
		}
	}
	return result
}

// FromITunes transforms an iTunes podcast to our simplified Podcast type
func FromITunes(p *itunes.Podcast) *Podcast {
	if p == nil {
		return nil
	}

	// iTunes doesn't provide categories as an array, use genre
	categories := []string{}
	if p.Genre != "" {
		categories = append(categories, p.Genre)
	}

	return &Podcast{
		ID:           p.ID, // Note: This is iTunes ID, might need mapping to Podcast Index ID
		Title:        p.Title,
		Author:       p.Author,
		Description:  p.Description,
		Link:         p.ITunesURL, // iTunes store URL for the podcast
		Image:        p.ArtworkURL,
		FeedURL:      p.FeedURL,
		ITunesID:     p.ID,
		Language:     p.Language,
		Categories:   categories,
		EpisodeCount: p.EpisodeCount,
		LastUpdated:  p.ReleaseDate.Unix(),
	}
}

// FromPodcastIndexEpisode transforms a Podcast Index episode to our simplified Episode type
func FromPodcastIndexEpisode(e *podcastindex.Episode) *Episode {
	if e == nil {
		return nil
	}

	// Use episode image if available, otherwise fall back to feed image
	image := e.Image
	if image == "" {
		image = e.FeedImage
	}

	return &Episode{
		ID:            e.ID,
		PodcastID:     int64(e.FeedId),
		Title:         e.Title,
		Description:   e.Description,
		Link:          e.Link,
		AudioURL:      e.EnclosureURL,
		Duration:      e.Duration,
		PublishedAt:   e.DatePublished,
		Image:         image,
		TranscriptURL: e.TranscriptURL,
		ChaptersURL:   e.ChaptersURL,
	}
}

// FromPodcastIndexEpisodeList transforms a list of Podcast Index episodes
func FromPodcastIndexEpisodeList(episodes []podcastindex.Episode) []Episode {
	result := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		if transformed := FromPodcastIndexEpisode(&e); transformed != nil {
			result = append(result, *transformed)
		}
	}
	return result
}

// FromServiceEpisode transforms an internal service episode to our simplified Episode type
func FromServiceEpisode(e *episodes.PodcastIndexEpisode) *Episode {
	if e == nil {
		return nil
	}

	duration := 0
	if e.Duration != nil {
		duration = *e.Duration
	}

	episode := 0
	if e.Episode != nil {
		episode = *e.Episode
	}

	season := 0
	if e.Season != nil {
		season = *e.Season
	}

	return &Episode{
		ID:            e.ID,
		PodcastID:     e.FeedID,
		Title:         e.Title,
		Description:   e.Description,
		Link:          e.Link,
		AudioURL:      e.EnclosureURL,
		Duration:      duration,
		PublishedAt:   e.DatePublished,
		Image:         e.Image,
		TranscriptURL: e.TranscriptURL,
		ChaptersURL:   e.ChaptersURL,
		Episode:       episode,
		Season:        season,
	}
}

// FromServiceEpisodeList transforms a list of internal service episodes
func FromServiceEpisodeList(episodes []episodes.PodcastIndexEpisode) []Episode {
	result := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		if transformed := FromServiceEpisode(&e); transformed != nil {
			result = append(result, *transformed)
		}
	}
	return result
}

// FromModelEpisode transforms a database model episode to our simplified Episode type
func FromModelEpisode(e *models.Episode) *Episode {
	if e == nil {
		return nil
	}

	duration := 0
	if e.Duration != nil {
		duration = *e.Duration
	}

	episode := 0
	if e.EpisodeNumber != nil {
		episode = *e.EpisodeNumber
	}

	season := 0
	if e.Season != nil {
		season = *e.Season
	}

	// Use episode image if available, otherwise fall back to feed image
	image := e.Image
	if image == "" {
		image = e.FeedImage
	}

	return &Episode{
		ID:            e.PodcastIndexID,
		PodcastID:     int64(e.PodcastID),
		Title:         e.Title,
		Description:   e.Description,
		Link:          e.Link,
		AudioURL:      e.AudioURL,
		Duration:      duration,
		PublishedAt:   e.PublishedAt.Unix(),
		Image:         image,
		TranscriptURL: "", // Not stored in models.Episode yet
		ChaptersURL:   "", // Not stored in models.Episode yet
		Episode:       episode,
		Season:        season,
	}
}

// FromModelEpisodeList transforms a list of database model episodes
func FromModelEpisodeList(episodes []models.Episode) []Episode {
	result := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		if transformed := FromModelEpisode(&e); transformed != nil {
			result = append(result, *transformed)
		}
	}
	return result
}

// FromModelPodcast transforms a database model podcast to our simplified Podcast type
func FromModelPodcast(p *models.Podcast) *Podcast {
	if p == nil {
		return nil
	}

	// Extract categories from JSON
	categories := []string{}
	if len(p.Categories) > 0 {
		// Categories is stored as JSON map[int]string
		var categoryMap map[string]string
		if err := json.Unmarshal(p.Categories, &categoryMap); err == nil {
			for _, category := range categoryMap {
				if category != "" {
					categories = append(categories, category)
				}
			}
		}
	}

	itunesID := int64(0)
	if p.ITunesID != nil {
		itunesID = *p.ITunesID
	}

	lastUpdated := int64(0)
	if p.LastUpdateTime != nil {
		lastUpdated = p.LastUpdateTime.Unix()
	}

	return &Podcast{
		ID:           p.PodcastIndexID,
		Title:        p.Title,
		Author:       p.Author,
		Description:  p.Description,
		Link:         p.Link,
		Image:        p.Image,
		FeedURL:      p.FeedURL,
		ITunesID:     itunesID,
		Language:     p.Language,
		Categories:   categories,
		EpisodeCount: p.EpisodeCount,
		LastUpdated:  lastUpdated,
	}
}

// FromModelPodcastList transforms a list of database model podcasts
func FromModelPodcastList(podcasts []models.Podcast) []Podcast {
	result := make([]Podcast, 0, len(podcasts))
	for _, p := range podcasts {
		if transformed := FromModelPodcast(&p); transformed != nil {
			result = append(result, *transformed)
		}
	}
	return result
}
