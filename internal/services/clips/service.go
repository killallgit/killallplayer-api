package clips

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/jobs"
	"gorm.io/gorm"
)

// Service defines the interface for clip management
type Service interface {
	// CreateClip creates a new clip from audio source
	CreateClip(ctx context.Context, params CreateClipParams) (*models.Clip, error)

	// GetClip retrieves a clip by UUID
	GetClip(ctx context.Context, uuid string) (*models.Clip, error)

	// GetClipsByEpisodeID retrieves all clips for an episode
	GetClipsByEpisodeID(ctx context.Context, episodeID int64) ([]*models.Clip, error)

	// UpdateClipLabel updates the label of a clip
	UpdateClipLabel(ctx context.Context, uuid, newLabel string) (*models.Clip, error)

	// DeleteClip deletes a clip and its file
	DeleteClip(ctx context.Context, uuid string) error

	// ListClips lists clips with optional filters
	ListClips(ctx context.Context, filters ListClipsFilters) ([]*models.Clip, error)

	// ExportDataset exports clips for ML training
	ExportDataset(ctx context.Context, exportPath string) error
}

// CreateClipParams contains parameters for creating a clip
type CreateClipParams struct {
	PodcastIndexEpisodeID int64 // Podcast Index Episode ID for fast lookups (audio URL will be resolved automatically)
	OriginalStartTime     float64
	OriginalEndTime       float64
	Label                 string
	Approved              bool // Whether clip is approved for extraction (false for analysis results)
}

// ListClipsFilters contains filters for listing clips
type ListClipsFilters struct {
	EpisodeID *int64 // Optional: filter by episode ID
	Label     string
	Status    string
	Approved  *bool  // Optional: filter by approval status
	Limit     int
	Offset    int
}

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	db                *gorm.DB
	storage           ClipStorage
	extractor         AudioExtractor
	jobService        jobs.Service
	episodeService    interface{ GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error) }
	audioCacheService interface{ GetCachedAudio(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error) }
}

// NewService creates a new clips service
func NewService(
	db *gorm.DB,
	storage ClipStorage,
	extractor AudioExtractor,
	jobService jobs.Service,
	episodeService interface{ GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error) },
	audioCacheService interface{ GetCachedAudio(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error) },
) Service {
	return &ServiceImpl{
		db:                db,
		storage:           storage,
		extractor:         extractor,
		jobService:        jobService,
		episodeService:    episodeService,
		audioCacheService: audioCacheService,
	}
}

// CreateClip creates a new clip from audio source
func (s *ServiceImpl) CreateClip(ctx context.Context, params CreateClipParams) (*models.Clip, error) {
	// Validate parameters
	if params.OriginalEndTime <= params.OriginalStartTime {
		return nil, fmt.Errorf("invalid time range: start=%f, end=%f", params.OriginalStartTime, params.OriginalEndTime)
	}

	if params.Label == "" {
		return nil, fmt.Errorf("label is required")
	}

	if params.PodcastIndexEpisodeID <= 0 {
		return nil, fmt.Errorf("podcast_index_episode_id must be positive")
	}

	// 1. Get episode from service (auto-fetches from Podcast Index API if not in DB)
	episode, err := s.episodeService.GetEpisodeByPodcastIndexID(ctx, params.PodcastIndexEpisodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode %d: %w", params.PodcastIndexEpisodeID, err)
	}

	// 2. Validate episode has audio URL
	if episode.AudioURL == "" {
		return nil, fmt.Errorf("episode %d has no audio URL", params.PodcastIndexEpisodeID)
	}

	// 3. Check if we have cached audio (optimization - use local file if available)
	var sourceURL string
	if s.audioCacheService != nil {
		cache, err := s.audioCacheService.GetCachedAudio(ctx, params.PodcastIndexEpisodeID)
		if err == nil && cache != nil && cache.OriginalPath != "" {
			// Use cached local file (MUCH faster - no download needed!)
			sourceURL = cache.OriginalPath
			log.Printf("[DEBUG] Using cached audio for episode %d: %s", params.PodcastIndexEpisodeID, sourceURL)
		}
	}

	// 4. Fall back to episode audio URL if not cached
	if sourceURL == "" {
		sourceURL = episode.AudioURL
		log.Printf("[DEBUG] Using remote audio URL for episode %d: %s", params.PodcastIndexEpisodeID, sourceURL)
	}

	// Generate unique filename
	clipID := uuid.New().String()
	filename := fmt.Sprintf("clip_%s.wav", clipID)

	// Determine initial status based on approval
	initialStatus := "detected" // Default for unapproved clips
	if params.Approved {
		initialStatus = "queued" // Will be processed by job system
	}

	// Create clip record
	clip := &models.Clip{
		UUID:                  clipID,
		PodcastIndexEpisodeID: params.PodcastIndexEpisodeID,
		SourceEpisodeURL:      sourceURL, // Store the determined source (URL or local path)
		OriginalStartTime:     params.OriginalStartTime,
		OriginalEndTime:       params.OriginalEndTime,
		Label:                 params.Label,
		ClipFilename:          &filename,
		Status:                initialStatus,
		Extracted:             false,
		Approved:              params.Approved,
		LabelMethod:           "manual", // Default to manual
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Save to database first
	if err := s.db.Create(clip).Error; err != nil {
		return nil, fmt.Errorf("failed to create clip record: %w", err)
	}

	// Only enqueue extraction job for approved clips
	if params.Approved {
		payload := models.JobPayload{
			"clip_uuid": clipID,
		}

		if _, err := s.jobService.EnqueueJob(ctx, models.JobTypeClipExtraction, payload); err != nil {
			// Update clip status to failed if we can't enqueue the job
			s.db.Model(clip).Updates(map[string]interface{}{
				"status":        "failed",
				"error_message": fmt.Sprintf("failed to enqueue extraction job: %v", err),
			})
			return nil, fmt.Errorf("failed to enqueue clip extraction job: %w", err)
		}
		log.Printf("[DEBUG] Enqueued clip extraction job for %s (approved)", clipID)
	} else {
		log.Printf("[DEBUG] Created clip %s with status='detected' (needs approval)", clipID)
	}

	return clip, nil
}

// GetClip retrieves a clip by UUID
func (s *ServiceImpl) GetClip(ctx context.Context, uuid string) (*models.Clip, error) {
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("clip not found")
		}
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}
	return &clip, nil
}

