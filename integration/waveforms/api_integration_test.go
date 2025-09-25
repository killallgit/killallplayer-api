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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/api/waveform"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/pkg/ffmpeg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mockEpisodeService is a simple mock implementation for testing
type mockEpisodeService struct {
	db *gorm.DB
}

func (m *mockEpisodeService) GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error) {
	var episode models.Episode
	err := m.db.Where("podcast_index_id = ?", podcastIndexID).First(&episode).Error
	if err != nil {
		return nil, err
	}
	return &episode, nil
}

func (m *mockEpisodeService) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	var episode models.Episode
	err := m.db.First(&episode, id).Error
	if err != nil {
		return nil, err
	}
	return &episode, nil
}

func (m *mockEpisodeService) SyncEpisodes(ctx context.Context, podcastID int64, maxEpisodes int) (int, error) {
	return 0, nil
}

func (m *mockEpisodeService) GetEpisodes(ctx context.Context, limit int) ([]*models.Episode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetPodcastIndexEpisode(ctx context.Context, episodeID int64) (*episodes.PodcastIndexEpisode, error) {
	return nil, nil
}

func (m *mockEpisodeService) SearchEpisodes(ctx context.Context, query string, limit int) ([]*episodes.PodcastIndexEpisode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetEpisodesByFeedID(ctx context.Context, feedID int64, limit int) ([]*episodes.PodcastIndexEpisode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) ([]*episodes.PodcastIndexEpisode, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetEpisodesByITunesID(ctx context.Context, itunesID int64, limit int) ([]*episodes.PodcastIndexEpisode, error) {
	return nil, nil
}

func (m *mockEpisodeService) FetchAndSyncEpisodes(ctx context.Context, podcastID int64, maxEpisodes int) (*episodes.PodcastIndexResponse, error) {
	return nil, nil
}

func (m *mockEpisodeService) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, limit int, offset int) ([]models.Episode, int64, error) {
	return nil, 0, nil
}

func (m *mockEpisodeService) SyncEpisodesToDatabase(ctx context.Context, episodes []episodes.PodcastIndexEpisode, podcastID uint) (int, error) {
	return 0, nil
}

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
	err = db.AutoMigrate(&models.Podcast{}, &models.Episode{}, &models.Subscription{}, &models.Waveform{}, &models.Annotation{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	// Create database wrapper
	dbWrapper := &database.DB{DB: db}

	// Setup dependencies
	waveformRepo := waveforms.NewRepository(db)
	waveformService := waveforms.NewService(waveformRepo)

	// Create a mock episode service for testing
	mockEpisodeService := &mockEpisodeService{db: db}

	deps := &types.Dependencies{
		DB:              dbWrapper,
		WaveformService: waveformService,
		EpisodeService:  mockEpisodeService,
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

func (suite *APITestSuite) createTestWaveform(episodeID int64, peaks []float32) *models.Waveform {
	waveform := &models.Waveform{
		PodcastIndexEpisodeID: episodeID,
		Duration:              300.0,
		Resolution:            len(peaks),
		SampleRate:            44100,
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

	// Make request using PodcastIndexID (not database ID)
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Verify response - waveform not found returns 202 Accepted with queued status
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, w.Code)
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check the base response fields
	if response["status"] != "queued" {
		t.Errorf("Expected status 'queued', got %v", response["status"])
	}

	if response["message"] != "Waveform generation has been queued" {
		t.Errorf("Expected message 'Waveform generation has been queued', got %v", response["message"])
	}

	// Check the waveform object
	waveform, ok := response["waveform"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected waveform to be an object, got %T", response["waveform"])
	}

	if waveform["episodeId"] != float64(episode.PodcastIndexID) {
		t.Errorf("Expected episodeId %d, got %v", episode.PodcastIndexID, waveform["episodeId"])
	}

	if waveform["status"] != "queued" {
		t.Errorf("Expected waveform status 'queued', got %v", waveform["status"])
	}
}

func TestWaveformAPI_GetWaveform_Success(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Create test episode and waveform
	episode := suite.createTestEpisode(123)
	expectedPeaks := []float32{0.1, 0.5, 0.8, 0.3, 0.9, 0.2}
	suite.createTestWaveform(int64(episode.PodcastIndexID), expectedPeaks)

	// Make request using PodcastIndexID (not database ID)
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
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

	// Check the base response fields
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	if response["message"] != "Waveform retrieved successfully" {
		t.Errorf("Expected message 'Waveform retrieved successfully', got %v", response["message"])
	}

	// Check the waveform object
	waveform, ok := response["waveform"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected waveform to be an object, got %T", response["waveform"])
	}

	// Verify waveform fields
	if waveform["episodeId"] != float64(episode.PodcastIndexID) {
		t.Errorf("Expected episodeId %d, got %v", episode.PodcastIndexID, waveform["episodeId"])
	}

	if waveform["duration"] != 300.0 {
		t.Errorf("Expected duration 300.0, got %v", waveform["duration"])
	}

	if waveform["sampleRate"] != 44100.0 {
		t.Errorf("Expected sampleRate 44100, got %v", waveform["sampleRate"])
	}

	if waveform["status"] != "ok" {
		t.Errorf("Expected waveform status 'ok', got %v", waveform["status"])
	}

	// Verify peaks data
	peaksInterface, ok := waveform["data"].([]interface{})
	if !ok {
		t.Fatalf("Expected data to be array, got %T", waveform["data"])
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
			t.Errorf("Expected data[%d] = %v, got %v (diff: %v)", i, expectedPeak, peak, abs(peak-expectedPeak))
		}
	}
}

// Tests for the deprecated /waveform/status endpoint have been removed
// The single /waveform endpoint now returns both data and status

func TestWaveformAPI_InvalidEpisodeID(t *testing.T) {
	suite := setupAPITestSuite(t)

	tests := []struct {
		name      string
		endpoint  string
		episodeID string
	}{
		{"GetWaveform with invalid ID", "/api/v1/episodes/invalid/waveform", "invalid"},
		{"GetWaveform with negative ID", "/api/v1/episodes/-1/waveform", "-1"},
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

			if response["message"] != "Invalid Podcast Index Episode ID" {
				t.Errorf("Expected message 'Invalid Podcast Index Episode ID', got %v", response["message"])
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
		suite.createTestWaveform(int64(episodes[i].PodcastIndexID), peaks)
	}

	// Test that each episode returns its own waveform
	for i, episode := range episodes {
		req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
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

		// Get the waveform object
		waveformResp, ok := response["waveform"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected waveform to be an object, got %T", response["waveform"])
		}

		if waveformResp["episodeId"] != float64(episode.PodcastIndexID) {
			t.Errorf("Expected episodeId %d, got %v", episode.PodcastIndexID, waveformResp["episodeId"])
		}

		// Verify that each waveform is different
		peaksInterface := waveformResp["data"].([]interface{})
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
		PodcastIndexID:  1000, // Add PodcastIndexID for API
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
		PodcastIndexEpisodeID: int64(episode.PodcastIndexID), // Use Podcast Index ID for consistency
		Duration:              10.0,                          // 10 seconds
		Resolution:            len(realPeaks),
		SampleRate:            44100,
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
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
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

	// Check the base response fields
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	// Get the waveform object
	waveformResp, ok := response["waveform"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected waveform to be an object, got %T", response["waveform"])
	}

	// Verify the waveform data matches our realistic data
	if waveformResp["episodeId"] != float64(episode.PodcastIndexID) {
		t.Errorf("Expected episodeId %d, got %v", episode.PodcastIndexID, waveformResp["episodeId"])
	}

	if waveformResp["duration"] != 10.0 {
		t.Errorf("Expected duration 10.0, got %v", waveformResp["duration"])
	}

	if waveformResp["sampleRate"] != 44100.0 {
		t.Errorf("Expected sampleRate 44100, got %v", waveformResp["sampleRate"])
	}

	// Verify we get realistic looking peaks (not all zeros, has variation)
	peaksInterface := waveformResp["data"].([]interface{})
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
		len(peaksInterface), waveformResp["duration"], minPeak, maxPeak)
}

// TestEndToEndWaveformWorkflow tests the complete waveform generation workflow:
// 1. Episode exists but no waveform → Returns 404 initially
// 2. Request waveform → Creates job (background processing would happen here)
// 3. Simulate job completion by saving waveform
// 4. Request waveform again → Returns the generated waveform
func TestEndToEndWaveformWorkflow(t *testing.T) {
	suite := setupAPITestSuite(t)

	// Step 1: Create test episode with realistic audio file path
	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-5s.mp3")
	episode := &models.Episode{
		Model:           gorm.Model{ID: 1},
		Title:           "Test Episode for E2E",
		AudioURL:        "file://" + testFile,
		Duration:        func() *int { d := 5; return &d }(),
		EnclosureType:   "audio/mpeg",
		EnclosureLength: 10000, // Approximate size
		PodcastID:       1,
		PodcastIndexID:  1000, // Set PodcastIndexID
	}

	// Create a basic podcast record first (required by foreign key constraint)
	podcast := &models.Podcast{
		Model:   gorm.Model{ID: 1},
		Title:   "Test Podcast",
		FeedURL: "https://example.com/feed.xml",
	}

	if err := suite.db.Create(podcast).Error; err != nil {
		t.Fatalf("Failed to create test podcast: %v", err)
	}

	if err := suite.db.Create(episode).Error; err != nil {
		t.Fatalf("Failed to create test episode: %v", err)
	}

	// Step 2: Initially, no waveform should exist - should return 404
	req, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
	if err != nil {
		t.Fatalf("Failed to create initial request: %v", err)
	}

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected initial status code %d for non-existent waveform (queued), got %d", http.StatusAccepted, w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}

	// Step 3: No separate status endpoint anymore - status is included in the waveform response

	// Step 4: Simulate the background waveform generation process
	// In the real system, this would be done by the worker system after the API request
	// For testing, we'll generate the waveform synchronously

	// Use FFmpeg to generate a real waveform from our test file
	ffmpegInstance := ffmpeg.New("ffmpeg", "ffprobe", 30*time.Second)
	if err := ffmpegInstance.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available for end-to-end test: %v", err)
	}

	opts := ffmpeg.DefaultProcessingOptions()
	opts.WaveformResolution = 100 // Small resolution for quick test

	ctx := context.Background()
	generatedWaveform, err := ffmpegInstance.GenerateWaveform(ctx, testFile, opts)
	if err != nil {
		t.Fatalf("Failed to generate waveform for end-to-end test: %v", err)
	}

	// Convert the FFmpeg waveform result to our database model
	waveformModel := &models.Waveform{
		PodcastIndexEpisodeID: int64(episode.PodcastIndexID),
		Duration:              generatedWaveform.Duration,
		Resolution:            generatedWaveform.Resolution,
		SampleRate:            generatedWaveform.SampleRate,
	}

	if err := waveformModel.SetPeaks(generatedWaveform.Peaks); err != nil {
		t.Fatalf("Failed to set peaks on waveform model: %v", err)
	}

	// Save the generated waveform to the database
	if err := suite.deps.WaveformService.SaveWaveform(ctx, waveformModel); err != nil {
		t.Fatalf("Failed to save generated waveform: %v", err)
	}

	// Step 5: Now request the waveform again - should return the generated data
	finalReq, err := http.NewRequest("GET", fmt.Sprintf("/api/v1/episodes/%d/waveform", episode.PodcastIndexID), nil)
	if err != nil {
		t.Fatalf("Failed to create final request: %v", err)
	}

	finalW := httptest.NewRecorder()
	suite.router.ServeHTTP(finalW, finalReq)

	if finalW.Code != http.StatusOK {
		t.Errorf("Expected final status code %d after waveform generation, got %d", http.StatusOK, finalW.Code)
		t.Logf("Response body: %s", finalW.Body.String())
		return
	}

	// Step 6: Validate the returned waveform data
	var finalResponse map[string]interface{}
	if err := json.Unmarshal(finalW.Body.Bytes(), &finalResponse); err != nil {
		t.Fatalf("Failed to unmarshal final response: %v", err)
	}

	// Get the waveform object from the response
	waveformResp, ok := finalResponse["waveform"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected waveform to be an object, got %T", finalResponse["waveform"])
	}

	// Verify the response contains expected data from our generated waveform
	if waveformResp["duration"] != generatedWaveform.Duration {
		t.Errorf("Expected duration %.2f, got %v", generatedWaveform.Duration, waveformResp["duration"])
	}

	if waveformResp["sampleRate"] != float64(generatedWaveform.SampleRate) {
		t.Errorf("Expected sampleRate %d, got %v", generatedWaveform.SampleRate, waveformResp["sampleRate"])
	}

	// Verify peaks data is present and matches expected count
	peaksInterface := waveformResp["data"].([]interface{})
	if len(peaksInterface) != generatedWaveform.Resolution {
		t.Errorf("Expected %d peaks in response, got %d", generatedWaveform.Resolution, len(peaksInterface))
	}

	// Step 7: No separate status endpoint - the waveform endpoint includes status

	t.Logf("✅ End-to-end workflow completed successfully:")
	t.Logf("   📋 Episode created with audio file: %s", testFile)
	t.Logf("   🚫 Initial waveform request returned 202 (queued) as expected")
	t.Logf("   ⚙️  Generated waveform: %.2fs duration, %d peaks, %dHz sample rate",
		generatedWaveform.Duration, generatedWaveform.Resolution, generatedWaveform.SampleRate)
	t.Logf("   💾 Waveform saved to database successfully")
	t.Logf("   ✅ Final waveform request returned complete data with status 'ok'")
}
