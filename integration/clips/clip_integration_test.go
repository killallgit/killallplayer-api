package clips_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/clips"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/workers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ClipTestSuite holds all dependencies for clip integration tests
type ClipTestSuite struct {
	t            *testing.T
	db           *gorm.DB
	jobService   jobs.Service
	clipService  clips.Service
	workerPool   *workers.WorkerPool
	tempDir      string
	clipsDir     string
	testAudioURL string
	cleanupFuncs []func()
}

// setupClipTestSuite initializes an isolated test environment
func setupClipTestSuite(t *testing.T) *ClipTestSuite {
	// Create temporary directories for test artifacts
	tempDir, err := os.MkdirTemp("", "clip-integration-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	clipsDir := filepath.Join(tempDir, "clips")
	err = os.MkdirAll(clipsDir, 0755)
	require.NoError(t, err, "Failed to create clips directory")

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to test database")

	// Run migrations
	err = db.AutoMigrate(&models.Job{}, &models.Clip{})
	require.NoError(t, err, "Failed to migrate test database")

	// Create database wrapper (for potential future use)
	_ = &database.DB{DB: db}

	// Initialize job service
	jobRepo := jobs.NewRepository(db)
	jobService := jobs.NewService(jobRepo)

	// Initialize clip components
	extractor, err := clips.NewFFmpegExtractor(tempDir, 15.0)
	require.NoError(t, err, "Failed to create FFmpeg extractor")

	storage, err := clips.NewLocalClipStorage(clipsDir)
	require.NoError(t, err, "Failed to create clip storage")

	clipService := clips.NewService(db, storage, extractor, jobService)

	// Create worker pool with short poll interval for tests
	workerPool := workers.NewWorkerPool(jobService, 2, 100*time.Millisecond)

	// Register clip processor
	clipProcessor := workers.NewClipExtractionProcessor(jobService, db, extractor, storage)
	workerPool.RegisterProcessor(clipProcessor)

	// Start worker pool
	ctx := context.Background()
	err = workerPool.Start(ctx)
	require.NoError(t, err, "Failed to start worker pool")

	suite := &ClipTestSuite{
		t:            t,
		db:           db,
		jobService:   jobService,
		clipService:  clipService,
		workerPool:   workerPool,
		tempDir:      tempDir,
		clipsDir:     clipsDir,
		cleanupFuncs: make([]func(), 0),
	}

	// Add cleanup for worker pool
	suite.cleanupFuncs = append(suite.cleanupFuncs, func() {
		workerPool.Stop()
	})

	// Add cleanup for temp directory
	suite.cleanupFuncs = append(suite.cleanupFuncs, func() {
		os.RemoveAll(tempDir)
	})

	return suite
}

// cleanup runs all cleanup functions
func (suite *ClipTestSuite) cleanup() {
	for _, fn := range suite.cleanupFuncs {
		fn()
	}
}

// startTestAudioServer starts an HTTP server serving test audio file
func (suite *ClipTestSuite) startTestAudioServer() {
	// Find test audio file
	testAudioPath := filepath.Join(".", "test-audio.mp3")
	if _, err := os.Stat(testAudioPath); os.IsNotExist(err) {
		// Try alternative path
		testAudioPath = filepath.Join("..", "sample.mp3")
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, testAudioPath)
	}))

	suite.testAudioURL = server.URL
	suite.cleanupFuncs = append(suite.cleanupFuncs, server.Close)
}

// waitForClipStatus polls clip status until it matches expected or times out
func (suite *ClipTestSuite) waitForClipStatus(clipUUID string, expectedStatus string, timeout time.Duration) *models.Clip {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			suite.t.Fatalf("Timeout waiting for clip %s to reach status %s", clipUUID, expectedStatus)
			return nil
		case <-ticker.C:
			var clip models.Clip
			err := suite.db.Where("uuid = ?", clipUUID).First(&clip).Error
			if err != nil {
				continue
			}
			if clip.Status == expectedStatus {
				return &clip
			}
		}
	}
}

