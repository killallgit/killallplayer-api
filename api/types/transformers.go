package types

import (
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
