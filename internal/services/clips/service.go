package clips

import (
	"context"
	"fmt"
	"io"
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

	// ApproveClip marks a clip as approved for extraction/export
	ApproveClip(ctx context.Context, uuid string) (*models.Clip, error)

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
	Approved  *bool // Optional: filter by approval status
	Limit     int
	Offset    int
}

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	db             *gorm.DB
	storage        ClipStorage
	extractor      AudioExtractor
	jobService     jobs.Service
	episodeService interface {
		GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error)
	}
	audioCacheService interface {
		GetCachedAudio(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error)
	}
}

func NewService(
	db *gorm.DB,
	storage ClipStorage,
	extractor AudioExtractor,
	jobService jobs.Service,
	episodeService interface {
		GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error)
	},
	audioCacheService interface {
		GetCachedAudio(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error)
	},
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

func (s *ServiceImpl) CreateClip(ctx context.Context, params CreateClipParams) (*models.Clip, error) {
	if params.OriginalEndTime <= params.OriginalStartTime {
		return nil, fmt.Errorf("invalid time range: start=%f, end=%f", params.OriginalStartTime, params.OriginalEndTime)
	}

	if params.Label == "" {
		return nil, fmt.Errorf("label is required")
	}

	if params.PodcastIndexEpisodeID <= 0 {
		return nil, fmt.Errorf("podcast_index_episode_id must be positive")
	}

	episode, err := s.episodeService.GetEpisodeByPodcastIndexID(ctx, params.PodcastIndexEpisodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode %d: %w", params.PodcastIndexEpisodeID, err)
	}

	if episode.AudioURL == "" {
		return nil, fmt.Errorf("episode %d has no audio URL", params.PodcastIndexEpisodeID)
	}

	var sourceURL string
	if s.audioCacheService != nil {
		cache, err := s.audioCacheService.GetCachedAudio(ctx, params.PodcastIndexEpisodeID)
		if err == nil && cache != nil && cache.OriginalPath != "" {
			// Use cached local file (MUCH faster - no download needed!)
			sourceURL = cache.OriginalPath
			log.Printf("[DEBUG] Using cached audio for episode %d: %s", params.PodcastIndexEpisodeID, sourceURL)
		}
	}

	if sourceURL == "" {
		sourceURL = episode.AudioURL
		log.Printf("[DEBUG] Using remote audio URL for episode %d: %s", params.PodcastIndexEpisodeID, sourceURL)
	}

	clipID := uuid.New().String()
	filename := fmt.Sprintf("clip_%s.wav", clipID)

	// Clips are just metadata - no extraction until export
	initialStatus := "pending"

	clip := &models.Clip{
		UUID:                  clipID,
		PodcastIndexEpisodeID: params.PodcastIndexEpisodeID,
		SourceEpisodeURL:      sourceURL,
		OriginalStartTime:     params.OriginalStartTime,
		OriginalEndTime:       params.OriginalEndTime,
		Label:                 params.Label,
		ClipFilename:          &filename,
		Status:                initialStatus,
		Extracted:             false,
		Approved:              params.Approved,
		LabelMethod:           "manual",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := s.db.Create(clip).Error; err != nil {
		return nil, fmt.Errorf("failed to create clip record: %w", err)
	}

	log.Printf("[DEBUG] Created clip %s (approved=%v, status=pending)", clipID, params.Approved)
	return clip, nil
}

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

func (s *ServiceImpl) GetClipsByEpisodeID(ctx context.Context, episodeID int64) ([]*models.Clip, error) {
	var clips []*models.Clip
	if err := s.db.Where("podcast_index_episode_id = ?", episodeID).
		Order("original_start_time ASC").
		Find(&clips).Error; err != nil {
		return nil, fmt.Errorf("failed to get clips for episode: %w", err)
	}
	return clips, nil
}

func (s *ServiceImpl) UpdateClipLabel(ctx context.Context, uuid, newLabel string) (*models.Clip, error) {
	if newLabel == "" {
		return nil, fmt.Errorf("label cannot be empty")
	}

	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("clip not found")
		}
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}

	if clip.Label == newLabel {
		return &clip, nil
	}

	oldLabel := clip.Label

	if clip.Status == "ready" && clip.ClipFilename != nil {
		if err := s.storage.MoveClip(ctx, oldLabel, newLabel, *clip.ClipFilename); err != nil {
			return nil, fmt.Errorf("failed to move clip file: %w", err)
		}
	}

	clip.Label = newLabel
	clip.UpdatedAt = time.Now()

	if err := s.db.Save(&clip).Error; err != nil {
		if clip.Status == "ready" && clip.ClipFilename != nil {
			_ = s.storage.MoveClip(ctx, newLabel, oldLabel, *clip.ClipFilename)
		}
		return nil, fmt.Errorf("failed to update clip: %w", err)
	}

	return &clip, nil
}

