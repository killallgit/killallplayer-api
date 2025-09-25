package annotations

import (
	"context"
	"errors"
	"fmt"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// RepositoryImpl implements the Repository interface
type RepositoryImpl struct {
	db *gorm.DB
}

// NewRepository creates a new annotation repository
func NewRepository(db *gorm.DB) Repository {
	return &RepositoryImpl{db: db}
}

// CreateAnnotation creates a new annotation in the database
func (r *RepositoryImpl) CreateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	if err := r.db.WithContext(ctx).Create(annotation).Error; err != nil {
		return fmt.Errorf("creating annotation: %w", err)
	}
	return nil
}

// GetAnnotationByID retrieves an annotation by its ID
func (r *RepositoryImpl) GetAnnotationByID(ctx context.Context, id uint) (*models.Annotation, error) {
	var annotation models.Annotation
	if err := r.db.WithContext(ctx).First(&annotation, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("annotation not found")
		}
		return nil, fmt.Errorf("getting annotation: %w", err)
	}
	return &annotation, nil
}

// GetAnnotationsByPodcastIndexEpisodeID retrieves all annotations for a specific episode
func (r *RepositoryImpl) GetAnnotationsByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) ([]models.Annotation, error) {
	var annotations []models.Annotation
	if err := r.db.WithContext(ctx).
		Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).
		Order("start_time ASC").
		Find(&annotations).Error; err != nil {
		return nil, fmt.Errorf("getting annotations for episode: %w", err)
	}
	return annotations, nil
}

// UpdateAnnotation updates an existing annotation
func (r *RepositoryImpl) UpdateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	result := r.db.WithContext(ctx).Save(annotation)
	if result.Error != nil {
		return fmt.Errorf("updating annotation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("annotation not found")
	}
	return nil
}

// DeleteAnnotation deletes an annotation by its ID
func (r *RepositoryImpl) DeleteAnnotation(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.Annotation{}, id)
	if result.Error != nil {
		return fmt.Errorf("deleting annotation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("annotation not found")
	}
	return nil
}

// GetAnnotationByUUID retrieves an annotation by its UUID
func (r *RepositoryImpl) GetAnnotationByUUID(ctx context.Context, uuid string) (*models.Annotation, error) {
	var annotation models.Annotation
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&annotation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("annotation not found")
		}
		return nil, fmt.Errorf("getting annotation: %w", err)
	}
	return &annotation, nil
}

// CheckOverlappingAnnotation checks if there's an existing annotation that overlaps with the given time range
func (r *RepositoryImpl) CheckOverlappingAnnotation(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Annotation{}).
		Where("podcast_index_episode_id = ? AND start_time < ? AND end_time > ?",
			podcastIndexEpisodeID, endTime, startTime).
		Count(&count).Error

	return count > 0, err
}

// CheckOverlappingAnnotationExcluding checks for overlaps excluding a specific annotation ID
func (r *RepositoryImpl) CheckOverlappingAnnotationExcluding(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64, excludeID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Annotation{}).
		Where("podcast_index_episode_id = ? AND id != ? AND start_time < ? AND end_time > ?",
			podcastIndexEpisodeID, excludeID, endTime, startTime).
		Count(&count).Error

	return count > 0, err
}
