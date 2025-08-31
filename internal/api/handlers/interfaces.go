package handlers

import (
	"context"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastSearcher defines the interface for searching podcasts
type PodcastSearcher interface {
	Search(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error)
}