func (s *ServiceImpl) ApproveClip(ctx context.Context, uuid string) (*models.Clip, error) {
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("clip not found")
		}
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}

	if clip.Approved {
		return &clip, nil // Already approved - idempotent operation
	}

	clip.Approved = true
	clip.UpdatedAt = time.Now()

	if err := s.db.Save(&clip).Error; err != nil {
		return nil, fmt.Errorf("failed to approve clip: %w", err)
	}

	return &clip, nil
}

func (s *ServiceImpl) DeleteClip(ctx context.Context, uuid string) error {
	var clip models.Clip
	if err := s.db.Where("uuid = ?", uuid).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to get clip: %w", err)
	}

	if clip.Status == "ready" && clip.ClipFilename != nil {
		if err := s.storage.DeleteClip(ctx, clip.Label, *clip.ClipFilename); err != nil {
			fmt.Printf("Warning: failed to delete clip file: %v\n", err)
		}
	}

	if err := s.db.Delete(&clip).Error; err != nil {
		return fmt.Errorf("failed to delete clip record: %w", err)
	}

	return nil
}

func (s *ServiceImpl) ListClips(ctx context.Context, filters ListClipsFilters) ([]*models.Clip, error) {
	query := s.db.Model(&models.Clip{})

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

	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	query = query.Order("created_at DESC")

	var clips []*models.Clip
	if err := query.Find(&clips).Error; err != nil {
		return nil, fmt.Errorf("failed to list clips: %w", err)
	}

	return clips, nil
}

// ExportDataset exports all approved clips to a directory for ML training.
//
// Extraction workflow (lazy evaluation with caching):
// 1. Query all approved clips (may be pending or already extracted)
// 2. For extracted clips: copy from storage cache to export dir (fast - no download)
// 3. For pending clips: extract to temp → save to storage → copy to export dir
// 4. Generate manifest.jsonl with metadata for all successfully exported clips
//
// This approach provides:
// - First export: Downloads and extracts audio (slower)
// - Subsequent exports: Reuses cached clips from storage (much faster)
// - Atomic operations: DB updates wrapped in transactions with storage cleanup on failure
// - Exact time ranges: No padding or cropping (unless targetDuration configured)
func (s *ServiceImpl) ExportDataset(ctx context.Context, exportPath string) error {
	// Query ALL approved clips (not just already-extracted ones)
	var clips []*models.Clip
	if err := s.db.Where("approved = ?", true).Find(&clips).Error; err != nil {
		return fmt.Errorf("failed to get approved clips: %w", err)
	}

	if len(clips) == 0 {
		log.Printf("[INFO] No approved clips to export")
		return nil
	}

	log.Printf("[INFO] Exporting %d approved clips", len(clips))

	// Track successfully exported clips for manifest
	var exportedClips []*models.Clip

	// Process each clip
	for _, clip := range clips {
		if clip.Extracted {
			// Clip already extracted - just copy it
			log.Printf("[DEBUG] Copying already-extracted clip %s", clip.UUID)
			if err := s.copyExtractedClip(ctx, clip, exportPath); err != nil {
				log.Printf("[WARN] Failed to copy clip %s: %v", clip.UUID, err)
				continue
			}
			exportedClips = append(exportedClips, clip)
		} else {
			// Extract clip on-demand during export
			log.Printf("[DEBUG] Extracting clip %s on-demand", clip.UUID)
			if err := s.extractClipForExport(ctx, clip, exportPath); err != nil {
				log.Printf("[WARN] Failed to extract clip %s: %v", clip.UUID, err)
				// Update clip status to failed
				s.db.Model(clip).Updates(map[string]interface{}{
					"status":        "failed",
					"error_message": err.Error(),
				})
				continue
			}
			exportedClips = append(exportedClips, clip)
		}
	}

	log.Printf("[INFO] Successfully exported %d/%d clips", len(exportedClips), len(clips))

	// Create manifest from successfully exported clips
	manifestPath := filepath.Join(exportPath, "manifest.jsonl")
	if err := s.createManifestForClips(ctx, manifestPath, exportedClips); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	return nil
}

