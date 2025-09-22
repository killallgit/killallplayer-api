package annotations_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/annotations"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
	annotationsService "github.com/killallgit/player-api/internal/services/annotations"
	episodesService "github.com/killallgit/player-api/internal/services/episodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type AnnotationTestSuite struct {
	t      *testing.T
	db     *gorm.DB
	deps   *types.Dependencies
	router *gin.Engine
}

func setupAnnotationTestSuite(t *testing.T) *AnnotationTestSuite {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate the models
	err = db.AutoMigrate(&models.Episode{}, &models.Annotation{})
	require.NoError(t, err, "Failed to migrate test database")

	// Setup dependencies with services
	deps := &types.Dependencies{
		DB: &database.DB{DB: db},
	}

	// Initialize episode service (required for fetching by PodcastIndexID)
	episodeRepo := episodesService.NewRepository(db)
	episodeCache := episodesService.NewCache(time.Hour)
	deps.EpisodeService = episodesService.NewService(
		nil, // No fetcher needed for tests
		episodeRepo,
		episodeCache,
	)

	// Initialize annotation service
	annotationRepo := annotationsService.NewRepository(db)
	deps.AnnotationService = annotationsService.NewService(annotationRepo)

	// Setup router
	router := gin.New()
	router.POST("/episodes/:id/annotations", annotations.CreateAnnotation(deps))
	router.GET("/episodes/:id/annotations", annotations.GetAnnotations(deps))
	router.PUT("/annotations/:uuid", annotations.UpdateAnnotationByUUID(deps))
	router.DELETE("/annotations/:uuid", annotations.DeleteAnnotation(deps))
	router.GET("/annotations/:uuid", annotations.GetAnnotationByUUID(deps))

	return &AnnotationTestSuite{
		t:      t,
		db:     db,
		deps:   deps,
		router: router,
	}
}

func (suite *AnnotationTestSuite) createTestEpisode() uint {
	episode := models.Episode{
		PodcastID:      1,
		PodcastIndexID: 12345,
		Title:          "Test Episode",
		Description:    "Test Description",
		AudioURL:       "https://example.com/audio.mp3",
	}

	result := suite.db.Create(&episode)
	require.NoError(suite.t, result.Error, "Failed to create test episode")

	return episode.ID
}

