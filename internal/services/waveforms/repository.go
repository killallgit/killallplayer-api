package waveforms

import (
	"context"
	"errors"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// repository implements WaveformRepository
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new waveform repository
func NewRepository(db *gorm.DB) WaveformRepository {
	return &repository{db: db}
}

// GetByEpisodeID retrieves waveform by episode ID
func (r *repository) GetByEpisodeID(ctx context.Context, episodeID uint) (*models.Waveform, error) {
	var waveform models.Waveform
	err := r.db.WithContext(ctx).
		Where("episode_id = ?", episodeID).
		First(&waveform).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWaveformNotFound
		}
		return nil, err
	}

	return &waveform, nil
}

// Create saves a new waveform
func (r *repository) Create(ctx context.Context, waveform *models.Waveform) error {
	return r.db.WithContext(ctx).Create(waveform).Error
}

// Update modifies an existing waveform
func (r *repository) Update(ctx context.Context, waveform *models.Waveform) error {
	return r.db.WithContext(ctx).Save(waveform).Error
}

// Delete removes a waveform by episode ID
func (r *repository) Delete(ctx context.Context, episodeID uint) error {
	result := r.db.WithContext(ctx).
		Where("episode_id = ?", episodeID).
		Delete(&models.Waveform{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrWaveformNotFound
	}

	return nil
}

// Exists checks if a waveform exists for an episode
func (r *repository) Exists(ctx context.Context, episodeID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Waveform{}).
		Where("episode_id = ?", episodeID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
