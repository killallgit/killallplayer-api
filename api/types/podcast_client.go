package types

import (
	"context"

	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastClient defines the interface for podcast index operations
type PodcastClient interface {
	Search(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error)
	GetTrending(limit int) (*podcastindex.SearchResponse, error)
	GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*podcastindex.EpisodesResponse, error)

	// Podcast metadata endpoints
	GetPodcastByFeedURL(ctx context.Context, feedURL string) (*podcastindex.PodcastResponse, error)
	GetPodcastByFeedID(ctx context.Context, feedID int64) (*podcastindex.PodcastResponse, error)
	GetPodcastByiTunesID(ctx context.Context, itunesID int64) (*podcastindex.PodcastResponse, error)

	// Alternative episode endpoints
	GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) (*podcastindex.EpisodesResponse, error)
	GetEpisodesByiTunesID(ctx context.Context, itunesID int64, limit int) (*podcastindex.EpisodesResponse, error)

	// Recent/discovery endpoints
	GetRecentEpisodes(ctx context.Context, limit int) (*podcastindex.EpisodesResponse, error)
	GetRecentFeeds(ctx context.Context, limit int) (*podcastindex.RecentFeedsResponse, error)
}
