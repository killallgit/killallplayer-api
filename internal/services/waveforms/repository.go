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

// GetByPodcastIndexEpisodeID retrieves waveform by Podcast Index episode ID
func (r *repository) GetByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) (*models.Waveform, error) {
	var waveform models.Waveform
	err := r.db.WithContext(ctx).
		Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).
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

// Delete removes a waveform by Podcast Index episode ID
func (r *repository) Delete(ctx context.Context, podcastIndexEpisodeID int64) error {
	result := r.db.WithContext(ctx).
		Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).
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
func (r *repository) Exists(ctx context.Context, podcastIndexEpisodeID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Waveform{}).
		Where("podcast_index_episode_id = ?", podcastIndexEpisodeID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
