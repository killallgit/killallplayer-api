package podcasts

import (
	"context"

	"github.com/killallgit/player-api/internal/models"
)

// PodcastRepository defines the data access interface for podcasts
type PodcastRepository interface {
	// Create/Update
	CreatePodcast(ctx context.Context, podcast *models.Podcast) error
	UpdatePodcast(ctx context.Context, podcast *models.Podcast) error
	UpsertPodcast(ctx context.Context, podcast *models.Podcast) error

	// Read
	GetPodcastByID(ctx context.Context, id uint) (*models.Podcast, error)
	GetPodcastByPodcastIndexID(ctx context.Context, piID int64) (*models.Podcast, error)
	GetPodcastByFeedURL(ctx context.Context, feedURL string) (*models.Podcast, error)
	GetPodcastByITunesID(ctx context.Context, itunesID int64) (*models.Podcast, error)

	// List
	ListPodcasts(ctx context.Context, page, limit int) ([]models.Podcast, int64, error)
	SearchPodcasts(ctx context.Context, query string, limit int) ([]models.Podcast, error)

	// Metadata
	UpdateLastFetched(ctx context.Context, podcastID uint) error
	IncrementFetchCount(ctx context.Context, podcastID uint) error
}

// PodcastService defines the business logic interface for podcast operations
type PodcastService interface {
	// DB-first lookup (main method)
	GetPodcastByPodcastIndexID(ctx context.Context, piID int64) (*models.Podcast, error)

	// Fetch from API and store
	FetchAndStorePodcast(ctx context.Context, piID int64) (*models.Podcast, error)

	// Background refresh
	RefreshPodcast(ctx context.Context, piID int64) (*models.Podcast, error)
	ShouldRefresh(podcast *models.Podcast) bool

	// Direct repository access
	GetByID(ctx context.Context, id uint) (*models.Podcast, error)
	UpdatePodcastMetrics(ctx context.Context, podcastID uint, episodeCount int) error
}
