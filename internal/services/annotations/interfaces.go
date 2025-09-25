package annotations

import (
	"context"

	"github.com/killallgit/player-api/internal/models"
)

// Repository defines the interface for annotation data access
type Repository interface {
	// Create operations
	CreateAnnotation(ctx context.Context, annotation *models.Annotation) error

	// Read operations
	GetAnnotationByID(ctx context.Context, id uint) (*models.Annotation, error)
	GetAnnotationByUUID(ctx context.Context, uuid string) (*models.Annotation, error)
	GetAnnotationsByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) ([]models.Annotation, error)

	// Validation operations
	CheckOverlappingAnnotation(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64) (bool, error)
	CheckOverlappingAnnotationExcluding(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64, excludeID uint) (bool, error)

	// Update operations
	UpdateAnnotation(ctx context.Context, annotation *models.Annotation) error

	// Delete operations
	DeleteAnnotation(ctx context.Context, id uint) error
}

// Service defines the interface for annotation business logic
type Service interface {
	// Create operations
	CreateAnnotation(ctx context.Context, annotation *models.Annotation) error

	// Read operations
	GetAnnotationByID(ctx context.Context, id uint) (*models.Annotation, error)
	GetAnnotationByUUID(ctx context.Context, uuid string) (*models.Annotation, error)
	GetAnnotationsByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) ([]models.Annotation, error)

	// Validation operations
	CheckOverlappingAnnotation(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64) (bool, error)
	CheckOverlappingAnnotationExcluding(ctx context.Context, podcastIndexEpisodeID int64, startTime, endTime float64, excludeID uint) (bool, error)

	// Update operations
	UpdateAnnotation(ctx context.Context, id uint, label string, startTime, endTime float64) (*models.Annotation, error)
	UpdateAnnotationByUUID(ctx context.Context, uuid, label string, startTime, endTime float64) (*models.Annotation, error)

	// Delete operations
	DeleteAnnotation(ctx context.Context, id uint) error
}
