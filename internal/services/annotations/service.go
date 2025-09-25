package annotations

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/killallgit/player-api/internal/models"
)

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	repository Repository
}

// NewService creates a new annotation service
func NewService(repository Repository) Service {
	return &ServiceImpl{
		repository: repository,
	}
}

// CreateAnnotation creates a new annotation with validation
func (s *ServiceImpl) CreateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	// Validate annotation
	if annotation.Label == "" {
		return fmt.Errorf("Label is required")
	}
	if annotation.StartTime >= annotation.EndTime {
		return fmt.Errorf("Start time must be before end time")
	}
	if annotation.PodcastIndexEpisodeID == 0 {
		return fmt.Errorf("Podcast Index Episode ID is required")
	}

	// Generate UUID if not provided
	if annotation.UUID == "" {
		annotation.UUID = uuid.New().String()
	}

	return s.repository.CreateAnnotation(ctx, annotation)
}

// GetAnnotationByID retrieves an annotation by its ID
func (s *ServiceImpl) GetAnnotationByID(ctx context.Context, id uint) (*models.Annotation, error) {
	return s.repository.GetAnnotationByID(ctx, id)
}

// GetAnnotationsByPodcastIndexEpisodeID retrieves all annotations for a specific episode
func (s *ServiceImpl) GetAnnotationsByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) ([]models.Annotation, error) {
	return s.repository.GetAnnotationsByPodcastIndexEpisodeID(ctx, podcastIndexEpisodeID)
}

// UpdateAnnotation updates an existing annotation
func (s *ServiceImpl) UpdateAnnotation(ctx context.Context, id uint, label string, startTime, endTime float64) (*models.Annotation, error) {
	// Validate input
	if label == "" {
		return nil, fmt.Errorf("Label is required")
	}
	if startTime >= endTime {
		return nil, fmt.Errorf("Start time must be before end time")
	}

	// Get existing annotation
	annotation, err := s.repository.GetAnnotationByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update fields
	annotation.Label = label
	annotation.StartTime = startTime
	annotation.EndTime = endTime

	// Save changes
	if err := s.repository.UpdateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}

	return annotation, nil
}

// DeleteAnnotation deletes an annotation by its ID
func (s *ServiceImpl) DeleteAnnotation(ctx context.Context, id uint) error {
	return s.repository.DeleteAnnotation(ctx, id)
}

// GetAnnotationByUUID retrieves an annotation by its UUID
func (s *ServiceImpl) GetAnnotationByUUID(ctx context.Context, uuid string) (*models.Annotation, error) {
	return s.repository.GetAnnotationByUUID(ctx, uuid)
}

// CheckOverlappingAnnotation checks if there's an existing annotation that overlaps with the given time range
func (s *ServiceImpl) CheckOverlappingAnnotation(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64) (bool, error) {
	return s.repository.CheckOverlappingAnnotation(ctx, podcastIndexEpisodeID, startTime, endTime)
}

// CheckOverlappingAnnotationExcluding checks for overlaps excluding a specific annotation ID
func (s *ServiceImpl) CheckOverlappingAnnotationExcluding(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64, excludeID uint) (bool, error) {
	return s.repository.CheckOverlappingAnnotationExcluding(ctx, podcastIndexEpisodeID, startTime, endTime, excludeID)
}

// UpdateAnnotationByUUID updates an existing annotation by UUID
func (s *ServiceImpl) UpdateAnnotationByUUID(ctx context.Context, uuid, label string, startTime, endTime float64) (*models.Annotation, error) {
	// Validate input
	if label == "" {
		return nil, fmt.Errorf("Label is required")
	}
	if startTime >= endTime {
		return nil, fmt.Errorf("Start time must be before end time")
	}

	// Get existing annotation
	annotation, err := s.repository.GetAnnotationByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Check if time bounds changed
	timeBoundsChanged := annotation.StartTime != startTime || annotation.EndTime != endTime

	// Check for overlaps with OTHER annotations (exclude current one)
	if timeBoundsChanged {
		isDuplicate, err := s.repository.CheckOverlappingAnnotationExcluding(ctx, annotation.PodcastIndexEpisodeID, startTime, endTime, annotation.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check for duplicates: %w", err)
		}
		if isDuplicate {
			return nil, fmt.Errorf("updated annotation would overlap with existing annotation")
		}
	}

	// Update fields
	annotation.Label = label
	annotation.StartTime = startTime
	annotation.EndTime = endTime

	// If time bounds changed, reset clip status for re-processing
	if timeBoundsChanged {
		annotation.ClipStatus = "pending"
		annotation.ClipSize = 0
		annotation.ClipPath = "" // Clear old clip path
	}

	// Save changes
	if err := s.repository.UpdateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}

	return annotation, nil
}
