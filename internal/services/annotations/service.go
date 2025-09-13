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
	if annotation.EpisodeID == 0 {
		return fmt.Errorf("Episode ID is required")
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

// GetAnnotationsByEpisodeID retrieves all annotations for a specific episode
func (s *ServiceImpl) GetAnnotationsByEpisodeID(ctx context.Context, episodeID uint) ([]models.Annotation, error) {
	return s.repository.GetAnnotationsByEpisodeID(ctx, episodeID)
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