// TestClipCreationEnqueuesJob tests that creating a clip enqueues a job
func TestClipCreationEnqueuesJob(t *testing.T) {
	suite := setupClipTestSuite(t)
	defer suite.cleanup()

	// Create a clip
	params := clips.CreateClipParams{
		SourceEpisodeURL:  "https://example.com/episode.mp3",
		OriginalStartTime: 10.0,
		OriginalEndTime:   25.0,
		Label:             "test",
	}

	clip, err := suite.clipService.CreateClip(context.Background(), params)
	require.NoError(t, err, "Failed to create clip")
	assert.NotEmpty(t, clip.UUID, "Clip should have UUID")
	assert.Equal(t, "queued", clip.Status, "Clip should be queued")

	// Verify job was created
	var job models.Job
	err = suite.db.Where("type = ?", models.JobTypeClipExtraction).First(&job).Error
	require.NoError(t, err, "Job should be created")
	assert.Equal(t, models.JobStatusPending, job.Status, "Job should be pending")

	// Verify job payload contains clip UUID
	clipUUIDFromPayload, ok := job.Payload["clip_uuid"].(string)
	require.True(t, ok, "Job payload should contain clip_uuid")
	assert.Equal(t, clip.UUID, clipUUIDFromPayload, "Job payload should reference correct clip")
}

// TestEndToEndClipProcessing tests full clip extraction workflow
func TestEndToEndClipProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow clip extraction integration test in short mode")
	}
	suite := setupClipTestSuite(t)
	defer suite.cleanup()

	// Start test audio server
	suite.startTestAudioServer()

	// Create a clip
	params := clips.CreateClipParams{
		SourceEpisodeURL:  suite.testAudioURL,
		OriginalStartTime: 0.5,
		OriginalEndTime:   3.5,
		Label:             "speech",
	}

	clip, err := suite.clipService.CreateClip(context.Background(), params)
	require.NoError(t, err, "Failed to create clip")

	// Wait for clip to be processed (workers poll every 100ms)
	processedClip := suite.waitForClipStatus(clip.UUID, "ready", 30*time.Second)
	require.NotNil(t, processedClip, "Clip should be processed")

	// Verify clip properties
	assert.Equal(t, "ready", processedClip.Status, "Clip should be ready")
	assert.Equal(t, 15.0, processedClip.ClipDuration, "Clip should be 15 seconds (padded)")
	assert.Greater(t, processedClip.ClipSizeBytes, int64(0), "Clip should have size")
	assert.NotEmpty(t, processedClip.ClipFilename, "Clip should have filename")

	// Verify physical file exists
	expectedPath := filepath.Join(suite.clipsDir, "speech", processedClip.ClipFilename)
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "Clip file should exist at %s", expectedPath)

	// Verify job was marked complete
	var job models.Job
	err = suite.db.Where("type = ? AND status = ?", models.JobTypeClipExtraction, models.JobStatusCompleted).First(&job).Error
	assert.NoError(t, err, "Job should be completed")
	assert.Equal(t, 100, job.Progress, "Job progress should be 100%")
}

// TestClipProcessingWithInvalidURL tests error handling
func TestClipProcessingWithInvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow clip extraction integration test in short mode")
	}
	suite := setupClipTestSuite(t)
	defer suite.cleanup()

	// Create clip with invalid URL
	params := clips.CreateClipParams{
		SourceEpisodeURL:  "https://invalid-domain-that-does-not-exist.example/audio.mp3",
		OriginalStartTime: 0.0,
		OriginalEndTime:   10.0,
		Label:             "test",
	}

	clip, err := suite.clipService.CreateClip(context.Background(), params)
	require.NoError(t, err, "Creating clip should succeed even with invalid URL")

	// Wait for clip to fail
	failedClip := suite.waitForClipStatus(clip.UUID, "failed", 15*time.Second)
	require.NotNil(t, failedClip, "Clip should fail")

	assert.Equal(t, "failed", failedClip.Status, "Clip should be failed")
	assert.NotEmpty(t, failedClip.ErrorMessage, "Clip should have error message")

	// Verify job failed with correct error type
	var job models.Job
	err = suite.db.Where("type = ?", models.JobTypeClipExtraction).Order("id DESC").First(&job).Error
	require.NoError(t, err, "Job should exist")

	// Job should have failed permanently after retries
	assert.True(t, job.Status == models.JobStatusFailed || job.Status == models.JobStatusPermanentlyFailed,
		"Job should be in failed state")
	assert.NotEmpty(t, job.Error, "Job should have error message")
}

