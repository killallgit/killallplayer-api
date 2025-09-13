package annotations

import (
	"context"
	"errors"
	"testing"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	args := m.Called(ctx, annotation)
	return args.Error(0)
}

func (m *MockRepository) GetAnnotationByID(ctx context.Context, id uint) (*models.Annotation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Annotation), args.Error(1)
}

func (m *MockRepository) GetAnnotationsByEpisodeID(ctx context.Context, episodeID uint) ([]models.Annotation, error) {
	args := m.Called(ctx, episodeID)
	return args.Get(0).([]models.Annotation), args.Error(1)
}

func (m *MockRepository) UpdateAnnotation(ctx context.Context, annotation *models.Annotation) error {
	args := m.Called(ctx, annotation)
	return args.Error(0)
}

func (m *MockRepository) DeleteAnnotation(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestServiceImpl_CreateAnnotation(t *testing.T) {
	ctx := context.Background()

	t.Run("generates UUID when not provided", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		annotation := &models.Annotation{
			EpisodeID: 1,
			Label:     "test_label",
			StartTime: 10.5,
			EndTime:   20.5,
		}

		mockRepo.On("CreateAnnotation", ctx, mock.AnythingOfType("*models.Annotation")).
			Run(func(args mock.Arguments) {
				ann := args.Get(1).(*models.Annotation)
				assert.NotEmpty(t, ann.UUID)
				assert.Len(t, ann.UUID, 36) // Standard UUID length
			}).
			Return(nil)

		err := service.CreateAnnotation(ctx, annotation)
		require.NoError(t, err)
		assert.NotEmpty(t, annotation.UUID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("preserves UUID when already provided", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		providedUUID := "12345678-1234-5678-1234-567812345678"
		annotation := &models.Annotation{
			UUID:      providedUUID,
			EpisodeID: 1,
			Label:     "test_label",
			StartTime: 10.5,
			EndTime:   20.5,
		}

		mockRepo.On("CreateAnnotation", ctx, annotation).Return(nil)

		err := service.CreateAnnotation(ctx, annotation)
		require.NoError(t, err)
		assert.Equal(t, providedUUID, annotation.UUID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("validates required fields", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		tests := []struct {
			name        string
			annotation  *models.Annotation
			expectedErr string
		}{
			{
				name: "missing label",
				annotation: &models.Annotation{
					EpisodeID: 1,
					StartTime: 10.5,
					EndTime:   20.5,
				},
				expectedErr: "Label is required",
			},
			{
				name: "invalid time range",
				annotation: &models.Annotation{
					EpisodeID: 1,
					Label:     "test",
					StartTime: 20.5,
					EndTime:   10.5,
				},
				expectedErr: "Start time must be before end time",
			},
			{
				name: "missing episode ID",
				annotation: &models.Annotation{
					Label:     "test",
					StartTime: 10.5,
					EndTime:   20.5,
				},
				expectedErr: "Episode ID is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := service.CreateAnnotation(ctx, tt.annotation)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}

		mockRepo.AssertNotCalled(t, "CreateAnnotation")
	})

	t.Run("handles repository error", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		annotation := &models.Annotation{
			EpisodeID: 1,
			Label:     "test_label",
			StartTime: 10.5,
			EndTime:   20.5,
		}

		expectedErr := errors.New("database error")
		mockRepo.On("CreateAnnotation", ctx, mock.AnythingOfType("*models.Annotation")).Return(expectedErr)

		err := service.CreateAnnotation(ctx, annotation)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestServiceImpl_UpdateAnnotation(t *testing.T) {
	ctx := context.Background()

	t.Run("preserves UUID during update", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		originalUUID := "12345678-1234-5678-1234-567812345678"
		existingAnnotation := &models.Annotation{
			UUID:      originalUUID,
			EpisodeID: 1,
			Label:     "original",
			StartTime: 10.5,
			EndTime:   20.5,
		}

		mockRepo.On("GetAnnotationByID", ctx, uint(1)).Return(existingAnnotation, nil)
		mockRepo.On("UpdateAnnotation", ctx, mock.AnythingOfType("*models.Annotation")).
			Run(func(args mock.Arguments) {
				ann := args.Get(1).(*models.Annotation)
				assert.Equal(t, originalUUID, ann.UUID)
				assert.Equal(t, "updated", ann.Label)
			}).
			Return(nil)

		updated, err := service.UpdateAnnotation(ctx, uint(1), "updated", 15.0, 25.0)
		require.NoError(t, err)
		assert.Equal(t, originalUUID, updated.UUID)
		assert.Equal(t, "updated", updated.Label)

		mockRepo.AssertExpectations(t)
	})

	t.Run("validates updated fields", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		tests := []struct {
			name        string
			label       string
			startTime   float64
			endTime     float64
			expectedErr string
		}{
			{
				name:        "empty label",
				label:       "",
				startTime:   10.5,
				endTime:     20.5,
				expectedErr: "Label is required",
			},
			{
				name:        "invalid time range",
				label:       "test",
				startTime:   25.0,
				endTime:     15.0,
				expectedErr: "Start time must be before end time",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := service.UpdateAnnotation(ctx, uint(1), tt.label, tt.startTime, tt.endTime)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}

		mockRepo.AssertNotCalled(t, "GetAnnotationByID")
		mockRepo.AssertNotCalled(t, "UpdateAnnotation")
	})

	t.Run("handles annotation not found", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		mockRepo.On("GetAnnotationByID", ctx, uint(999)).Return(nil, errors.New("annotation not found"))

		_, err := service.UpdateAnnotation(ctx, uint(999), "test", 10.5, 20.5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "annotation not found")

		mockRepo.AssertExpectations(t)
	})
}

func TestServiceImpl_GetAnnotationsByEpisodeID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns annotations with UUIDs", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		expectedAnnotations := []models.Annotation{
			{
				UUID:      "uuid-1",
				EpisodeID: 1,
				Label:     "intro",
				StartTime: 0,
				EndTime:   10,
			},
			{
				UUID:      "uuid-2",
				EpisodeID: 1,
				Label:     "content",
				StartTime: 10,
				EndTime:   100,
			},
			{
				UUID:      "uuid-3",
				EpisodeID: 1,
				Label:     "outro",
				StartTime: 100,
				EndTime:   110,
			},
		}

		mockRepo.On("GetAnnotationsByEpisodeID", ctx, uint(1)).Return(expectedAnnotations, nil)

		annotations, err := service.GetAnnotationsByEpisodeID(ctx, uint(1))
		require.NoError(t, err)
		assert.Len(t, annotations, 3)

		for i, ann := range annotations {
			assert.Equal(t, expectedAnnotations[i].UUID, ann.UUID)
			assert.Equal(t, expectedAnnotations[i].Label, ann.Label)
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("handles empty result", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		mockRepo.On("GetAnnotationsByEpisodeID", ctx, uint(1)).Return([]models.Annotation{}, nil)

		annotations, err := service.GetAnnotationsByEpisodeID(ctx, uint(1))
		require.NoError(t, err)
		assert.Empty(t, annotations)

		mockRepo.AssertExpectations(t)
	})
}
