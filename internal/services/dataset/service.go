package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/annotations"
	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/episodes"
	"gorm.io/gorm"
)

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	repository        Repository
	annotationService annotations.Service
	audioCacheService audiocache.Service
	episodeService    episodes.EpisodeService
	storageBasePath   string
}

// NewService creates a new dataset service
func NewService(
	repository Repository,
	annotationService annotations.Service,
	audioCacheService audiocache.Service,
	episodeService episodes.EpisodeService,
	storageBasePath string,
) Service {
	return &ServiceImpl{
		repository:        repository,
		annotationService: annotationService,
		audioCacheService: audioCacheService,
		episodeService:    episodeService,
		storageBasePath:   storageBasePath,
	}
}

// GenerateDataset creates a JSONL dataset from annotations
func (s *ServiceImpl) GenerateDataset(ctx context.Context, request *GenerateRequest) (*Dataset, error) {
	startTime := time.Now()
	log.Printf("[INFO] Starting dataset generation: %s (label: %s, format: %s)", request.Name, request.Label, request.Format)

	// Create base storage path
	if err := os.MkdirAll(s.storageBasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Generate unique dataset ID and paths
	datasetID := generateDatasetID()
	datasetDir := filepath.Join(s.storageBasePath, datasetID)
	if err := os.MkdirAll(datasetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create dataset directory: %w", err)
	}

	// Get all annotations that match our criteria
	annotations, err := s.getFilteredAnnotations(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get annotations: %w", err)
	}

	if len(annotations) == 0 {
		return nil, fmt.Errorf("no annotations found matching the specified criteria")
	}

	log.Printf("[INFO] Found %d annotations for dataset generation", len(annotations))

	// Generate dataset entries
	entries, stats, err := s.generateDatasetEntries(ctx, annotations, request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate dataset entries: %w", err)
	}

	// Write dataset file(s)
	datasetPath, err := s.writeDatasetFiles(ctx, datasetDir, entries, request)
	if err != nil {
		return nil, fmt.Errorf("failed to write dataset files: %w", err)
	}

	// Write metadata file
	metadataPath, err := s.writeMetadataFile(ctx, datasetDir, request, stats)
	if err != nil {
		return nil, fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Create dataset record
	generationTime := time.Since(startTime)
	dataset := &models.Dataset{
		ID:               datasetID,
		Name:             request.Name,
		Description:      request.Description,
		Label:            request.Label,
		Format:           string(request.Format),
		AudioFormat:      string(request.AudioFormat),
		TotalSamples:     stats.TotalSamples,
		TotalDuration:    stats.TotalDuration,
		AverageDuration:  stats.AverageDuration,
		TotalSize:        stats.TotalSize,
		DatasetPath:      datasetPath,
		MetadataPath:     metadataPath,
		GenerationTimeMs: generationTime.Milliseconds(),
	}

	// Encode filters and metadata as JSON
	if request.Filters != nil {
		filtersJSON, _ := json.Marshal(request.Filters)
		dataset.FiltersJSON = string(filtersJSON)
	}
	if request.Metadata != nil {
		metadataJSON, _ := json.Marshal(request.Metadata)
		dataset.MetadataJSON = string(metadataJSON)
	}

	// Save to database
	if err := s.repository.Create(ctx, dataset); err != nil {
		return nil, fmt.Errorf("failed to save dataset record: %w", err)
	}

	log.Printf("[INFO] Dataset generation completed: %s (%d samples, %.1fs, %s)",
		datasetID, stats.TotalSamples, generationTime.Seconds(), formatBytes(stats.TotalSize))

	return s.modelToDataset(dataset), nil
}

// GetDataset retrieves an existing dataset by ID
func (s *ServiceImpl) GetDataset(ctx context.Context, id string) (*Dataset, error) {
	dataset, err := s.repository.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("dataset not found: %s", id)
		}
		return nil, err
	}
	return s.modelToDataset(dataset), nil
}

