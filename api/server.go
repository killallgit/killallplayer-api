package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/cleanup"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/workers"
	"github.com/killallgit/player-api/pkg/ffmpeg"
	"github.com/spf13/viper"
)

// Server represents the HTTP server
type Server struct {
	engine             *gin.Engine
	httpServer         *http.Server
	db                 *database.DB
	episodeCache       episodes.EpisodeCache
	rateLimiters       *sync.Map
	cleanupInitialized sync.Once
	cleanupStop        chan struct{}
	workerPool         *workers.WorkerPool
	workerCancel       context.CancelFunc
	cleanupService     *cleanup.Service

	// Dependencies for handlers
	dependencies *types.Dependencies
}

// NewServer creates a new HTTP server
func NewServer(address string) *Server {
	// Create Gin engine with recovery middleware only
	engine := gin.New()
	engine.Use(gin.Recovery())

	server := &Server{
		engine:       engine,
		rateLimiters: &sync.Map{},
		cleanupStop:  make(chan struct{}),
		httpServer: &http.Server{
			Addr:           address,
			Handler:        engine,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   0, // No timeout for long-running endpoints
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MB
		},
	}

	return server
}

// SetDatabase sets the database connection
func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
	if s.dependencies == nil {
		s.dependencies = &types.Dependencies{}
	}
	s.dependencies.DB = db
}

// SetDependencies sets all handler dependencies
func (s *Server) SetDependencies(deps *types.Dependencies) {
	s.dependencies = deps
}

// Engine returns the Gin engine for testing
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Initialize sets up middleware and routes
func (s *Server) Initialize() error {
	// Setup global middleware
	s.setupMiddleware()

	// Setup routes
	if err := s.setupRoutes(); err != nil {
		return err
	}

	// Initialize and start worker pool
	if err := s.initializeWorkerPool(); err != nil {
		return err
	}

	// Initialize and start cleanup service
	s.initializeCleanupService()

	return nil
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware() {
	// Logger middleware
	s.engine.Use(gin.Logger())

	// Global CORS
	s.engine.Use(CORS())

	// Global request size limit
	s.engine.Use(RequestSizeLimit())
}

// setupRoutes delegates to the main route registration
func (s *Server) setupRoutes() error {
	return RegisterRoutes(s.engine, s.dependencies, s.rateLimiters, s.cleanupStop, &s.cleanupInitialized)
}

// initializeWorkerPool creates and starts the worker pool for background processing
func (s *Server) initializeWorkerPool() error {
	// Check if dependencies are set
	if s.dependencies == nil {
		log.Println("[WARN] Dependencies not set, skipping worker pool initialization")
		return nil
	}

	// Check if required services are available
	if s.dependencies.JobService == nil || s.dependencies.WaveformService == nil || s.dependencies.EpisodeService == nil {
		log.Println("[WARN] Required services not available, skipping worker pool initialization")
		return nil
	}

	// Get configuration values
	numWorkers := viper.GetInt("processing.workers")
	if numWorkers == 0 {
		numWorkers = 2 // Default to 2 workers
	}

	ffmpegPath := viper.GetString("processing.ffmpeg_path")
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg" // Default to PATH lookup
	}

	ffprobePath := viper.GetString("processing.ffprobe_path")
	if ffprobePath == "" {
		ffprobePath = "ffprobe" // Default to PATH lookup
	}

	ffmpegTimeout := viper.GetDuration("processing.ffmpeg_timeout")
	if ffmpegTimeout == 0 {
		ffmpegTimeout = 5 * time.Minute // Default timeout
	}

	// Create FFmpeg instance
	ffmpegInstance := ffmpeg.New(ffmpegPath, ffprobePath, ffmpegTimeout)

	// Validate FFmpeg binaries
	if err := ffmpegInstance.ValidateBinaries(); err != nil {
		log.Printf("[WARN] FFmpeg binaries not available: %v", err)
		// Don't fail initialization if FFmpeg is not available
		// The processor will handle errors gracefully
	}

	// Create enhanced waveform processor
	// Note: EpisodeService is already the correct interface type
	waveformProcessor := workers.NewEnhancedWaveformProcessor(
		s.dependencies.JobService,
		s.dependencies.WaveformService,
		s.dependencies.EpisodeService,
		s.dependencies.AudioCacheService, // May be nil if not initialized
		ffmpegInstance,
		ffmpeg.DefaultProcessingOptions(),
	)

	// Create transcription processor if transcription service is available
	var transcriptionProcessor *workers.TranscriptionProcessor
	if s.dependencies.TranscriptionService != nil {
		transcriptionProcessor = workers.NewTranscriptionProcessor(
			s.dependencies.JobService,
			s.dependencies.TranscriptionService,
			s.dependencies.EpisodeService,
			s.dependencies.AudioCacheService,
		)
	}

	// Create worker pool with proper arguments
	pollInterval := 5 * time.Second // Poll for new jobs every 5 seconds
	s.workerPool = workers.NewWorkerPool(s.dependencies.JobService, numWorkers, pollInterval)
	s.workerPool.RegisterProcessor(waveformProcessor)

	// Register transcription processor if available
	if transcriptionProcessor != nil {
		s.workerPool.RegisterProcessor(transcriptionProcessor)
	}

	// Start worker pool in background
	ctx, cancel := context.WithCancel(context.Background())
	s.workerCancel = cancel

	go func() {
		log.Printf("[INFO] Starting worker pool with %d workers", numWorkers)
		if err := s.workerPool.Start(ctx); err != nil {
			log.Printf("[ERROR] Worker pool error: %v", err)
		}
	}()

	// Store worker pool in dependencies
	s.dependencies.WorkerPool = s.workerPool

	return nil
}

// initializeCleanupService creates and starts the cleanup service for temporary files
func (s *Server) initializeCleanupService() {
	// Get configuration values
	tempDir := viper.GetString("processing.temp_dir")
	if tempDir == "" {
		tempDir = "/tmp" // Default temp directory
	}

	cleanupInterval := viper.GetDuration("processing.cleanup_interval")
	if cleanupInterval == 0 {
		cleanupInterval = 1 * time.Hour // Default to hourly cleanup
	}

	maxTempAge := viper.GetDuration("processing.max_temp_age")
	if maxTempAge == 0 {
		maxTempAge = 24 * time.Hour // Default to 24 hours
	}

	// Create and start cleanup service
	s.cleanupService = cleanup.NewService(tempDir, maxTempAge, cleanupInterval)
	s.cleanupService.Start(context.Background())

	log.Printf("[INFO] Cleanup service started for %s (interval: %v, max age: %v)", tempDir, cleanupInterval, maxTempAge)

	// Note: No token cleanup needed for Supabase auth - tokens are managed by Supabase
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the worker pool
	if s.workerPool != nil && s.workerCancel != nil {
		log.Println("[INFO] Stopping worker pool...")
		s.workerCancel()
		s.workerPool.Stop()
	}

	// Stop the cleanup service
	if s.cleanupService != nil {
		log.Println("[INFO] Stopping cleanup service...")
		s.cleanupService.Stop()
	}

	// Stop the cache cleanup goroutine if it exists
	if s.episodeCache != nil {
		s.episodeCache.Stop()
	}

	// Stop the rate limiter cleanup goroutine
	close(s.cleanupStop)

	return s.httpServer.Shutdown(ctx)
}
