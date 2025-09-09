package dataset

import (
	"context"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

// Service defines the interface for dataset generation operations
type Service interface {
	// GenerateDataset creates a JSONL dataset from annotations
	GenerateDataset(ctx context.Context, request *GenerateRequest) (*Dataset, error)

	// GetDataset retrieves an existing dataset by ID
	GetDataset(ctx context.Context, id string) (*Dataset, error)

	// ListDatasets lists all available datasets
	ListDatasets(ctx context.Context, filters *ListFilters) ([]Dataset, error)

	// DeleteDataset removes a dataset and its associated files
	DeleteDataset(ctx context.Context, id string) error

	// GetDatasetStats returns statistics about available datasets
	GetDatasetStats(ctx context.Context) (*DatasetStats, error)
}

// GenerateRequest represents a request to generate a dataset
type GenerateRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Label       string            `json:"label"`        // e.g., "advertisement", "music", etc.
	Format      DatasetFormat     `json:"format"`       // "jsonl" or "audiofolder"
	AudioFormat AudioFormat       `json:"audio_format"` // "original" or "processed"
	Filters     *AnnotationFilter `json:"filters,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AnnotationFilter defines filters for selecting annotations
type AnnotationFilter struct {
	EpisodeIDs    []uint    `json:"episode_ids,omitempty"`
	PodcastIDs    []uint    `json:"podcast_ids,omitempty"`
	CreatedAfter  time.Time `json:"created_after,omitempty"`
	CreatedBefore time.Time `json:"created_before,omitempty"`
	MinDuration   float64   `json:"min_duration,omitempty"` // seconds
	MaxDuration   float64   `json:"max_duration,omitempty"` // seconds
	Labels        []string  `json:"labels,omitempty"`       // filter by annotation labels
}

// ListFilters defines filters for listing datasets
type ListFilters struct {
	Label  string `json:"label,omitempty"`
	Format string `json:"format,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Dataset represents a generated dataset
type Dataset struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Label       string        `json:"label"`
	Format      DatasetFormat `json:"format"`
	AudioFormat AudioFormat   `json:"audio_format"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`

	// Dataset statistics
	TotalSamples    int     `json:"total_samples"`
	TotalDuration   float64 `json:"total_duration_seconds"`
	AverageDuration float64 `json:"average_duration_seconds"`
	TotalSize       int64   `json:"total_size_bytes"`

	// File paths
	DatasetPath  string `json:"dataset_path"`  // Path to JSONL or directory
	MetadataPath string `json:"metadata_path"` // Path to metadata file

	// Generation info
	GenerationTime time.Duration     `json:"generation_time"`
	Filters        *AnnotationFilter `json:"filters,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// DatasetEntry represents a single entry in a JSONL dataset
type DatasetEntry struct {
	AudioPath    string  `json:"audio_path"`              // Path or URL to audio file
	StartTime    float64 `json:"start_time"`              // Start time in seconds
	EndTime      float64 `json:"end_time"`                // End time in seconds
	Duration     float64 `json:"duration"`                // Duration in seconds
	Label        string  `json:"label"`                   // Classification label
	EpisodeID    uint    `json:"episode_id"`              // Episode database ID
	EpisodeTitle string  `json:"episode_title,omitempty"` // Episode title
	PodcastName  string  `json:"podcast_name,omitempty"`  // Podcast name
	OriginalURL  string  `json:"original_url,omitempty"`  // Original audio URL
	SampleRate   int     `json:"sample_rate,omitempty"`   // Audio sample rate
	CreatedAt    string  `json:"created_at"`              // ISO 8601 timestamp
}

// DatasetFormat represents the format of the dataset
type DatasetFormat string

const (
	FormatJSONL       DatasetFormat = "jsonl"       // HuggingFace JSONL format
	FormatAudioFolder DatasetFormat = "audiofolder" // HuggingFace AudioFolder format
)

// AudioFormat represents which audio files to use
type AudioFormat string

const (
	AudioFormatOriginal  AudioFormat = "original"  // Use original cached audio files
	AudioFormatProcessed AudioFormat = "processed" // Use processed (16kHz mono) audio files
)

// DatasetStats represents statistics about all datasets
type DatasetStats struct {
	TotalDatasets   int            `json:"total_datasets"`
	TotalSamples    int            `json:"total_samples"`
	TotalDuration   float64        `json:"total_duration_seconds"`
	TotalSize       int64          `json:"total_size_bytes"`
	ByLabel         map[string]int `json:"by_label"`        // Sample count by label
	ByFormat        map[string]int `json:"by_format"`       // Dataset count by format
	ByAudioFormat   map[string]int `json:"by_audio_format"` // Dataset count by audio format
	CreatedToday    int            `json:"created_today"`
	CreatedThisWeek int            `json:"created_this_week"`
}

// Repository defines the interface for dataset persistence
type Repository interface {
	// Create creates a new dataset record
	Create(ctx context.Context, dataset *models.Dataset) error

	// GetByID retrieves a dataset by ID
	GetByID(ctx context.Context, id string) (*models.Dataset, error)

	// List retrieves datasets with optional filters
	List(ctx context.Context, filters *ListFilters) ([]models.Dataset, error)

	// Update updates an existing dataset
	Update(ctx context.Context, dataset *models.Dataset) error

	// Delete deletes a dataset record
	Delete(ctx context.Context, id string) error

	// GetStats retrieves dataset statistics
	GetStats(ctx context.Context) (*DatasetStats, error)
}