func TestCreateAnnotation(t *testing.T) {
	suite := setupAnnotationTestSuite(t)
	episodeID := suite.createTestEpisode()

	tests := []struct {
		name           string
		episodeID      string
		payload        map[string]interface{}
		expectedStatus int
		validateFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "successful creation",
			episodeID: "12345", // Use PodcastIndexID
			payload: map[string]interface{}{
				"label":      "Introduction",
				"start_time": 0.0,
				"end_time":   30.5,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var annotation types.Annotation
				err := json.Unmarshal(w.Body.Bytes(), &annotation)
				assert.NoError(t, err)
				assert.Equal(t, "Introduction", annotation.Label)
				assert.Equal(t, 0.0, annotation.StartTime)
				assert.Equal(t, 30.5, annotation.EndTime)
				assert.Equal(t, int64(episodeID), annotation.EpisodeID)
				assert.NotEmpty(t, annotation.ID) // UUID should be set
			},
		},
		{
			name:      "missing label",
			episodeID: "12345", // Use PodcastIndexID
			payload: map[string]interface{}{
				"start_time": 50.0,
				"end_time":   80.5,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Label is required")
			},
		},
		{
			name:      "invalid time range",
			episodeID: "12345", // Use PodcastIndexID
			payload: map[string]interface{}{
				"label":      "Invalid Range",
				"start_time": 100.0,
				"end_time":   90.0,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Start time must be before end time")
			},
		},
		{
			name:           "invalid episode ID",
			episodeID:      "invalid",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Invalid id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/episodes/%s/annotations", tt.episodeID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

func TestGetAnnotations(t *testing.T) {
	suite := setupAnnotationTestSuite(t)
	episodeID := suite.createTestEpisode()

	// Create test annotations
	annotations := []models.Annotation{
		{EpisodeID: episodeID, Label: "Introduction", StartTime: 0.0, EndTime: 30.0},
		{EpisodeID: episodeID, Label: "Main Content", StartTime: 30.0, EndTime: 1800.0},
		{EpisodeID: episodeID, Label: "Conclusion", StartTime: 1800.0, EndTime: 1850.0},
	}

	for _, annotation := range annotations {
		result := suite.db.Create(&annotation)
		require.NoError(t, result.Error, "Failed to create test annotation")
	}

	tests := []struct {
		name           string
		episodeID      string
		expectedStatus int
		validateFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "successful retrieval",
			episodeID:      "12345", // Use PodcastIndexID
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				annotationsData := response["annotations"].([]interface{})
				assert.Len(t, annotationsData, 3)

				// Check that annotations are ordered by start time
				firstAnnotation := annotationsData[0].(map[string]interface{})
				assert.Equal(t, "Introduction", firstAnnotation["label"])
				assert.Equal(t, 0.0, firstAnnotation["startTime"])
			},
		},
		{
			name:           "invalid episode ID",
			episodeID:      "invalid",
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Invalid id")
			},
		},
		{
			name:           "non-existent episode",
			episodeID:      "99999",
			expectedStatus: http.StatusNotFound, // Changed to NotFound since episode doesn't exist
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Episode not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/episodes/%s/annotations", tt.episodeID), nil)

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

func TestUpdateAnnotation(t *testing.T) {
	suite := setupAnnotationTestSuite(t)
	episodeID := suite.createTestEpisode()

	// Create test annotation
	annotation := models.Annotation{
		EpisodeID: episodeID,
		Label:     "Original Label",
		StartTime: 0.0,
		EndTime:   30.0,
	}
	result := suite.db.Create(&annotation)
	require.NoError(t, result.Error, "Failed to create test annotation")

	tests := []struct {
		name           string
		annotationID   string
		payload        map[string]interface{}
		expectedStatus int
		validateFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "successful update",
			annotationID: annotation.UUID,
			payload: map[string]interface{}{
				"label":      "Updated Label",
				"start_time": 5.0,
				"end_time":   45.0,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response types.SingleAnnotationResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				updatedAnnotation := response.Annotation
				assert.Equal(t, "Updated Label", updatedAnnotation.Label)
				assert.Equal(t, 5.0, updatedAnnotation.StartTime)
				assert.Equal(t, 45.0, updatedAnnotation.EndTime)
			},
		},
		{
			name:         "missing label",
			annotationID: annotation.UUID,
			payload: map[string]interface{}{
				"start_time": 5.0,
				"end_time":   45.0,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Label is required")
			},
		},
		{
			name:         "invalid time range",
			annotationID: annotation.UUID,
			payload: map[string]interface{}{
				"label":      "Updated Label",
				"start_time": 45.0,
				"end_time":   5.0,
			},
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Start time must be before end time")
			},
		},
		{
			name:           "non-existent annotation",
			annotationID:   "99999",
			payload:        map[string]interface{}{"label": "Test", "start_time": 0.0, "end_time": 30.0},
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Annotation not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/annotations/%s", tt.annotationID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

func TestDeleteAnnotation(t *testing.T) {
	suite := setupAnnotationTestSuite(t)
	episodeID := suite.createTestEpisode()

	// Create test annotation
	annotation := models.Annotation{
		EpisodeID: episodeID,
		Label:     "To Be Deleted",
		StartTime: 0.0,
		EndTime:   30.0,
	}
	result := suite.db.Create(&annotation)
	require.NoError(t, result.Error, "Failed to create test annotation")

	tests := []struct {
		name           string
		annotationID   string
		expectedStatus int
		validateFunc   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "successful deletion",
			annotationID:   annotation.UUID,
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Annotation deleted successfully", response["message"])

				// Verify annotation is deleted from database
				var count int64
				suite.db.Model(&models.Annotation{}).Where("id = ?", annotation.ID).Count(&count)
				assert.Equal(t, int64(0), count)
			},
		},
		{
			name:           "invalid annotation ID",
			annotationID:   "invalid-uuid",
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Annotation not found")
			},
		},
		{
			name:           "non-existent annotation",
			annotationID:   "99999",
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Annotation not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/annotations/%s", tt.annotationID), nil)

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

// TestAnnotationUUID tests UUID generation and stability for annotations
func TestAnnotationUUID(t *testing.T) {
	suite := setupAnnotationTestSuite(t)
	episodeID := suite.createTestEpisode()

	t.Run("UUID is generated on creation", func(t *testing.T) {
		payload := map[string]interface{}{
			"label":      "Test Annotation",
			"start_time": 10.0,
			"end_time":   20.0,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/episodes/12345/annotations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var annotation types.Annotation
		err := json.Unmarshal(w.Body.Bytes(), &annotation)
		require.NoError(t, err)

		// Verify UUID was generated
		assert.NotEmpty(t, annotation.ID)
		assert.Len(t, annotation.ID, 36) // Standard UUID format with hyphens

		// Verify UUID follows UUID v4 format (xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx)
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, annotation.ID)
	})

	t.Run("UUID remains stable after update", func(t *testing.T) {
		// First create an annotation
		annotation := models.Annotation{
			EpisodeID: episodeID,
			Label:     "Original",
			StartTime: 100.0,
			EndTime:   110.0,
		}
		result := suite.db.Create(&annotation)
		require.NoError(t, result.Error)
		originalUUID := annotation.UUID

		// Update the annotation
		updatePayload := map[string]interface{}{
			"label":      "Updated",
			"start_time": 105.0,
			"end_time":   115.0,
		}

		body, _ := json.Marshal(updatePayload)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/annotations/%s", annotation.UUID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response types.SingleAnnotationResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		updatedAnnotation := response.Annotation

		// UUID should remain the same
		assert.Equal(t, originalUUID, updatedAnnotation.ID)
		// But other fields should be updated
		assert.Equal(t, "Updated", updatedAnnotation.Label)
		assert.Equal(t, 105.0, updatedAnnotation.StartTime)
		assert.Equal(t, 115.0, updatedAnnotation.EndTime)
	})

	t.Run("Each annotation gets unique UUID", func(t *testing.T) {
		uuids := make(map[string]bool)

		// Create multiple annotations
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{
				"label":      fmt.Sprintf("Annotation %d", i),
				"start_time": float64(150 + i*10),
				"end_time":   float64(150 + (i+1)*10),
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/episodes/12345/annotations", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var annotation types.Annotation
			err := json.Unmarshal(w.Body.Bytes(), &annotation)
			require.NoError(t, err)

			// Check UUID is unique
			assert.NotEmpty(t, annotation.ID)
			_, exists := uuids[annotation.ID]
			assert.False(t, exists, "UUID should be unique")
			uuids[annotation.ID] = true
		}

		// Verify we created 5 unique UUIDs
		assert.Len(t, uuids, 5)
	})

	t.Run("UUID is included in GET response", func(t *testing.T) {
		// Create an annotation
		annotation := models.Annotation{
			EpisodeID: episodeID,
			Label:     "Test",
			StartTime: 200.0,
			EndTime:   210.0,
		}
		result := suite.db.Create(&annotation)
		require.NoError(t, result.Error)
		require.NotEmpty(t, annotation.UUID)

		// Get annotations for the episode
		req := httptest.NewRequest(http.MethodGet, "/episodes/12345/annotations", nil)
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		annotationsData := response["annotations"].([]interface{})
		require.Greater(t, len(annotationsData), 0)

		// Find our annotation in the response by comparing labels
		var foundAnnotation map[string]interface{}
		for _, ann := range annotationsData {
			annMap := ann.(map[string]interface{})
			if annMap["label"] == "Test" {
				foundAnnotation = annMap
				break
			}
		}

		require.NotNil(t, foundAnnotation, "Should find our test annotation")
		// Check that UUID is present and matches
		assert.Equal(t, annotation.UUID, foundAnnotation["id"])
	})
}