// extractClipForExport extracts a clip on-demand during dataset export
// This workflow: extract to temp → save to storage (for caching) → copy to export dir
func (s *ServiceImpl) extractClipForExport(ctx context.Context, clip *models.Clip, exportPath string) error {
	if clip.ClipFilename == nil {
		return fmt.Errorf("clip has no filename")
	}

	// Step 1: Extract to temporary file
	tempFile := filepath.Join(os.TempDir(), *clip.ClipFilename)
	result, err := s.extractor.ExtractClip(ctx, ExtractParams{
		SourceURL:  clip.SourceEpisodeURL,
		StartTime:  clip.OriginalStartTime,
		EndTime:    clip.OriginalEndTime,
		OutputPath: tempFile,
	})

	if err != nil {
		return fmt.Errorf("failed to extract clip: %w", err)
	}

	// Ensure temp file cleanup
	defer os.Remove(tempFile)

	// Step 2: Save to storage for future exports (caching)
	file, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	if err := s.storage.SaveClip(ctx, clip.Label, *clip.ClipFilename, file); err != nil {
		return fmt.Errorf("failed to save clip to storage: %w", err)
	}

	// Step 3: Update clip record in transaction (atomic DB operation)
	updates := map[string]interface{}{
		"extracted":       true,
		"clip_duration":   result.Duration,
		"clip_size_bytes": result.SizeBytes,
		"status":          "ready",
		"updated_at":      time.Now(),
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(clip).Updates(updates).Error
	})

	if err != nil {
		// DB update failed - clean up the storage file to maintain consistency
		log.Printf("[ERROR] Failed to update clip record for %s: %v", clip.UUID, err)
		if cleanupErr := s.storage.DeleteClip(ctx, clip.Label, *clip.ClipFilename); cleanupErr != nil {
			log.Printf("[ERROR] Failed to clean up storage file after DB failure: %v", cleanupErr)
		}
		return fmt.Errorf("failed to update clip record: %w", err)
	}

	// Update in-memory clip for manifest
	clip.Extracted = true
	clip.ClipDuration = &result.Duration
	clip.ClipSizeBytes = &result.SizeBytes
	clip.Status = "ready"

	// Step 4: Copy from storage to export directory
	return s.copyFromStorageToExport(clip, exportPath)
}

// copyExtractedClip copies an already-extracted clip from storage to the export directory
// This is much faster than re-extracting since the clip is already cached in storage
func (s *ServiceImpl) copyExtractedClip(ctx context.Context, clip *models.Clip, exportPath string) error {
	if clip.ClipFilename == nil {
		return fmt.Errorf("clip has no filename")
	}

	log.Printf("[DEBUG] Copying already-extracted clip %s from storage", clip.UUID)

	// Copy from storage to export directory (fast - no download/extraction needed)
	return s.copyFromStorageToExport(clip, exportPath)
}

// createManifestForClips creates a manifest file from a list of clips
func (s *ServiceImpl) createManifestForClips(ctx context.Context, manifestPath string, clips []*models.Clip) error {
	file, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

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

// copyFromStorageToExport copies a clip from storage to the export directory
// Uses storage abstraction (GetClip) to work with any storage backend
func (s *ServiceImpl) copyFromStorageToExport(clip *models.Clip, exportPath string) error {
	if clip.ClipFilename == nil {
		return fmt.Errorf("clip has no filename")
	}

	// Use storage abstraction to get clip data
	reader, err := s.storage.GetClip(context.Background(), clip.Label, *clip.ClipFilename)
	if err != nil {
		return fmt.Errorf("failed to get clip from storage: %w", err)
	}
	defer reader.Close()

	// Create destination directory
	dstDir := filepath.Join(exportPath, clip.Label)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create export label directory: %w", err)
	}

	// Write to destination file
	dstPath := filepath.Join(dstDir, *clip.ClipFilename)
	destFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy data from storage to destination
	if _, err := io.Copy(destFile, reader); err != nil {
		return fmt.Errorf("failed to copy clip data: %w", err)
	}

	return destFile.Sync()
}