// TestConcurrentClipProcessing tests multiple clips processed concurrently
func TestConcurrentClipProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow clip extraction integration test in short mode")
	}
	suite := setupClipTestSuite(t)
	defer suite.cleanup()
	suite.startTestAudioServer()

	// Create multiple clips
	numClips := 5
	clipUUIDs := make([]string, numClips)

	for i := 0; i < numClips; i++ {
		params := clips.CreateClipParams{
			SourceEpisodeURL:  suite.testAudioURL,
			OriginalStartTime: float64(i),
			OriginalEndTime:   float64(i + 3),
			Label:             fmt.Sprintf("label-%d", i),
		}

		clip, err := suite.clipService.CreateClip(context.Background(), params)
		require.NoError(t, err, "Failed to create clip %d", i)
		clipUUIDs[i] = clip.UUID
	}

	// Wait for all clips to be processed
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)

	for _, uuid := range clipUUIDs {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("Timeout waiting for clips to process")
		}

		processedClip := suite.waitForClipStatus(uuid, "ready", remaining)
		require.NotNil(t, processedClip, "Clip %s should be processed", uuid)
		assert.Equal(t, "ready", processedClip.Status, "Clip should be ready")
	}

	// Verify all jobs completed
	var completedJobs int64
	suite.db.Model(&models.Job{}).
		Where("type = ? AND status = ?", models.JobTypeClipExtraction, models.JobStatusCompleted).
		Count(&completedJobs)

	assert.Equal(t, int64(numClips), completedJobs, "All jobs should complete")
}

// TestClipStorageOrganization tests clips are stored in correct directories
func TestClipStorageOrganization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow clip extraction integration test in short mode")
	}
	suite := setupClipTestSuite(t)
	defer suite.cleanup()
	suite.startTestAudioServer()

	labels := []string{"advertisement", "music", "speech"}
	clipsByLabel := make(map[string]string)

	// Create clips with different labels
	for _, label := range labels {
		params := clips.CreateClipParams{
			SourceEpisodeURL:  suite.testAudioURL,
			OriginalStartTime: 0.0,
			OriginalEndTime:   5.0,
			Label:             label,
		}

		clip, err := suite.clipService.CreateClip(context.Background(), params)
		require.NoError(t, err, "Failed to create clip with label %s", label)
		clipsByLabel[label] = clip.UUID
	}

	// Wait for all to process
	for label, uuid := range clipsByLabel {
		processedClip := suite.waitForClipStatus(uuid, "ready", 30*time.Second)
		require.NotNil(t, processedClip, "Clip with label %s should process", label)

		// Verify file is in correct subdirectory
		expectedDir := filepath.Join(suite.clipsDir, label)
		expectedPath := filepath.Join(expectedDir, processedClip.ClipFilename)

		_, err := os.Stat(expectedPath)
		assert.NoError(t, err, "Clip should exist in %s directory", label)
	}
}

// TestListClipsWithFilters tests filtering clips by label
func TestListClipsWithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow clip extraction integration test in short mode")
	}
	suite := setupClipTestSuite(t)
	defer suite.cleanup()
	suite.startTestAudioServer()

	// Create clips with different labels
	labels := map[string]int{
		"advertisement": 3,
		"speech":        2,
		"music":         1,
	}

	for label, count := range labels {
		for i := 0; i < count; i++ {
			params := clips.CreateClipParams{
				SourceEpisodeURL:  suite.testAudioURL,
				OriginalStartTime: float64(i),
				OriginalEndTime:   float64(i + 3),
				Label:             label,
			}
			_, err := suite.clipService.CreateClip(context.Background(), params)
			require.NoError(t, err, "Failed to create clip")
		}
	}

	// Wait a bit for processing
	time.Sleep(2 * time.Second)

	// Test filtering by label
	filters := clips.ListClipsFilters{Label: "advertisement"}
	adClips, err := suite.clipService.ListClips(context.Background(), filters)
	require.NoError(t, err, "Failed to list clips")
	assert.Len(t, adClips, 3, "Should have 3 advertisement clips")

	// Test no filter
	allClips, err := suite.clipService.ListClips(context.Background(), clips.ListClipsFilters{})
	require.NoError(t, err, "Failed to list all clips")
	assert.Len(t, allClips, 6, "Should have 6 total clips")
}
