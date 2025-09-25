package waveforms

import (
	"context"
	"log"
	"strings"

	"github.com/killallgit/player-api/internal/models"
)

// service implements WaveformService
type service struct {
	repo WaveformRepository
}

// NewService creates a new waveform service
func NewService(repo WaveformRepository) WaveformService {
	return &service{
		repo: repo,
	}
}

// GetWaveform retrieves waveform data for an episode
func (s *service) GetWaveform(ctx context.Context, podcastIndexEpisodeID int64) (*models.Waveform, error) {
	if podcastIndexEpisodeID == 0 {
		return nil, ErrInvalidEpisodeID
	}

	log.Printf("[DEBUG] Getting waveform for Podcast Index Episode ID: %d", podcastIndexEpisodeID)

	waveform, err := s.repo.GetByPodcastIndexEpisodeID(ctx, podcastIndexEpisodeID)
	if err != nil {
		log.Printf("[DEBUG] Failed to get waveform for Podcast Index Episode ID %d: %v", podcastIndexEpisodeID, err)
		return nil, err
	}

	log.Printf("[DEBUG] Found waveform for Podcast Index Episode ID %d: resolution=%d, duration=%.2f",
		podcastIndexEpisodeID, waveform.Resolution, waveform.Duration)

	return waveform, nil
}

// SaveWaveform stores waveform data for an episode
func (s *service) SaveWaveform(ctx context.Context, waveform *models.Waveform) error {
	if waveform.PodcastIndexEpisodeID == 0 {
		return ErrInvalidEpisodeID
	}

	if len(waveform.PeaksData) == 0 {
		return ErrInvalidPeaksData
	}

	// Check if waveform already exists
	exists, err := s.repo.Exists(ctx, waveform.PodcastIndexEpisodeID)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("[DEBUG] Updating existing waveform for Podcast Index Episode ID: %d", waveform.PodcastIndexEpisodeID)
		return s.repo.Update(ctx, waveform)
	}

	log.Printf("[DEBUG] Creating new waveform for Podcast Index Episode ID: %d", waveform.PodcastIndexEpisodeID)
	err = s.repo.Create(ctx, waveform)
	if err != nil {
		// Check if it's a UNIQUE constraint violation (race condition)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Printf("[DEBUG] UNIQUE constraint failed for episode %d, waveform already exists (race condition)", waveform.PodcastIndexEpisodeID)
			// Another worker beat us to it, try to update instead
			return s.repo.Update(ctx, waveform)
		}
		return err
	}
	return nil
}

// DeleteWaveform removes waveform data for an episode
func (s *service) DeleteWaveform(ctx context.Context, podcastIndexEpisodeID int64) error {
	if podcastIndexEpisodeID == 0 {
		return ErrInvalidEpisodeID
	}

	log.Printf("[DEBUG] Deleting waveform for Podcast Index Episode ID: %d", podcastIndexEpisodeID)
	return s.repo.Delete(ctx, podcastIndexEpisodeID)
}

// WaveformExists checks if waveform data exists for an episode
func (s *service) WaveformExists(ctx context.Context, podcastIndexEpisodeID int64) (bool, error) {
	if podcastIndexEpisodeID == 0 {
		return false, ErrInvalidEpisodeID
	}

	exists, err := s.repo.Exists(ctx, podcastIndexEpisodeID)
	log.Printf("[DEBUG] Checking if waveform exists for Podcast Index Episode ID %d: %t", podcastIndexEpisodeID, exists)
	return exists, err
}