// ListDatasets lists all available datasets
func (s *ServiceImpl) ListDatasets(ctx context.Context, filters *ListFilters) ([]Dataset, error) {
	datasets, err := s.repository.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	result := make([]Dataset, len(datasets))
	for i, dataset := range datasets {
		result[i] = *s.modelToDataset(&dataset)
	}

	return result, nil
}

// DeleteDataset removes a dataset and its associated files
func (s *ServiceImpl) DeleteDataset(ctx context.Context, id string) error {
	// Get dataset to find file paths
	dataset, err := s.repository.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("dataset not found: %s", id)
		}
		return err
	}

	// Remove dataset directory
	datasetDir := filepath.Dir(dataset.DatasetPath)
	if err := os.RemoveAll(datasetDir); err != nil {
		log.Printf("[WARN] Failed to remove dataset directory %s: %v", datasetDir, err)
	}

	// Remove from database
	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete dataset record: %w", err)
	}

	log.Printf("[INFO] Dataset deleted: %s", id)
	return nil
}

// GetDatasetStats returns statistics about available datasets
func (s *ServiceImpl) GetDatasetStats(ctx context.Context) (*DatasetStats, error) {
	return s.repository.GetStats(ctx)
}

// getFilteredAnnotations retrieves annotations based on request filters
func (s *ServiceImpl) getFilteredAnnotations(ctx context.Context, request *GenerateRequest) ([]models.Annotation, error) {
	// For now, get all annotations and filter client-side
	// In a real implementation, you'd want to push filtering to the database

	// Get all episodes to collect their annotations
	var allAnnotations []models.Annotation

	// If specific episode IDs are provided, use those
	if request.Filters != nil && len(request.Filters.EpisodeIDs) > 0 {
		for _, podcastIndexEpisodeID := range request.Filters.EpisodeIDs {
			annotations, err := s.annotationService.GetAnnotationsByPodcastIndexEpisodeID(ctx, int64(podcastIndexEpisodeID))
			if err != nil {
				log.Printf("[WARN] Failed to get annotations for Podcast Index episode %d: %v", podcastIndexEpisodeID, err)
				continue
			}
			allAnnotations = append(allAnnotations, annotations...)
		}
	} else {
		// This is a simplified approach - in production you'd want a more efficient query
		return nil, fmt.Errorf("episode IDs filter is required for now")
	}

	// Apply additional filters
	var filteredAnnotations []models.Annotation
	for _, annotation := range allAnnotations {
		if s.matchesFilters(&annotation, request.Filters) {
			// Override label if specified in request
			if request.Label != "" {
				annotation.Label = request.Label
			}
			filteredAnnotations = append(filteredAnnotations, annotation)
		}
	}

	return filteredAnnotations, nil
}

