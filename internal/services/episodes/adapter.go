package episodes

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastIndexAdapter adapts the podcastindex.Client to the EpisodeFetcher interface
type PodcastIndexAdapter struct {
	client *podcastindex.Client
}

// NewPodcastIndexAdapter creates a new adapter for the Podcast Index client
func NewPodcastIndexAdapter(client *podcastindex.Client) EpisodeFetcher {
	return &PodcastIndexAdapter{
		client: client,
	}
}

// GetEpisodesByPodcastID fetches episodes for a specific podcast
func (a *PodcastIndexAdapter) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error) {
	// Call the podcastindex client
	resp, err := a.client.GetEpisodesByPodcastID(ctx, podcastID, limit)
	if err != nil {
		return nil, err
	}

	// Convert from podcastindex.EpisodesResponse to PodcastIndexResponse
	return a.convertEpisodesResponse(resp), nil
}

// GetEpisodeByGUID fetches a single episode by GUID
func (a *PodcastIndexAdapter) GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error) {
	// Call the podcastindex client
	resp, err := a.client.GetEpisodeByGUID(ctx, guid)
	if err != nil {
		return nil, err
	}

	// Convert from podcastindex.EpisodeByGUIDResponse to EpisodeByGUIDResponse
	return a.convertEpisodeByGUIDResponse(resp), nil
}

// GetEpisodeMetadata fetches metadata for an episode URL
func (a *PodcastIndexAdapter) GetEpisodeMetadata(ctx context.Context, episodeURL string) (*EpisodeMetadata, error) {
	// Parse URL to validate it
	parsedURL, err := url.Parse(episodeURL)
	if err != nil {
		return nil, NewValidationError("episodeURL", "invalid URL format")
	}

	// Create a HEAD request to get metadata
	req, err := http.NewRequestWithContext(ctx, "HEAD", episodeURL, nil)
	if err != nil {
		return nil, err
	}

	// Use a simple HTTP client for external URLs (not Podcast Index API)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Extract filename from URL path
	fileName := "episode"
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		parts := strings.Split(parsedURL.Path, "/")
		if len(parts) > 0 {
			lastName := parts[len(parts)-1]
			if lastName != "" {
				fileName = lastName
			}
		}
	}

	// Extract metadata from headers
	metadata := &EpisodeMetadata{
		URL:          episodeURL,
		ContentType:  resp.Header.Get("Content-Type"),
		Size:         resp.ContentLength,
		LastModified: time.Now(), // Parse Last-Modified header if available
		FileName:     fileName,
	}

	// Try to parse Last-Modified header
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := time.Parse(time.RFC1123, lastMod); err == nil {
			metadata.LastModified = t
		}
	}

	return metadata, nil
}

// convertEpisodesResponse converts from podcastindex types to internal types
func (a *PodcastIndexAdapter) convertEpisodesResponse(resp *podcastindex.EpisodesResponse) *PodcastIndexResponse {
	items := make([]PodcastIndexEpisode, len(resp.Items))
	for i, ep := range resp.Items {
		items[i] = a.convertEpisode(ep)
	}

	return &PodcastIndexResponse{
		Status:      resp.Status,
		Items:       items,
		Count:       resp.Count,
		Description: resp.Description,
	}
}

// convertEpisodeByGUIDResponse converts from podcastindex types to internal types
func (a *PodcastIndexAdapter) convertEpisodeByGUIDResponse(resp *podcastindex.EpisodeByGUIDResponse) *EpisodeByGUIDResponse {
	episode := a.convertEpisode(resp.Episode)
	return &EpisodeByGUIDResponse{
		Status:      resp.Status,
		Episode:     &episode,
		Description: resp.Description,
	}
}

// convertEpisode converts a single episode from podcastindex type to internal type
func (a *PodcastIndexAdapter) convertEpisode(ep podcastindex.Episode) PodcastIndexEpisode {
	// Helper functions to convert int to pointer
	intPtr := func(i int) *int {
		return &i
	}

	int64Ptr := func(i int64) *int64 {
		return &i
	}

	var duration *int
	if ep.Duration > 0 {
		duration = intPtr(ep.Duration)
	}

	var episode *int
	if ep.Episode > 0 {
		episode = intPtr(ep.Episode)
	}

	var season *int
	if ep.Season > 0 {
		season = intPtr(ep.Season)
	}

	var feedItunesID *int64
	if ep.FeedItunesId > 0 {
		feedItunesID = int64Ptr(int64(ep.FeedItunesId))
	}

	var feedDuplicateOf *int64
	if ep.FeedDuplicateOf > 0 {
		feedDuplicateOf = int64Ptr(int64(ep.FeedDuplicateOf))
	}

	return PodcastIndexEpisode{
		ID:                  ep.ID,
		Title:               ep.Title,
		Link:                ep.Link,
		Description:         ep.Description,
		GUID:                ep.GUID,
		DatePublished:       ep.DatePublished,
		DatePublishedPretty: ep.DatePublishedPretty,
		DateCrawled:         ep.DateCrawled,
		EnclosureURL:        ep.EnclosureURL,
		EnclosureType:       ep.EnclosureType,
		EnclosureLength:     int64(ep.EnclosureLength),
		Duration:            duration,
		Explicit:            ep.Explicit,
		Episode:             episode,
		EpisodeType:         ep.EpisodeType,
		Season:              season,
		Image:               ep.Image,
		FeedItunesID:        feedItunesID,
		FeedImage:           ep.FeedImage,
		FeedID:              int64(ep.FeedId),
		FeedLanguage:        ep.FeedLanguage,
		FeedDead:            ep.FeedDead,
		FeedDuplicateOf:     feedDuplicateOf,
		ChaptersURL:         ep.ChaptersURL,
		TranscriptURL:       ep.TranscriptURL,
	}
}
