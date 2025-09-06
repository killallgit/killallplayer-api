package waveforms

import (
	"context"
	"log"

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
func (s *service) GetWaveform(ctx context.Context, episodeID uint) (*models.Waveform, error) {
	if episodeID == 0 {
		return nil, ErrInvalidEpisodeID
	}

	log.Printf("[DEBUG] Getting waveform for episode database ID: %d", episodeID)

	waveform, err := s.repo.GetByEpisodeID(ctx, episodeID)
	if err != nil {
		log.Printf("[DEBUG] Failed to get waveform for episode database ID %d: %v", episodeID, err)
		return nil, err
	}

	log.Printf("[DEBUG] Found waveform for episode database ID %d: resolution=%d, duration=%.2f",
		episodeID, waveform.Resolution, waveform.Duration)

	return waveform, nil
}

// SaveWaveform stores waveform data for an episode
func (s *service) SaveWaveform(ctx context.Context, waveform *models.Waveform) error {
	if waveform.EpisodeID == 0 {
		return ErrInvalidEpisodeID
	}

	if len(waveform.PeaksData) == 0 {
		return ErrInvalidPeaksData
	}

	// Check if waveform already exists
	exists, err := s.repo.Exists(ctx, waveform.EpisodeID)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("[DEBUG] Updating existing waveform for episode database ID: %d", waveform.EpisodeID)
		return s.repo.Update(ctx, waveform)
	}

	log.Printf("[DEBUG] Creating new waveform for episode database ID: %d", waveform.EpisodeID)
	return s.repo.Create(ctx, waveform)
}

// DeleteWaveform removes waveform data for an episode
func (s *service) DeleteWaveform(ctx context.Context, episodeID uint) error {
	if episodeID == 0 {
		return ErrInvalidEpisodeID
	}

	log.Printf("[DEBUG] Deleting waveform for episode database ID: %d", episodeID)
	return s.repo.Delete(ctx, episodeID)
}

// WaveformExists checks if waveform data exists for an episode
func (s *service) WaveformExists(ctx context.Context, episodeID uint) (bool, error) {
	if episodeID == 0 {
		return false, ErrInvalidEpisodeID
	}

	exists, err := s.repo.Exists(ctx, episodeID)
	log.Printf("[DEBUG] Checking if waveform exists for episode database ID %d: %t", episodeID, exists)
	return exists, err
}
