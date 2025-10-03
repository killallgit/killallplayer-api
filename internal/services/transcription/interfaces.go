package transcription

import (
	"context"

	"github.com/killallgit/player-api/internal/models"
)

// TranscriptionService defines the interface for transcription operations
type TranscriptionService interface {
	// GetTranscription retrieves a transcription by podcast index episode ID
	GetTranscription(ctx context.Context, podcastIndexEpisodeID int64) (*models.Transcription, error)

	// SaveTranscription saves a new transcription or updates an existing one
	SaveTranscription(ctx context.Context, transcription *models.Transcription) error

	// DeleteTranscription removes a transcription by podcast index episode ID
	DeleteTranscription(ctx context.Context, podcastIndexEpisodeID int64) error

	// ExistsTranscription checks if a transcription exists for an episode
	ExistsTranscription(ctx context.Context, podcastIndexEpisodeID int64) (bool, error)
}

// Repository defines the interface for transcription data persistence
type Repository interface {
	// Create creates a new transcription
	Create(ctx context.Context, transcription *models.Transcription) error

	// GetByEpisodeID retrieves a transcription by podcast index episode ID
	GetByEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) (*models.Transcription, error)

	// Update updates an existing transcription
	Update(ctx context.Context, transcription *models.Transcription) error

	// Delete removes a transcription
	Delete(ctx context.Context, podcastIndexEpisodeID int64) error

	// Exists checks if a transcription exists for an episode
	Exists(ctx context.Context, podcastIndexEpisodeID int64) (bool, error)
}
