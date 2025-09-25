package annotations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api"
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

type IntegrationTestSuite struct {
	t      *testing.T
	db     *gorm.DB
	deps   *types.Dependencies
	router *gin.Engine
}

func setupIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.Podcast{},
		&models.Episode{},
		&models.Subscription{},
		&models.Waveform{},
		&models.Job{},
		&models.Annotation{},
	)
	require.NoError(t, err, "Failed to migrate test database")

	// Setup dependencies
	deps := &types.Dependencies{
		DB: &database.DB{DB: db},
	}

	// Initialize required services for annotation tests
	// Episode service (needed by annotation handlers)
	episodeRepo := episodesService.NewRepository(deps.DB.DB) // Use the same DB instance
	episodeCache := episodesService.NewCache(time.Hour)
	deps.EpisodeService = episodesService.NewService(
		nil, // No fetcher needed for integration tests
		episodeRepo,
		episodeCache,
	)
	deps.EpisodeTransformer = episodesService.NewTransformer()

	// Annotation service
	annotationRepo := annotationsService.NewRepository(deps.DB.DB) // Use the same DB instance
	deps.AnnotationService = annotationsService.NewService(annotationRepo)

	// Setup router with all routes
	router := gin.New()
	router.Use(gin.Recovery())

	// Create a minimal rate limiter setup for testing
	rateLimiters := &sync.Map{}
	cleanupStop := make(chan struct{})
	cleanupInitialized := &sync.Once{}

	// Register routes like the real application
	err = api.RegisterRoutes(router, deps, rateLimiters, cleanupStop, cleanupInitialized)
	require.NoError(t, err, "Failed to register routes")

	return &IntegrationTestSuite{
		t:      t,
		db:     db,
		deps:   deps,
		router: router,
	}
}

func (suite *IntegrationTestSuite) createTestEpisode() *models.Episode {
	episode := &models.Episode{
		PodcastID:      1,
		PodcastIndexID: 12345,
		Title:          "Integration Test Episode",
		Description:    "Test Description for Integration",
		AudioURL:       "https://example.com/test-audio.mp3",
		GUID:           "test-guid-123",
		PublishedAt:    time.Now(),
	}

	result := suite.db.Create(episode)
	require.NoError(suite.t, result.Error, "Failed to create test episode")

	// Verify the episode was actually created and can be found
	var foundEpisode models.Episode
	findResult := suite.db.Where("podcast_index_id = ?", 12345).First(&foundEpisode)
	require.NoError(suite.t, findResult.Error, "Failed to find created episode")

	suite.t.Logf("Created episode with ID=%d, PodcastIndexID=%d", episode.ID, episode.PodcastIndexID)

	// Test the service method directly to see if it works
	testEpisode, err := suite.deps.EpisodeService.GetEpisodeByPodcastIndexID(context.Background(), 12345)
	if err != nil {
		suite.t.Logf("Service failed to find episode: %v", err)
	} else {
		suite.t.Logf("Service found episode with ID=%d, PodcastIndexID=%d", testEpisode.ID, testEpisode.PodcastIndexID)
	}

	return episode
}

