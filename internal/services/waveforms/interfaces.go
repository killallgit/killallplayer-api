package waveforms

import (
	"context"

	"github.com/killallgit/player-api/internal/models"
)

// WaveformService defines the interface for waveform operations
type WaveformService interface {
	// GetWaveform retrieves waveform data for an episode
	GetWaveform(ctx context.Context, episodeID uint) (*models.Waveform, error)
	
	// SaveWaveform stores waveform data for an episode
	SaveWaveform(ctx context.Context, waveform *models.Waveform) error
	
	// DeleteWaveform removes waveform data for an episode
	DeleteWaveform(ctx context.Context, episodeID uint) error
	
	// WaveformExists checks if waveform data exists for an episode
	WaveformExists(ctx context.Context, episodeID uint) (bool, error)
}

// WaveformRepository defines the interface for waveform data access
type WaveformRepository interface {
	// GetByEpisodeID retrieves waveform by episode ID
	GetByEpisodeID(ctx context.Context, episodeID uint) (*models.Waveform, error)
	
	// Create saves a new waveform
	Create(ctx context.Context, waveform *models.Waveform) error
	
	// Update modifies an existing waveform
	Update(ctx context.Context, waveform *models.Waveform) error
	
	// Delete removes a waveform by episode ID
	Delete(ctx context.Context, episodeID uint) error
	
	// Exists checks if a waveform exists for an episode
	Exists(ctx context.Context, episodeID uint) (bool, error)
}