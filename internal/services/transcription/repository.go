package transcription

import (
	"context"
	"errors"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// repository implements the Repository interface using GORM
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new transcription repository
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Create creates a new transcription
func (r *repository) Create(ctx context.Context, transcription *models.Transcription) error {
	if transcription == nil {
		return errors.New("transcription cannot be nil")
	}

	result := r.db.WithContext(ctx).Create(transcription)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetByEpisodeID retrieves a transcription by podcast index episode ID
func (r *repository) GetByEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) (*models.Transcription, error) {
	var transcription models.Transcription

	result := r.db.WithContext(ctx).Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).First(&transcription)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	return &transcription, nil
}

// Update updates an existing transcription
func (r *repository) Update(ctx context.Context, transcription *models.Transcription) error {
	if transcription == nil {
		return errors.New("transcription cannot be nil")
	}

	result := r.db.WithContext(ctx).Save(transcription)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// Delete removes a transcription
func (r *repository) Delete(ctx context.Context, podcastIndexEpisodeID int64) error {
	result := r.db.WithContext(ctx).Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).Delete(&models.Transcription{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// Exists checks if a transcription exists for an episode
func (r *repository) Exists(ctx context.Context, podcastIndexEpisodeID int64) (bool, error) {
	var count int64

	result := r.db.WithContext(ctx).Model(&models.Transcription{}).Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}

	return count > 0, nil
}