// matchesFilters checks if an annotation matches the request filters
func (s *ServiceImpl) matchesFilters(annotation *models.Annotation, filters *AnnotationFilter) bool {
	if filters == nil {
		return true
	}

	// Check duration filters
	duration := annotation.EndTime - annotation.StartTime
	if filters.MinDuration > 0 && duration < filters.MinDuration {
		return false
	}
	if filters.MaxDuration > 0 && duration > filters.MaxDuration {
		return false
	}

	// Check time filters
	if !filters.CreatedAfter.IsZero() && annotation.CreatedAt.Before(filters.CreatedAfter) {
		return false
	}
	if !filters.CreatedBefore.IsZero() && annotation.CreatedAt.After(filters.CreatedBefore) {
		return false
	}

	// Check label filters
	if len(filters.Labels) > 0 {
		found := false
		for _, label := range filters.Labels {
			if annotation.Label == label {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// generateDatasetEntries creates dataset entries from annotations
func (s *ServiceImpl) generateDatasetEntries(ctx context.Context, annotations []models.Annotation, request *GenerateRequest) ([]DatasetEntry, *DatasetGenerationStats, error) {
	var entries []DatasetEntry
	var totalDuration float64
	var totalSize int64
	successCount := 0

	for _, annotation := range annotations {
		// Get episode details using Podcast Index ID
		episode, err := s.episodeService.GetEpisodeByPodcastIndexID(ctx, annotation.PodcastIndexEpisodeID)
		if err != nil {
			log.Printf("[WARN] Failed to get episode with Podcast Index ID %d for annotation: %v", annotation.PodcastIndexEpisodeID, err)
			continue
		}

		// Get cached audio using Podcast Index ID
		audioCache, err := s.audioCacheService.GetCachedAudio(ctx, annotation.PodcastIndexEpisodeID)
		if err != nil || audioCache == nil {
			log.Printf("[WARN] No cached audio found for Podcast Index episode %d, skipping annotation", annotation.PodcastIndexEpisodeID)
			continue
		}

		// Determine which audio file to use
		var audioPath string
		var audioSize int64
		var sampleRate int

		switch request.AudioFormat {
		case AudioFormatProcessed:
			if audioCache.ProcessedPath != "" {
				audioPath = audioCache.ProcessedPath
				audioSize = audioCache.ProcessedSize
				sampleRate = audioCache.SampleRate
			} else {
				log.Printf("[WARN] No processed audio available for Podcast Index episode %d, using original", annotation.PodcastIndexEpisodeID)
				audioPath = audioCache.OriginalPath
				audioSize = audioCache.OriginalSize
				sampleRate = 0 // Unknown for original
			}
		case AudioFormatOriginal:
			audioPath = audioCache.OriginalPath
			audioSize = audioCache.OriginalSize
			sampleRate = 0 // Unknown for original
		default:
			audioPath = audioCache.OriginalPath
			audioSize = audioCache.OriginalSize
			sampleRate = 0
		}

		if audioPath == "" {
			log.Printf("[WARN] No audio path available for Podcast Index episode %d, skipping annotation", annotation.PodcastIndexEpisodeID)
			continue
		}

		// Create dataset entry
		duration := annotation.EndTime - annotation.StartTime
		entry := DatasetEntry{
			AudioPath:    audioPath,
			StartTime:    annotation.StartTime,
			EndTime:      annotation.EndTime,
			Duration:     duration,
			Label:        annotation.Label,
			EpisodeID:    uint(annotation.PodcastIndexEpisodeID),
			EpisodeTitle: episode.Title,
			PodcastName:  episode.Description, // TODO: Get actual podcast name
			OriginalURL:  audioCache.OriginalURL,
			SampleRate:   sampleRate,
			CreatedAt:    annotation.CreatedAt.Format(time.RFC3339),
		}

		entries = append(entries, entry)
		totalDuration += duration
		totalSize += audioSize
		successCount++
	}

	var averageDuration float64
	if successCount > 0 {
		averageDuration = totalDuration / float64(successCount)
	}

	stats := &DatasetGenerationStats{
		TotalSamples:    successCount,
		TotalDuration:   totalDuration,
		AverageDuration: averageDuration,
		TotalSize:       totalSize,
	}

	log.Printf("[INFO] Generated %d dataset entries from %d annotations", successCount, len(annotations))
	return entries, stats, nil
}

// writeDatasetFiles writes the dataset to disk in the requested format
func (s *ServiceImpl) writeDatasetFiles(_ context.Context, datasetDir string, entries []DatasetEntry, request *GenerateRequest) (string, error) {
	switch request.Format {
	case FormatJSONL:
		return s.writeJSONLDataset(datasetDir, entries, request)
	case FormatAudioFolder:
		return s.writeAudioFolderDataset(datasetDir, entries, request)
	default:
		return "", fmt.Errorf("unsupported dataset format: %s", request.Format)
	}
}

// writeJSONLDataset writes a JSONL format dataset
func (s *ServiceImpl) writeJSONLDataset(datasetDir string, entries []DatasetEntry, request *GenerateRequest) (string, error) {
	datasetPath := filepath.Join(datasetDir, "dataset.jsonl")

	file, err := os.Create(datasetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create JSONL file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return "", fmt.Errorf("failed to encode entry: %w", err)
		}
	}

	return datasetPath, nil
}

// writeAudioFolderDataset writes an AudioFolder format dataset
func (s *ServiceImpl) writeAudioFolderDataset(datasetDir string, entries []DatasetEntry, request *GenerateRequest) (string, error) {
	// AudioFolder format creates subdirectories by label
	labelDir := filepath.Join(datasetDir, request.Label)
	if err := os.MkdirAll(labelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create label directory: %w", err)
	}

	// Write metadata file for each entry
	metadataPath := filepath.Join(datasetDir, "metadata.jsonl")
	file, err := os.Create(metadataPath)
	if err != nil {
		return "", fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		// Create metadata entry for AudioFolder format
		metadata := map[string]interface{}{
			"file_name":     filepath.Base(entry.AudioPath),
			"label":         entry.Label,
			"start_time":    entry.StartTime,
			"end_time":      entry.EndTime,
			"duration":      entry.Duration,
			"episode_id":    entry.EpisodeID,
			"episode_title": entry.EpisodeTitle,
			"podcast_name":  entry.PodcastName,
			"created_at":    entry.CreatedAt,
		}

		if err := encoder.Encode(metadata); err != nil {
			return "", fmt.Errorf("failed to encode metadata: %w", err)
		}
	}

	return datasetDir, nil
}

// writeMetadataFile writes dataset metadata
func (s *ServiceImpl) writeMetadataFile(ctx context.Context, datasetDir string, request *GenerateRequest, stats *DatasetGenerationStats) (string, error) {
	metadataPath := filepath.Join(datasetDir, "info.json")

	metadata := map[string]interface{}{
		"name":             request.Name,
		"description":      request.Description,
		"label":            request.Label,
		"format":           request.Format,
		"audio_format":     request.AudioFormat,
		"total_samples":    stats.TotalSamples,
		"total_duration":   stats.TotalDuration,
		"average_duration": stats.AverageDuration,
		"total_size":       stats.TotalSize,
		"generated_at":     time.Now().Format(time.RFC3339),
		"filters":          request.Filters,
		"metadata":         request.Metadata,
	}

	file, err := os.Create(metadataPath)
	if err != nil {
		return "", fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return "", fmt.Errorf("failed to encode metadata: %w", err)
	}

	return metadataPath, nil
}

// modelToDataset converts a model to service Dataset
func (s *ServiceImpl) modelToDataset(dataset *models.Dataset) *Dataset {
	result := &Dataset{
		ID:              dataset.ID,
		Name:            dataset.Name,
		Description:     dataset.Description,
		Label:           dataset.Label,
		Format:          DatasetFormat(dataset.Format),
		AudioFormat:     AudioFormat(dataset.AudioFormat),
		CreatedAt:       dataset.CreatedAt,
		UpdatedAt:       dataset.UpdatedAt,
		TotalSamples:    dataset.TotalSamples,
		TotalDuration:   dataset.TotalDuration,
		AverageDuration: dataset.AverageDuration,
		TotalSize:       dataset.TotalSize,
		DatasetPath:     dataset.DatasetPath,
		MetadataPath:    dataset.MetadataPath,
		GenerationTime:  time.Duration(dataset.GenerationTimeMs) * time.Millisecond,
	}

	// Decode JSON fields
	if dataset.FiltersJSON != "" {
		var filters AnnotationFilter
		if err := json.Unmarshal([]byte(dataset.FiltersJSON), &filters); err == nil {
			result.Filters = &filters
		}
	}

	if dataset.MetadataJSON != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(dataset.MetadataJSON), &metadata); err == nil {
			result.Metadata = metadata
		}
	}

	return result
}

// DatasetGenerationStats holds statistics about dataset generation
type DatasetGenerationStats struct {
	TotalSamples    int
	TotalDuration   float64
	AverageDuration float64
	TotalSize       int64
}

// formatBytes formats byte count as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// generateDatasetID generates a unique dataset ID with timestamp
func generateDatasetID() string {
	return fmt.Sprintf("ds-%d", time.Now().UnixNano()/1000000) // milliseconds
}
