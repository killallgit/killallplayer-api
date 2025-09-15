package types

import (
	"context"

	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastClient defines the interface for podcast index operations
type PodcastClient interface {
	Search(ctx context.Context, query string, limit int, fullText bool, val string, apOnly bool, clean bool) (*podcastindex.SearchResponse, error)
	GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*podcastindex.SearchResponse, error)
	GetCategories() (*podcastindex.CategoriesResponse, error)
	GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*podcastindex.EpisodesResponse, error)

	// Alternative episode endpoints
	GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) (*podcastindex.EpisodesResponse, error)
	GetEpisodesByiTunesID(ctx context.Context, itunesID int64, limit int) (*podcastindex.EpisodesResponse, error)

	// Recent/discovery endpoints
	GetRecentEpisodes(ctx context.Context, limit int) (*podcastindex.EpisodesResponse, error)

	GetRandomEpisodes(ctx context.Context, max int, lang string, notCategories []string) (*podcastindex.EpisodesResponse, error)
	GetRecentFeeds(ctx context.Context, limit int) (*podcastindex.RecentFeedsResponse, error)
}
