package audiocache

import (
	"context"
	"io"

	"github.com/killallgit/player-api/internal/models"
)

// Service defines the interface for audio caching operations
type Service interface {
	// GetOrDownloadAudio retrieves cached audio or downloads if not present
	// Returns paths to both original and processed audio files
	GetOrDownloadAudio(ctx context.Context, episodeID uint, audioURL string) (*models.AudioCache, error)

	// GetCachedAudio retrieves cached audio without downloading
	GetCachedAudio(ctx context.Context, episodeID uint) (*models.AudioCache, error)

	// ProcessAudioForML converts audio to 16kHz mono for ML training
	ProcessAudioForML(ctx context.Context, originalPath string, outputPath string) error

	// UpdateLastUsed updates the last used timestamp for cache entry
	UpdateLastUsed(ctx context.Context, cacheID uint) error

	// CleanupOldCache removes cache entries older than specified days
	CleanupOldCache(ctx context.Context, olderThanDays int) error

	// GetCacheStats returns statistics about the cache
	GetCacheStats(ctx context.Context) (*CacheStats, error)
}

// Repository defines the interface for audio cache data persistence
type Repository interface {
	// Create creates a new audio cache entry
	Create(ctx context.Context, cache *models.AudioCache) error

	// GetByEpisodeID retrieves cache entry by episode ID
	GetByEpisodeID(ctx context.Context, episodeID uint) (*models.AudioCache, error)

	// GetBySHA256 retrieves cache entry by SHA256 hash
	GetBySHA256(ctx context.Context, sha256 string) (*models.AudioCache, error)

	// Update updates an existing cache entry
	Update(ctx context.Context, cache *models.AudioCache) error

	// Delete deletes a cache entry
	Delete(ctx context.Context, id uint) error

	// GetOlderThan retrieves cache entries older than specified time
	GetOlderThan(ctx context.Context, olderThan int) ([]models.AudioCache, error)

	// GetStats retrieves cache statistics
	GetStats(ctx context.Context) (*CacheStats, error)
}

// StorageBackend defines the interface for file storage operations
type StorageBackend interface {
	// Save saves data to storage and returns the path
	Save(ctx context.Context, data io.Reader, filename string) (string, error)

	// Load loads data from storage
	Load(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes data from storage
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL for accessing the file (for cloud storage)
	GetURL(ctx context.Context, path string) (string, error)
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries    int64   `json:"total_entries"`
	TotalSizeBytes  int64   `json:"total_size_bytes"`
	OriginalSize    int64   `json:"original_size"`
	ProcessedSize   int64   `json:"processed_size"`
	OldestEntry     string  `json:"oldest_entry"`
	NewestEntry     string  `json:"newest_entry"`
	AverageDuration float64 `json:"average_duration_seconds"`
}
