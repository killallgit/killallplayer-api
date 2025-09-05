package transcription

import (
	"context"
	"errors"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// Service implements the TranscriptionService interface
type Service struct {
	repo Repository
}

// NewService creates a new transcription service
func NewService(repo Repository) TranscriptionService {
	return &Service{repo: repo}
}

// GetTranscription retrieves a transcription by episode ID
func (s *Service) GetTranscription(ctx context.Context, episodeID uint) (*models.Transcription, error) {
	return s.repo.GetByEpisodeID(ctx, episodeID)
}

// SaveTranscription saves a new transcription or updates an existing one
func (s *Service) SaveTranscription(ctx context.Context, transcription *models.Transcription) error {
	if transcription == nil {
		return errors.New("transcription cannot be nil")
	}

	// Check if transcription already exists
	existing, err := s.repo.GetByEpisodeID(ctx, transcription.EpisodeID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if existing != nil {
		// Update existing transcription
		existing.Text = transcription.Text
		existing.Language = transcription.Language
		existing.Model = transcription.Model
		existing.Duration = transcription.Duration
		return s.repo.Update(ctx, existing)
	}

	// Create new transcription
	return s.repo.Create(ctx, transcription)
}

// DeleteTranscription removes a transcription by episode ID
func (s *Service) DeleteTranscription(ctx context.Context, episodeID uint) error {
	return s.repo.Delete(ctx, episodeID)
}

// ExistsTranscription checks if a transcription exists for an episode
func (s *Service) ExistsTranscription(ctx context.Context, episodeID uint) (bool, error) {
	return s.repo.Exists(ctx, episodeID)
}
