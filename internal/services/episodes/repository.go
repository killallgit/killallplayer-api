package episodes

import (
	"context"
	"errors"
	"fmt"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

// Ensure Repository implements EpisodeRepository interface
var _ EpisodeRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateEpisode(ctx context.Context, episode *models.Episode) error {
	if err := r.db.WithContext(ctx).Create(episode).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("episode with GUID %s already exists", episode.GUID)
		}
		return fmt.Errorf("creating episode: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEpisode(ctx context.Context, episode *models.Episode) error {
	result := r.db.WithContext(ctx).Save(episode)
	if result.Error != nil {
		return fmt.Errorf("updating episode: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewNotFoundError("episode", episode.ID)
	}
	return nil
}

func (r *Repository) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	var episode models.Episode
	if err := r.db.WithContext(ctx).First(&episode, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFoundError("episode", id)
		}
		return nil, fmt.Errorf("getting episode: %w", err)
	}
	return &episode, nil
}

func (r *Repository) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	var episode models.Episode
	if err := r.db.WithContext(ctx).Where("guid = ?", guid).First(&episode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFoundError("episode", guid)
		}
		return nil, fmt.Errorf("getting episode: %w", err)
	}
	return &episode, nil
}

func (r *Repository) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error) {
	var episodes []models.Episode
	var total int64

	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).Model(&models.Episode{}).Where("podcast_id = ?", podcastID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting episodes: %w", err)
	}

	if err := query.
		Order("published_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&episodes).Error; err != nil {
		return nil, 0, fmt.Errorf("getting episodes: %w", err)
	}

	return episodes, total, nil
}

func (r *Repository) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	var episodes []models.Episode

	if err := r.db.WithContext(ctx).
		Order("published_at DESC").
		Limit(limit).
		Find(&episodes).Error; err != nil {
		return nil, fmt.Errorf("getting recent episodes: %w", err)
	}

	return episodes, nil
}

func (r *Repository) DeleteEpisode(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.Episode{}, id)
	if result.Error != nil {
		return fmt.Errorf("deleting episode: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewNotFoundError("episode", id)
	}
	return nil
}

func (r *Repository) UpsertEpisode(ctx context.Context, episode *models.Episode) error {
	var existing models.Episode
	err := r.db.WithContext(ctx).Where("guid = ?", episode.GUID).First(&existing).Error

	if err == nil {
		episode.ID = existing.ID
		episode.CreatedAt = existing.CreatedAt
		return r.UpdateEpisode(ctx, episode)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.CreateEpisode(ctx, episode)
	}

	return fmt.Errorf("checking existing episode: %w", err)
}

func (r *Repository) MarkEpisodeAsPlayed(ctx context.Context, id uint, played bool) error {
	result := r.db.WithContext(ctx).
		Model(&models.Episode{}).
		Where("id = ?", id).
		Update("played", played)

	if result.Error != nil {
		return fmt.Errorf("updating played status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewNotFoundError("episode", id)
	}
	return nil
}

func (r *Repository) UpdatePlaybackPosition(ctx context.Context, id uint, position int) error {
	result := r.db.WithContext(ctx).
		Model(&models.Episode{}).
		Where("id = ?", id).
		Update("position", position)

	if result.Error != nil {
		return fmt.Errorf("updating playback position: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewNotFoundError("episode", id)
	}
	return nil
}
