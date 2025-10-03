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
	"github.com/killallgit/player-api/internal/services/autolabel"
	"github.com/killallgit/player-api/internal/services/cleanup"
	"github.com/killallgit/player-api/internal/services/clips"
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

func NewServer(address string) *Server {
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
			WriteTimeout:   0,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}

	return server
}

func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
	if s.dependencies == nil {
		s.dependencies = &types.Dependencies{}
	}
	s.dependencies.DB = db
}

func (s *Server) SetDependencies(deps *types.Dependencies) {
	s.dependencies = deps
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) Initialize() error {
	s.setupMiddleware()

	if err := s.setupRoutes(); err != nil {
		return err
	}

	if err := s.initializeWorkerPool(); err != nil {
		return err
	}

	s.initializeCleanupService()

	return nil
}

func (s *Server) setupMiddleware() {
	s.engine.Use(gin.Logger())
	s.engine.Use(CORS())
	s.engine.Use(RequestSizeLimit())
}

func (s *Server) setupRoutes() error {
	return RegisterRoutes(s.engine, s.dependencies, s.rateLimiters, s.cleanupStop, &s.cleanupInitialized)
}

func (s *Server) initializeWorkerPool() error {
	if s.dependencies == nil {
		log.Println("[WARN] Dependencies not set, skipping worker pool initialization")
		return nil
	}

	if s.dependencies.JobService == nil || s.dependencies.WaveformService == nil || s.dependencies.EpisodeService == nil {
		log.Println("[WARN] Required services not available, skipping worker pool initialization")
		return nil
	}

	numWorkers := viper.GetInt("processing.workers")
	ffmpegPath := viper.GetString("ffmpeg.path")
	ffprobePath := viper.GetString("ffmpeg.ffprobe_path")
	ffmpegTimeout := viper.GetDuration("ffmpeg.timeout")

	ffmpegInstance := ffmpeg.New(ffmpegPath, ffprobePath, ffmpegTimeout)

	if err := ffmpegInstance.ValidateBinaries(); err != nil {
		log.Printf("[WARN] FFmpeg binaries not available: %v", err)
		// Don't fail initialization if FFmpeg is not available
		// The processor will handle errors gracefully
	}

	waveformProcessor := workers.NewEnhancedWaveformProcessor(
		s.dependencies.JobService,
		s.dependencies.WaveformService,
		s.dependencies.EpisodeService,
		s.dependencies.AudioCacheService,
		ffmpegInstance,
		ffmpeg.DefaultProcessingOptions(),
	)

	var transcriptionProcessor *workers.TranscriptionProcessor
	if s.dependencies.TranscriptionService != nil {
		transcriptionProcessor = workers.NewTranscriptionProcessor(
			s.dependencies.JobService,
			s.dependencies.TranscriptionService,
			s.dependencies.EpisodeService,
			s.dependencies.AudioCacheService,
		)
	}

	pollInterval := 5 * time.Second
	s.workerPool = workers.NewWorkerPool(s.dependencies.JobService, numWorkers, pollInterval)
	s.workerPool.RegisterProcessor(waveformProcessor)

	if transcriptionProcessor != nil {
		s.workerPool.RegisterProcessor(transcriptionProcessor)
	}

	var clipProcessor *workers.ClipExtractionProcessor
	if s.dependencies.ClipService != nil {
		clipsBasePath := viper.GetString("clips.storage_path")
		tempDir := viper.GetString("temp_dir")
		targetDuration := viper.GetFloat64("clips.target_duration")

		extractor, err := clips.NewFFmpegExtractor(tempDir, targetDuration)
		if err == nil {
			storage, err := clips.NewLocalClipStorage(clipsBasePath)
			if err == nil {
				clipProcessor = workers.NewClipExtractionProcessor(
					s.dependencies.JobService,
					s.dependencies.DB.DB,
					extractor,
					storage,
				)
				s.workerPool.RegisterProcessor(clipProcessor)
				log.Printf("[INFO] Registered clip extraction processor")
			} else {
				log.Printf("[WARN] Failed to create clip storage for processor: %v", err)
			}
		} else {
			log.Printf("[WARN] Failed to create FFmpeg extractor for clip processor: %v", err)
		}
	}

	if s.dependencies.ClipService != nil {
		clipsBasePath := viper.GetString("clips.storage_path")

		peakDetector := autolabel.NewFFmpegPeakDetector("")
		autolabelSvc := autolabel.NewService(s.dependencies.DB.DB, peakDetector)

		autolabelProcessor := workers.NewAutoLabelProcessor(
			s.dependencies.JobService,
			s.dependencies.DB.DB,
			autolabelSvc,
			clipsBasePath,
		)
		s.workerPool.RegisterProcessor(autolabelProcessor)
		log.Printf("[INFO] Registered autolabel processor")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.workerCancel = cancel

	go func() {
		log.Printf("[INFO] Starting worker pool with %d workers", numWorkers)
		if err := s.workerPool.Start(ctx); err != nil {
			log.Printf("[ERROR] Worker pool error: %v", err)
		}
	}()

	s.dependencies.WorkerPool = s.workerPool

	return nil
}

func (s *Server) initializeCleanupService() {
	tempDir := viper.GetString("temp_dir")
	cleanupInterval := viper.GetDuration("cleanup.interval")
	maxTempAge := viper.GetDuration("cleanup.max_age")

	s.cleanupService = cleanup.NewService(tempDir, maxTempAge, cleanupInterval)
	s.cleanupService.Start(context.Background())

	log.Printf("[INFO] Cleanup service started for %s (interval: %v, max age: %v)", tempDir, cleanupInterval, maxTempAge)
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.workerPool != nil && s.workerCancel != nil {
		log.Println("[INFO] Stopping worker pool...")
		s.workerCancel()
		s.workerPool.Stop()
	}

	if s.cleanupService != nil {
		log.Println("[INFO] Stopping cleanup service...")
		s.cleanupService.Stop()
	}

	if s.episodeCache != nil {
		s.episodeCache.Stop()
	}

	close(s.cleanupStop)

	return s.httpServer.Shutdown(ctx)
}