// GetClipsByEpisodeID retrieves all clips for an episode
func (s *ServiceImpl) GetClipsByEpisodeID(ctx context.Context, episodeID int64) ([]*models.Clip, error) {
	var clips []*models.Clip
	if err := s.db.Where("podcast_index_episode_id = ?", episodeID).
		Order("original_start_time ASC").
		Find(&clips).Error; err != nil {
		return nil, fmt.Errorf("failed to get clips for episode: %w", err)
	}
	return clips, nil
}

// UpdateClipLabel updates the label of a clip
func (s *ServiceImpl) UpdateClipLabel(ctx context.Context, uuid, newLabel string) (*models.Clip, error) {
	if newLabel == "" {
		return nil, fmt.Errorf("label cannot be empty")
	}

	// Get the clip
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("clip not found")
		}
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}

	// Skip if label hasn't changed
	if clip.Label == newLabel {
		return &clip, nil
	}

	oldLabel := clip.Label

	// Move the file in storage (only if clip is ready and has a filename)
	if clip.Status == "ready" && clip.ClipFilename != nil {
		if err := s.storage.MoveClip(ctx, oldLabel, newLabel, *clip.ClipFilename); err != nil {
			return nil, fmt.Errorf("failed to move clip file: %w", err)
		}
	}

	// Update database
	clip.Label = newLabel
	clip.UpdatedAt = time.Now()

	if err := s.db.Save(&clip).Error; err != nil {
		// Try to move file back if database update fails
		if clip.Status == "ready" && clip.ClipFilename != nil {
			_ = s.storage.MoveClip(ctx, newLabel, oldLabel, *clip.ClipFilename)
		}
		return nil, fmt.Errorf("failed to update clip: %w", err)
	}

	return &clip, nil
}

// DeleteClip deletes a clip and its file
func (s *ServiceImpl) DeleteClip(ctx context.Context, uuid string) error {
	// Get the clip
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to get clip: %w", err)
	}

	// Delete from storage (only if clip is ready and has a filename)
	if clip.Status == "ready" && clip.ClipFilename != nil {
		if err := s.storage.DeleteClip(ctx, clip.Label, *clip.ClipFilename); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to delete clip file: %v\n", err)
		}
	}

	// Delete from database
	if err := s.db.Delete(&clip).Error; err != nil {
		return fmt.Errorf("failed to delete clip record: %w", err)
	}

	return nil
}

// ListClips lists clips with optional filters
func (s *ServiceImpl) ListClips(ctx context.Context, filters ListClipsFilters) ([]*models.Clip, error) {
	query := s.db.Model(&models.Clip{})

	// Apply filters
	if filters.EpisodeID != nil {
		query = query.Where("podcast_index_episode_id = ?", *filters.EpisodeID)
	}
	if filters.Label != "" {
		query = query.Where("label = ?", filters.Label)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Approved != nil {
		query = query.Where("approved = ?", *filters.Approved)
	}

	// Apply pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	// Order by creation time (newest first)
	query = query.Order("created_at DESC")

	var clips []*models.Clip
	if err := query.Find(&clips).Error; err != nil {
		return nil, fmt.Errorf("failed to list clips: %w", err)
	}

	return clips, nil
}

// ExportDataset exports clips for ML training
func (s *ServiceImpl) ExportDataset(ctx context.Context, exportPath string) error {
	// Get all unique labels
	var labels []string
	if err := s.db.Model(&models.Clip{}).
		Where("status = ?", "ready").
		Distinct("label").
		Pluck("label", &labels).Error; err != nil {
		return fmt.Errorf("failed to get labels: %w", err)
	}

	// Export clips to dataset structure
	if err := s.storage.ExportDataset(ctx, exportPath, labels); err != nil {
		return fmt.Errorf("failed to export dataset: %w", err)
	}

	// Create manifest file
	manifestPath := filepath.Join(exportPath, "manifest.jsonl")
	if err := s.createManifest(ctx, manifestPath); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	return nil
}

// createManifest creates a JSONL manifest file for the dataset
func (s *ServiceImpl) createManifest(ctx context.Context, manifestPath string) error {
	// Get all ready clips
	var clips []*models.Clip
	if err := s.db.Where("status = ?", "ready").Find(&clips).Error; err != nil {
		return fmt.Errorf("failed to get clips: %w", err)
	}

	// Create manifest file
	file, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

	// Write JSONL entries
	for _, clip := range clips {
		export := clip.ToExport()
		line := fmt.Sprintf(`{"file_path":"%s","label":"%s","duration":%.3f,"source_url":"%s","original_start_time":%.3f,"original_end_time":%.3f,"uuid":"%s","created_at":"%s"}`,
			export.FilePath,
			export.Label,
			export.Duration,
			export.SourceURL,
			export.OriginalStartTime,
			export.OriginalEndTime,
			export.UUID,
			export.CreatedAt,
		)
		if _, err := file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write manifest entry: %w", err)
		}
	}

	return nil
}
