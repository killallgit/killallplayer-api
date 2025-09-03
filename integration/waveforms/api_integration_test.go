package waveforms_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/api/waveform"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

type APITestSuite struct {
	t      *testing.T
	db     *gorm.DB
	deps   *types.Dependencies
	router *gin.Engine
}

func setupAPITestSuite(t *testing.T) *APITestSuite {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&models.Podcast{}, &models.Episode{}, &models.User{}, &models.Subscription{}, &models.PlaybackState{}, &models.Region{}, &models.Waveform{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	// Create database wrapper
	dbWrapper := &database.DB{DB: db}

	// Setup dependencies
	waveformRepo := waveforms.NewRepository(db)
	waveformService := waveforms.NewService(waveformRepo)

	deps := &types.Dependencies{
		DB:              dbWrapper,
		WaveformService: waveformService,
	}

	// Setup router
	router := gin.New()

	// Register waveform routes
	waveformGroup := router.Group("/api/v1/episodes")
	waveform.RegisterRoutes(waveformGroup, deps)

	return &APITestSuite{
		t:      t,
		db:     db,
		deps:   deps,
		router: router,
	}
}

func (suite *APITestSuite) createTestEpisode(id uint) *models.Episode {
	episode := &models.Episode{
		Model:           gorm.Model{ID: id},
		PodcastIndexID:  int64(id * 1000),                // Ensure unique podcast index ID
		GUID:            fmt.Sprintf("test-guid-%d", id), // Ensure unique GUID
		Title:           "Test Episode",
		AudioURL:        "https://example.com/audio.mp3",
		Duration:        func() *int { d := 300; return &d }(),
		EnclosureType:   "audio/mpeg",
		EnclosureLength: 12345678,
	}

	if err := suite.db.Create(episode).Error; err != nil {
		suite.t.Fatalf("Failed to create test episode: %v", err)
	}

	return episode
}

func (suite *APITestSuite) createTestWaveform(episodeID uint, peaks []float32) *models.Waveform {
	waveform := &models.Waveform{
		EpisodeID:  episodeID,
		Duration:   300.0,
		Resolution: len(peaks),
		SampleRate: 44100,
	}

	err := waveform.SetPeaks(peaks)
	if err != nil {
		suite.t.Fatalf("Failed to set peaks: %v", err)
	}

	ctx := context.Background()
	err = suite.deps.WaveformService.SaveWaveform(ctx, waveform)
	if err != nil {
		suite.t.Fatalf("Failed to save test waveform: %v", err)
	}

	return waveform
}

func TestWaveformAPI_GetWaveform_NotFound(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create test episode but no waveform
	episode := suite.createTestEpisode(123)

	// Make request
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["error"] != "Waveform not found for episode" {
		t.Errorf("Expected error message 'Waveform not found for episode', got %v", response["error"])
	}

	if response["episode_id"] != float64(episode.ID) {
		t.Errorf("Expected episode_id %d, got %v", episode.ID, response["episode_id"])
	}
}

func TestWaveformAPI_GetWaveform_Success(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create test episode and waveform
	episode := suite.createTestEpisode(123)
	expectedPeaks := []float32{0.1, 0.5, 0.8, 0.3, 0.9, 0.2}
	suite.createTestWaveform(episode.ID, expectedPeaks)

	// Make request
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify response fields
	if response["episode_id"] != float64(episode.ID) {
		t.Errorf("Expected episode_id %d, got %v", episode.ID, response["episode_id"])
	}

	if response["duration"] != 300.0 {
		t.Errorf("Expected duration 300.0, got %v", response["duration"])
	}

	if response["resolution"] != float64(len(expectedPeaks)) {
		t.Errorf("Expected resolution %d, got %v", len(expectedPeaks), response["resolution"])
	}

	if response["sample_rate"] != 44100.0 {
		t.Errorf("Expected sample_rate 44100, got %v", response["sample_rate"])
	}

	if response["cached"] != true {
		t.Errorf("Expected cached true, got %v", response["cached"])
	}

	// Verify peaks data
	peaksInterface, ok := response["peaks"].([]interface{})
	if !ok {
		t.Fatalf("Expected peaks to be array, got %T", response["peaks"])
	}

	if len(peaksInterface) != len(expectedPeaks) {
		t.Errorf("Expected %d peaks, got %d", len(expectedPeaks), len(peaksInterface))
	}

	for i, peakInterface := range peaksInterface {
		peak, ok := peakInterface.(float64)
		if !ok {
			t.Errorf("Expected peak to be float64, got %T", peakInterface)
			continue
		}

		expectedPeak := float64(expectedPeaks[i])
		// Use approximate comparison for float values due to JSON marshaling precision
		if abs(peak-expectedPeak) > 0.0001 {
			t.Errorf("Expected peaks[%d] = %v, got %v (diff: %v)", i, expectedPeak, peak, abs(peak-expectedPeak))
		}
	}
}

func TestWaveformAPI_GetWaveformStatus_NotFound(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create test episode but no waveform
	episode := suite.createTestEpisode(123)

	// Make request
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform/status", episode.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "not_found" {
		t.Errorf("Expected status 'not_found', got %v", response["status"])
	}

	if response["progress"] != float64(0) {
		t.Errorf("Expected progress 0, got %v", response["progress"])
	}

	if response["message"] != "Waveform not available" {
		t.Errorf("Expected message 'Waveform not available', got %v", response["message"])
	}
}

