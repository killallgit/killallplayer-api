package episodes

import (
	"context"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

// EpisodeRepository defines the interface for episode data persistence
type EpisodeRepository interface {
	// Create operations
	CreateEpisode(ctx context.Context, episode *models.Episode) error
	UpsertEpisode(ctx context.Context, episode *models.Episode) error

	// Read operations
	GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error)
	GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error)
	GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error)
	GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error)
	GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error)

	// Update operations
	UpdateEpisode(ctx context.Context, episode *models.Episode) error
	MarkEpisodeAsPlayed(ctx context.Context, id uint, played bool) error
	UpdatePlaybackPosition(ctx context.Context, id uint, position int) error

	// Delete operations
	DeleteEpisode(ctx context.Context, id uint) error
}

// EpisodeFetcher defines the interface for fetching episodes from external sources
type EpisodeFetcher interface {
	GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error)
	GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error)
	GetEpisodeMetadata(ctx context.Context, episodeURL string) (*EpisodeMetadata, error)
}

// EpisodeCache defines the interface for caching episode data
type EpisodeCache interface {
	// Single episode operations
	GetEpisode(key string) (*models.Episode, bool)
	SetEpisode(key string, episode *models.Episode)

	// Episode list operations
	GetEpisodeList(key string) ([]models.Episode, int64, bool)
	SetEpisodeList(key string, episodes []models.Episode, total int64)

	// Cache management
	Invalidate(key string)
	InvalidatePattern(pattern string)
	Clear()
	Stop() // For graceful shutdown
}

// EpisodeService defines the business logic interface for episode operations
type EpisodeService interface {
	// Fetch and sync operations
	FetchAndSyncEpisodes(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error)
	SyncEpisodesToDatabase(ctx context.Context, episodes []PodcastIndexEpisode, podcastID uint) (int, error)

	// Get operations with caching and fallback
	GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error)
	GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error)
	GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error)
	GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error)
	GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error)

	// Playback operations
	UpdatePlaybackState(ctx context.Context, id uint, position int, played bool) error
	UpdatePlaybackStateByPodcastIndexID(ctx context.Context, podcastIndexID int64, position int, played bool) error
}

// EpisodeTransformer defines the interface for transforming between different episode formats
type EpisodeTransformer interface {
	ModelToPodcastIndex(episode *models.Episode) PodcastIndexEpisode
	PodcastIndexToModel(pie PodcastIndexEpisode, podcastID uint) *models.Episode
	CreateSuccessResponse(episodes []models.Episode, description string) PodcastIndexResponse
	CreateSingleEpisodeResponse(episode *models.Episode) EpisodeByGUIDResponse
	CreateErrorResponse(message string) PodcastIndexErrorResponse
}

// HTTPHeaders represents HTTP response headers
// This is kept for compatibility with existing code
type HTTPHeaders struct {
	ContentType   string
	ContentLength int64
	LastModified  time.Time
}

// CacheKeyGenerator defines the interface for generating cache keys
type CacheKeyGenerator interface {
	EpisodeByID(id uint) string
	EpisodeByGUID(guid string) string
	EpisodeByPodcastIndexID(podcastIndexID int64) string
	EpisodesByPodcast(podcastID uint, page, limit int) string
	RecentEpisodes(limit int) string
	PodcastPattern(podcastID uint) string
}
