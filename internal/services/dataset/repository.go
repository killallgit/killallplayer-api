package dataset

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

// NewRepository creates a new dataset repository
func NewRepository(db *gorm.DB) Repository {
	return &RepositoryImpl{db: db}
}

// Create creates a new dataset record
func (r *RepositoryImpl) Create(ctx context.Context, dataset *models.Dataset) error {
	return r.db.WithContext(ctx).Create(dataset).Error
}

// GetByID retrieves a dataset by ID
func (r *RepositoryImpl) GetByID(ctx context.Context, id string) (*models.Dataset, error) {
	var dataset models.Dataset
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&dataset).Error
	if err != nil {
		return nil, err
	}
	return &dataset, nil
}

// List retrieves datasets with optional filters
func (r *RepositoryImpl) List(ctx context.Context, filters *ListFilters) ([]models.Dataset, error) {
	var datasets []models.Dataset
	query := r.db.WithContext(ctx).Model(&models.Dataset{})

	// Apply filters
	if filters != nil {
		if filters.Label != "" {
			query = query.Where("label = ?", filters.Label)
		}
		if filters.Format != "" {
			query = query.Where("format = ?", filters.Format)
		}
		if filters.Limit > 0 {
			query = query.Limit(filters.Limit)
		}
		if filters.Offset > 0 {
			query = query.Offset(filters.Offset)
		}
	}

	// Order by creation time (newest first)
	query = query.Order("created_at DESC")

	err := query.Find(&datasets).Error
	return datasets, err
}

// Update updates an existing dataset
func (r *RepositoryImpl) Update(ctx context.Context, dataset *models.Dataset) error {
	return r.db.WithContext(ctx).Save(dataset).Error
}

// Delete deletes a dataset record
func (r *RepositoryImpl) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.Dataset{}, "id = ?", id).Error
}

// GetStats retrieves dataset statistics
func (r *RepositoryImpl) GetStats(ctx context.Context) (*DatasetStats, error) {
	stats := &DatasetStats{
		ByLabel:       make(map[string]int),
		ByFormat:      make(map[string]int),
		ByAudioFormat: make(map[string]int),
	}

	// Get total datasets
	var totalDatasets int64
	r.db.WithContext(ctx).Model(&models.Dataset{}).Count(&totalDatasets)
	stats.TotalDatasets = int(totalDatasets)

	// Get aggregated statistics
	var aggregates struct {
		TotalSamples  int64
		TotalDuration float64
		TotalSize     int64
	}

	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Select("COALESCE(SUM(total_samples), 0) as total_samples, COALESCE(SUM(total_duration), 0) as total_duration, COALESCE(SUM(total_size), 0) as total_size").
		Scan(&aggregates)

	stats.TotalSamples = int(aggregates.TotalSamples)
	stats.TotalDuration = aggregates.TotalDuration
	stats.TotalSize = aggregates.TotalSize

	// Get counts by label
	var labelCounts []struct {
		Label string
		Count int
	}
	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Select("label, SUM(total_samples) as count").
		Group("label").
		Scan(&labelCounts)

	for _, lc := range labelCounts {
		stats.ByLabel[lc.Label] = lc.Count
	}

	// Get counts by format
	var formatCounts []struct {
		Format string
		Count  int
	}
	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Select("format, COUNT(*) as count").
		Group("format").
		Scan(&formatCounts)

	for _, fc := range formatCounts {
		stats.ByFormat[fc.Format] = fc.Count
	}

	// Get counts by audio format
	var audioFormatCounts []struct {
		AudioFormat string
		Count       int
	}
	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Select("audio_format, COUNT(*) as count").
		Group("audio_format").
		Scan(&audioFormatCounts)

	for _, afc := range audioFormatCounts {
		stats.ByAudioFormat[afc.AudioFormat] = afc.Count
	}

	// Get time-based counts
	today := time.Now().Truncate(24 * time.Hour)
	thisWeek := today.AddDate(0, 0, -7)

	var createdToday, createdThisWeek int64
	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Where("created_at >= ?", today).
		Count(&createdToday)
	stats.CreatedToday = int(createdToday)

	r.db.WithContext(ctx).Model(&models.Dataset{}).
		Where("created_at >= ?", thisWeek).
		Count(&createdThisWeek)
	stats.CreatedThisWeek = int(createdThisWeek)

	return stats, nil
}
