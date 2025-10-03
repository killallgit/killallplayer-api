package podcasts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) PodcastRepository {
	return &Repository{db: db}
}

// CreatePodcast creates a new podcast
func (r *Repository) CreatePodcast(ctx context.Context, podcast *models.Podcast) error {
	if err := r.db.WithContext(ctx).Create(podcast).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("podcast with PodcastIndexID %d or FeedURL %s already exists", podcast.PodcastIndexID, podcast.FeedURL)
		}
		return fmt.Errorf("creating podcast: %w", err)
	}
	return nil
}

// UpdatePodcast updates an existing podcast
func (r *Repository) UpdatePodcast(ctx context.Context, podcast *models.Podcast) error {
	result := r.db.WithContext(ctx).Save(podcast)
	if result.Error != nil {
		return fmt.Errorf("updating podcast: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("podcast not found")
	}
	return nil
}

// UpsertPodcast creates or updates a podcast based on PodcastIndexID
func (r *Repository) UpsertPodcast(ctx context.Context, podcast *models.Podcast) error {
	var existing models.Podcast
	err := r.db.WithContext(ctx).
		Where("podcast_index_id = ?", podcast.PodcastIndexID).
		First(&existing).Error

	if err == nil {
		// Update existing
		podcast.ID = existing.ID
		podcast.CreatedAt = existing.CreatedAt
		return r.UpdatePodcast(ctx, podcast)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new
		return r.CreatePodcast(ctx, podcast)
	}

	return fmt.Errorf("checking existing podcast: %w", err)
}

// GetPodcastByID retrieves a podcast by its database ID
func (r *Repository) GetPodcastByID(ctx context.Context, id uint) (*models.Podcast, error) {
	var podcast models.Podcast
	if err := r.db.WithContext(ctx).First(&podcast, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("podcast not found")
		}
		return nil, fmt.Errorf("getting podcast: %w", err)
	}
	return &podcast, nil
}

// GetPodcastByPodcastIndexID retrieves a podcast by Podcast Index ID
func (r *Repository) GetPodcastByPodcastIndexID(ctx context.Context, piID int64) (*models.Podcast, error) {
	var podcast models.Podcast
	if err := r.db.WithContext(ctx).
		Where("podcast_index_id = ?", piID).
		First(&podcast).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("podcast not found")
		}
		return nil, fmt.Errorf("getting podcast by podcast index id: %w", err)
	}
	return &podcast, nil
}

// GetPodcastByFeedURL retrieves a podcast by feed URL
func (r *Repository) GetPodcastByFeedURL(ctx context.Context, feedURL string) (*models.Podcast, error) {
	var podcast models.Podcast
	if err := r.db.WithContext(ctx).
		Where("feed_url = ?", feedURL).
		First(&podcast).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("podcast not found")
		}
		return nil, fmt.Errorf("getting podcast by feed url: %w", err)
	}
	return &podcast, nil
}

// GetPodcastByITunesID retrieves a podcast by iTunes ID
func (r *Repository) GetPodcastByITunesID(ctx context.Context, itunesID int64) (*models.Podcast, error) {
	var podcast models.Podcast
	if err := r.db.WithContext(ctx).
		Where("itunes_id = ?", itunesID).
		First(&podcast).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("podcast not found")
		}
		return nil, fmt.Errorf("getting podcast by itunes id: %w", err)
	}
	return &podcast, nil
}

// ListPodcasts returns paginated list of podcasts
func (r *Repository) ListPodcasts(ctx context.Context, page, limit int) ([]models.Podcast, int64, error) {
	var podcasts []models.Podcast
	var total int64

	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).Model(&models.Podcast{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting podcasts: %w", err)
	}

	if err := query.
		Order("last_update_time DESC NULLS LAST").
		Limit(limit).
		Offset(offset).
		Find(&podcasts).Error; err != nil {
		return nil, 0, fmt.Errorf("listing podcasts: %w", err)
	}

	return podcasts, total, nil
}

// SearchPodcasts searches podcasts by title or author
func (r *Repository) SearchPodcasts(ctx context.Context, query string, limit int) ([]models.Podcast, error) {
	var podcasts []models.Podcast

	searchPattern := "%" + query + "%"

	if err := r.db.WithContext(ctx).
		Where("title LIKE ? OR author LIKE ?", searchPattern, searchPattern).
		Order("episode_count DESC").
		Limit(limit).
		Find(&podcasts).Error; err != nil {
		return nil, fmt.Errorf("searching podcasts: %w", err)
	}

	return podcasts, nil
}

// UpdateLastFetched updates the last fetched timestamp
func (r *Repository) UpdateLastFetched(ctx context.Context, podcastID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.Podcast{}).
		Where("id = ?", podcastID).
		Updates(map[string]interface{}{
			"last_fetched_at": now,
		}).Error
}

// IncrementFetchCount increments the fetch count
func (r *Repository) IncrementFetchCount(ctx context.Context, podcastID uint) error {
	return r.db.WithContext(ctx).
		Model(&models.Podcast{}).
		Where("id = ?", podcastID).
		Update("fetch_count", gorm.Expr("fetch_count + 1")).Error
}