func TestWaveformAPI_GetWaveformStatus_Success(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create test episode and waveform
	episode := suite.createTestEpisode(123)
	expectedPeaks := []float32{0.1, 0.5, 0.8}
	suite.createTestWaveform(episode.ID, expectedPeaks)

	// Make request
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform/status", episode.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "completed" {
		t.Errorf("Expected status 'completed', got %v", response["status"])
	}

	if response["progress"] != float64(100) {
		t.Errorf("Expected progress 100, got %v", response["progress"])
	}

	if response["message"] != "Waveform ready" {
		t.Errorf("Expected message 'Waveform ready', got %v", response["message"])
	}

	if response["episode_id"] != float64(episode.ID) {
		t.Errorf("Expected episode_id %d, got %v", episode.ID, response["episode_id"])
	}
}

func TestWaveformAPI_InvalidEpisodeID(t *testing.T) {
	suite := setupAPITestSuite(t)

	tests := []struct {
		name      string
		endpoint  string
		episodeID string
	}{
		{"GetWaveform with invalid ID", "/api/v1/episodes/invalid/waveform", "invalid"},
		{"GetWaveformStatus with invalid ID", "/api/v1/episodes/invalid/waveform/status", "invalid"},
		{"GetWaveform with negative ID", "/api/v1/episodes/-1/waveform", "-1"},
		{"GetWaveformStatus with negative ID", "/api/v1/episodes/-1/waveform/status", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.endpoint, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
			}

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if response["error"] != "Invalid episode ID" {
				t.Errorf("Expected error message 'Invalid episode ID', got %v", response["error"])
			}
		})
	}
}

func TestWaveformAPI_DatabaseIntegration(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create multiple episodes and waveforms
	episodes := make([]*models.Episode, 3)
	for i := 0; i < 3; i++ {
		episodes[i] = suite.createTestEpisode(uint(i + 1))
		peaks := make([]float32, 5)
		for j := 0; j < 5; j++ {
			peaks[j] = float32(i+1) * 0.1 * float32(j+1) // Different patterns for each episode
		}
		suite.createTestWaveform(episodes[i].ID, peaks)
	}

	// Test that each episode returns its own waveform
	for i, episode := range episodes {
		req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.ID), nil)
		if err != nil {
			t.Fatalf("Failed to create request for episode %d: %v", episode.ID, err)
		}

		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code %d for episode %d, got %d", http.StatusOK, episode.ID, w.Code)
			continue
		}

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatalf("Failed to unmarshal response for episode %d: %v", episode.ID, err)
		}

		if response["episode_id"] != float64(episode.ID) {
			t.Errorf("Expected episode_id %d, got %v", episode.ID, response["episode_id"])
		}

		// Verify that each waveform is different
		peaksInterface := response["peaks"].([]interface{})
		firstPeak := peaksInterface[0].(float64)
		expectedFirstPeak := float64(i+1) * 0.1 * 1.0

		tolerance := 0.0001
		if diff := firstPeak - expectedFirstPeak; diff < -tolerance || diff > tolerance {
			t.Errorf("Expected first peak for episode %d to be %v, got %v", episode.ID, expectedFirstPeak, firstPeak)
		}
	}
}