func TestFullAnnotationWorkflow(t *testing.T) {
	suite := setupIntegrationTestSuite(t)
	episode := suite.createTestEpisode()

	// Step 1: Create an annotation
	createPayload := map[string]interface{}{
		"label":      "Integration Test Annotation",
		"start_time": 10.5,
		"end_time":   120.0,
	}

	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "Failed to create annotation")

	var createdAnnotation types.Annotation
	err := json.Unmarshal(w.Body.Bytes(), &createdAnnotation)
	require.NoError(t, err, "Failed to parse created annotation")
	require.NotEmpty(t, createdAnnotation.ID, "Created annotation should have an ID")

	suite.t.Logf("Created annotation with UUID: %s", createdAnnotation.ID)

	// Step 2: Get annotations for the episode
	req = httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
		nil)

	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to get annotations")

	var getResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err, "Failed to parse get response")

	annotations := getResponse["annotations"].([]interface{})
	require.Len(t, annotations, 1, "Should have exactly one annotation")

	firstAnnotation := annotations[0].(map[string]interface{})
	assert.Equal(t, "Integration Test Annotation", firstAnnotation["label"])
	assert.Equal(t, 10.5, firstAnnotation["startTime"])
	assert.Equal(t, 120.0, firstAnnotation["endTime"])

	// Step 3: Update the annotation
	updatePayload := map[string]interface{}{
		"label":      "Updated Integration Test Annotation",
		"start_time": 15.0,
		"end_time":   150.0,
	}

	body, _ = json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/api/v1/episodes/annotations/%s", createdAnnotation.ID),
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to update annotation")

	var updateResponse types.SingleAnnotationResponse
	err = json.Unmarshal(w.Body.Bytes(), &updateResponse)
	require.NoError(t, err, "Failed to parse updated annotation response")
	assert.Equal(t, "Updated Integration Test Annotation", updateResponse.Annotation.Label)
	assert.Equal(t, 15.0, updateResponse.Annotation.StartTime)
	assert.Equal(t, 150.0, updateResponse.Annotation.EndTime)

	// Step 4: Verify update by getting annotations again
	req = httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
		nil)

	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to get annotations after update")

	err = json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err, "Failed to parse get response after update")

	annotations = getResponse["annotations"].([]interface{})
	require.Len(t, annotations, 1, "Should still have exactly one annotation")

	updatedFirstAnnotation := annotations[0].(map[string]interface{})
	assert.Equal(t, "Updated Integration Test Annotation", updatedFirstAnnotation["label"])
	assert.Equal(t, 15.0, updatedFirstAnnotation["startTime"])
	assert.Equal(t, 150.0, updatedFirstAnnotation["endTime"])

	// Step 5: Delete the annotation
	req = httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/episodes/annotations/%s", createdAnnotation.ID),
		nil)

	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to delete annotation")

	var deleteResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &deleteResponse)
	require.NoError(t, err, "Failed to parse delete response")
	assert.Equal(t, "Annotation deleted successfully", deleteResponse["message"])

	// Step 6: Verify deletion by getting annotations again
	req = httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
		nil)

	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to get annotations after deletion")

	err = json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err, "Failed to parse get response after deletion")

	annotations = getResponse["annotations"].([]interface{})
	assert.Len(t, annotations, 0, "Should have no annotations after deletion")
}

func TestMultipleAnnotationsOrderedByTime(t *testing.T) {
	suite := setupIntegrationTestSuite(t)
	episode := suite.createTestEpisode()

	// Create multiple annotations in random order
	annotations := []map[string]interface{}{
		{"label": "Third Section", "start_time": 200.0, "end_time": 300.0},
		{"label": "First Section", "start_time": 0.0, "end_time": 100.0},
		{"label": "Second Section", "start_time": 100.0, "end_time": 200.0},
	}

	// Create all annotations
	for _, annotation := range annotations {
		body, _ := json.Marshal(annotation)
		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
			bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code, "Failed to create annotation")
	}

	// Get all annotations
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
		nil)

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to get annotations")

	var getResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &getResponse)
	require.NoError(t, err, "Failed to parse get response")

	retrievedAnnotations := getResponse["annotations"].([]interface{})
	require.Len(t, retrievedAnnotations, 3, "Should have exactly three annotations")

	// Verify they are ordered by start_time
	expectedLabels := []string{"First Section", "Second Section", "Third Section"}
	expectedStartTimes := []float64{0.0, 100.0, 200.0}

	for i, annotation := range retrievedAnnotations {
		annotationMap := annotation.(map[string]interface{})
		assert.Equal(t, expectedLabels[i], annotationMap["label"])
		assert.Equal(t, expectedStartTimes[i], annotationMap["startTime"])
	}
}

func TestAnnotationValidationConstraints(t *testing.T) {
	suite := setupIntegrationTestSuite(t)
	episode := suite.createTestEpisode()

	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty label",
			payload:        map[string]interface{}{"label": "", "start_time": 0.0, "end_time": 30.0},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Label is required",
		},
		{
			name:           "start time equals end time",
			payload:        map[string]interface{}{"label": "Equal Times", "start_time": 50.0, "end_time": 50.0},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Start time must be before end time",
		},
		{
			name:           "negative start time",
			payload:        map[string]interface{}{"label": "Negative Start", "start_time": -5.0, "end_time": 30.0},
			expectedStatus: http.StatusCreated, // This should be allowed for ML training
		},
		{
			name: "very long label",
			payload: map[string]interface{}{
				"label":      strings.Repeat("A", 1000), // Use regular characters instead of null bytes
				"start_time": 100.0,                     // Use different time range to avoid overlap
				"end_time":   130.0,
			},
			expectedStatus: http.StatusCreated, // Long labels should be allowed for detailed ML annotations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/v1/episodes/%d/annotations", episode.PodcastIndexID),
				bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				// Print the response to debug
				t.Logf("Expected status %d but got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}
		})
	}
}
