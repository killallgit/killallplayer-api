package audiocache

import (
	"context"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// RepositoryImpl implements the Repository interface using GORM
type RepositoryImpl struct {
	db *gorm.DB
}

// NewRepository creates a new audio cache repository
func NewRepository(db *gorm.DB) Repository {
	return &RepositoryImpl{db: db}
}

// Create creates a new audio cache entry
func (r *RepositoryImpl) Create(ctx context.Context, cache *models.AudioCache) error {
	return r.db.WithContext(ctx).Create(cache).Error
}

// GetByEpisodeID retrieves cache entry by episode ID
func (r *RepositoryImpl) GetByEpisodeID(ctx context.Context, episodeID uint) (*models.AudioCache, error) {
	var cache models.AudioCache
	err := r.db.WithContext(ctx).Where("episode_id = ?", episodeID).First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// GetBySHA256 retrieves cache entry by SHA256 hash
func (r *RepositoryImpl) GetBySHA256(ctx context.Context, sha256 string) (*models.AudioCache, error) {
	var cache models.AudioCache
	err := r.db.WithContext(ctx).Where("original_sha256 = ? OR processed_sha256 = ?", sha256, sha256).First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// Update updates an existing cache entry
func (r *RepositoryImpl) Update(ctx context.Context, cache *models.AudioCache) error {
	return r.db.WithContext(ctx).Save(cache).Error
}

// Delete deletes a cache entry
func (r *RepositoryImpl) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.AudioCache{}, id).Error
}

// GetOlderThan retrieves cache entries older than specified days
func (r *RepositoryImpl) GetOlderThan(ctx context.Context, olderThanDays int) ([]models.AudioCache, error) {
	var caches []models.AudioCache
	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)
	err := r.db.WithContext(ctx).Where("last_used_at < ?", cutoffTime).Find(&caches).Error
	return caches, err
}

// GetStats retrieves cache statistics
func (r *RepositoryImpl) GetStats(ctx context.Context) (*CacheStats, error) {
	stats := &CacheStats{}

	// Get total entries
	r.db.WithContext(ctx).Model(&models.AudioCache{}).Count(&stats.TotalEntries)

	// Get size statistics
	r.db.WithContext(ctx).Model(&models.AudioCache{}).
		Select("COALESCE(SUM(original_size), 0) as original_size, COALESCE(SUM(processed_size), 0) as processed_size").
		Scan(&struct {
			OriginalSize  int64
			ProcessedSize int64
		}{
			OriginalSize:  stats.OriginalSize,
			ProcessedSize: stats.ProcessedSize,
		})

	stats.TotalSizeBytes = stats.OriginalSize + stats.ProcessedSize

	// Get average duration
	r.db.WithContext(ctx).Model(&models.AudioCache{}).
		Select("COALESCE(AVG(duration_seconds), 0)").
		Scan(&stats.AverageDuration)

	// Get oldest and newest entries
	var oldest, newest models.AudioCache
	r.db.WithContext(ctx).Model(&models.AudioCache{}).Order("created_at ASC").First(&oldest)
	r.db.WithContext(ctx).Model(&models.AudioCache{}).Order("created_at DESC").First(&newest)

	if oldest.ID > 0 {
		stats.OldestEntry = oldest.CreatedAt.Format(time.RFC3339)
	}
	if newest.ID > 0 {
		stats.NewestEntry = newest.CreatedAt.Format(time.RFC3339)
	}

	return stats, nil
}
