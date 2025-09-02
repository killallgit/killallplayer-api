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
}