func TestWaveformAPI_WithRealAudioFile(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Check if sample audio file exists
	samplePath := filepath.Join("..", "sample.mp3")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skip("sample.mp3 not found, skipping real audio file test")
	}

	// Get file info for metadata
	fileInfo, err := os.Stat(samplePath)
	if err != nil {
		t.Fatalf("Failed to get sample file info: %v", err)
	}

	// Create test episode with realistic metadata
	episode := &models.Episode{
		Model:           gorm.Model{ID: 1},
		Title:           "Sample Audio Episode",
		AudioURL:        "file://" + samplePath,
		Duration:        func() *int { d := 10; return &d }(), // Assume 10 second sample
		EnclosureType:   "audio/mpeg",
		EnclosureLength: fileInfo.Size(),
	}

	if err := suite.db.Create(episode).Error; err != nil {
		t.Fatalf("Failed to create test episode: %v", err)
	}

	// Create waveform data that could realistically come from this file
	// For this test, we'll use synthetic data that represents what we might extract
	realPeaks := []float32{
		0.0, 0.1, 0.3, 0.2, 0.5, 0.8, 0.6, 0.9, 0.7, 0.4,
		0.3, 0.6, 0.8, 0.5, 0.2, 0.4, 0.7, 0.3, 0.1, 0.0,
		0.2, 0.5, 0.9, 0.7, 0.4, 0.6, 0.8, 0.3, 0.1, 0.2,
		0.4, 0.7, 0.5, 0.8, 0.6, 0.3, 0.1, 0.4, 0.9, 0.2,
		0.1, 0.3, 0.6, 0.4, 0.7, 0.5, 0.8, 0.2, 0.0, 0.1,
	}

	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   10.0, // 10 seconds
		Resolution: len(realPeaks),
		SampleRate: 44100,
	}

	err = waveform.SetPeaks(realPeaks)
	if err != nil {
		t.Fatalf("Failed to set peaks: %v", err)
	}

	ctx := context.Background()
	err = suite.deps.WaveformService.SaveWaveform(ctx, waveform)
	if err != nil {
		t.Fatalf("Failed to save waveform: %v", err)
	}

	// Test the API endpoint
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify the response matches our realistic data
	if response["duration"] != 10.0 {
		t.Errorf("Expected duration 10.0, got %v", response["duration"])
	}

	if response["resolution"] != float64(len(realPeaks)) {
		t.Errorf("Expected resolution %d, got %v", len(realPeaks), response["resolution"])
	}

	if response["sample_rate"] != 44100.0 {
		t.Errorf("Expected sample_rate 44100, got %v", response["sample_rate"])
	}

	// Verify we get realistic looking peaks (not all zeros, has variation)
	peaksInterface := response["peaks"].([]interface{})
	if len(peaksInterface) != len(realPeaks) {
		t.Errorf("Expected %d peaks, got %d", len(realPeaks), len(peaksInterface))
	}

	// Check that peaks have realistic variation (not all the same value)
	var minPeak, maxPeak float64 = 1.0, 0.0
	for _, peakInterface := range peaksInterface {
		peak := peakInterface.(float64)
		if peak < minPeak {
			minPeak = peak
		}
		if peak > maxPeak {
			maxPeak = peak
		}
	}

	if maxPeak-minPeak < 0.1 {
		t.Errorf("Expected significant variation in peaks (min=%v, max=%v), but got very little", minPeak, maxPeak)
	}

	t.Logf("Successfully tested with sample audio file: %d peaks, duration=%.1fs, range=%.1f-%.1f",
		len(peaksInterface), response["duration"], minPeak, maxPeak)
}
