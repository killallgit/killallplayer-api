package annotations_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/annotations"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
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

	// Setup dependencies
	deps := &types.Dependencies{
		DB: &database.DB{DB: db},
	}

	// Setup router
	router := gin.New()
	router.POST("/episodes/:id/annotations", annotations.CreateAnnotation(deps))
	router.GET("/episodes/:id/annotations", annotations.GetAnnotations(deps))
	router.PUT("/annotations/:id", annotations.UpdateAnnotation(deps))
	router.DELETE("/annotations/:id", annotations.DeleteAnnotation(deps))

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
			episodeID: strconv.Itoa(int(episodeID)),
			payload: map[string]interface{}{
				"label":      "Introduction",
				"start_time": 0.0,
				"end_time":   30.5,
			},
			expectedStatus: http.StatusCreated,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var annotation models.Annotation
				err := json.Unmarshal(w.Body.Bytes(), &annotation)
				assert.NoError(t, err)
				assert.Equal(t, "Introduction", annotation.Label)
				assert.Equal(t, 0.0, annotation.StartTime)
				assert.Equal(t, 30.5, annotation.EndTime)
				assert.Equal(t, episodeID, annotation.EpisodeID)
			},
		},
		{
			name:      "missing label",
			episodeID: strconv.Itoa(int(episodeID)),
			payload: map[string]interface{}{
				"start_time": 0.0,
				"end_time":   30.5,
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
			episodeID: strconv.Itoa(int(episodeID)),
			payload: map[string]interface{}{
				"label":      "Invalid Range",
				"start_time": 30.0,
				"end_time":   10.0,
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
			episodeID:      strconv.Itoa(int(episodeID)),
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
				assert.Equal(t, 0.0, firstAnnotation["start_time"])
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
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				annotationsData := response["annotations"].([]interface{})
				assert.Len(t, annotationsData, 0)
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
			annotationID: strconv.Itoa(int(annotation.ID)),
			payload: map[string]interface{}{
				"label":      "Updated Label",
				"start_time": 5.0,
				"end_time":   45.0,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var updatedAnnotation models.Annotation
				err := json.Unmarshal(w.Body.Bytes(), &updatedAnnotation)
				assert.NoError(t, err)
				assert.Equal(t, "Updated Label", updatedAnnotation.Label)
				assert.Equal(t, 5.0, updatedAnnotation.StartTime)
				assert.Equal(t, 45.0, updatedAnnotation.EndTime)
			},
		},
		{
			name:         "missing label",
			annotationID: strconv.Itoa(int(annotation.ID)),
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
			annotationID: strconv.Itoa(int(annotation.ID)),
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
			annotationID:   strconv.Itoa(int(annotation.ID)),
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
			annotationID:   "invalid",
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Invalid annotation ID")
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
