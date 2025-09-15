package api

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/killallgit/player-api/api/annotations"
	"github.com/killallgit/player-api/api/categories"
	"github.com/killallgit/player-api/api/episodes"
	"github.com/killallgit/player-api/api/health"
	"github.com/killallgit/player-api/api/podcasts"
	"github.com/killallgit/player-api/api/random"
	"github.com/killallgit/player-api/api/search"
	transcriptionAPI "github.com/killallgit/player-api/api/transcription"
	"github.com/killallgit/player-api/api/trending"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/api/version"
	"github.com/killallgit/player-api/api/waveform"
	_ "github.com/killallgit/player-api/docs"
	annotationsService "github.com/killallgit/player-api/internal/services/annotations"
	episodesService "github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/itunes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/killallgit/player-api/internal/services/transcription"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/viper"
)

// Rate limiting constants to avoid duplication
const (
	// Search endpoints have stricter limits due to external API calls
	SearchRateLimit      = 5
	SearchRateLimitBurst = 10

	// General API endpoints have moderate limits
	GeneralRateLimit      = 10
	GeneralRateLimitBurst = 20
)

// RegisterRoutes registers all API routes
func RegisterRoutes(engine *gin.Engine, deps *types.Dependencies, rateLimiters *sync.Map, cleanupStop chan struct{}, cleanupInitialized *sync.Once) error {
	// Register public routes (no rate limiting)
	health.RegisterRoutes(engine, deps)
	version.RegisterRoutes(engine, deps)

	// Register Swagger documentation route
	engine.GET("/docs", func(c *gin.Context) {
		c.Redirect(301, "/docs/index.html")
	})
	docsGroup := engine.Group("/docs")
	docsGroup.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Setup 404 handler
	engine.NoRoute(NotFoundHandler())

	// API v1 routes
	v1 := engine.Group("/api/v1")

	// Load config for API routes
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Initialize services if not already set
	if deps == nil {
		deps = &types.Dependencies{}
	}

	// Initialize podcast client if not set
	if deps.PodcastClient == nil {
		// Use Viper directly for Podcast Index credentials since unmarshal isn't working correctly
		apiKey := viper.GetString("podcast_index.api_key")
		apiSecret := viper.GetString("podcast_index.api_secret")
		baseURL := viper.GetString("podcast_index.api_url")

		if apiKey == "" {
			apiKey = cfg.PodcastIndex.APIKey
		}
		if apiSecret == "" {
			apiSecret = cfg.PodcastIndex.APISecret
		}
		if baseURL == "" {
			baseURL = cfg.PodcastIndex.BaseURL
		}

		deps.PodcastClient = podcastindex.NewClient(podcastindex.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			BaseURL:   baseURL,
		})
	}

	// Register search routes with dedicated rate limiting
	searchGroup := v1.Group("/search")
	searchGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, SearchRateLimit, SearchRateLimitBurst))
	search.RegisterRoutes(searchGroup, deps)

	// Register trending routes with general rate limiting
	trendingGroup := v1.Group("/trending")
	trendingGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	trending.RegisterRoutes(trendingGroup, deps)

	// Register categories routes with general rate limiting
	categoriesGroup := v1.Group("/categories")
	categoriesGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	categories.RegisterRoutes(categoriesGroup, deps)

	// Register random routes with general rate limiting
	randomGroup := v1.Group("/random")
	randomGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	random.RegisterRoutes(randomGroup, deps)

	// Initialize all services if database is available
	if deps.DB != nil && deps.DB.DB != nil {
		initializeAllServices(deps, cfg)

		// Register all episode-related routes with general rate limiting
		// All episode features share the same rate limits since they operate on the same resource
		episodeGroup := v1.Group("/episodes")
		episodeGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))

		// Register all episode-related routes under the same group
		episodes.RegisterRoutes(episodeGroup, deps)
		waveform.RegisterRoutes(episodeGroup, deps)         // Waveform generation may be CPU intensive
		transcriptionAPI.RegisterRoutes(episodeGroup, deps) // Transcription generation may be CPU intensive
		annotations.RegisterRoutes(episodeGroup, deps)

		// Register podcast routes with rate limiting
		podcastGroup := v1.Group("/podcasts")
		// Create middleware for rate limiting
		episodesMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst)
		podcasts.RegisterRoutes(podcastGroup, deps, episodesMiddleware)

	}

	return nil
}

// initializeAllServices creates and configures all services in one place
func initializeAllServices(deps *types.Dependencies, cfg *config.Config) {
	// Initialize episode service if not set
	if deps.EpisodeService == nil || deps.EpisodeTransformer == nil {
		initializeEpisodeService(deps, cfg)
	}

	// Initialize waveform service if not set
	if deps.WaveformService == nil {
		initializeWaveformService(deps)
	}

	// Initialize transcription service if not set
	if deps.TranscriptionService == nil {
		initializeTranscriptionService(deps)
	}

	// Initialize annotation service if not set
	if deps.AnnotationService == nil {
		initializeAnnotationService(deps)
	}

	// Initialize job service if not set
	if deps.JobService == nil {
		initializeJobService(deps)
	}

	// Initialize iTunes client if not set
	if deps.ITunesClient == nil {
		initializeITunesClient(deps)
	}
}

// initializeEpisodeService creates and configures the episode service
func initializeEpisodeService(deps *types.Dependencies, cfg *config.Config) {
	// Create dependencies
	podcastClient := deps.PodcastClient.(*podcastindex.Client)
	episodeFetcher := episodesService.NewPodcastIndexAdapter(podcastClient)
	episodeRepo := episodesService.NewRepository(deps.DB.DB)
	episodeCache := episodesService.NewCache(time.Hour)

	// Get configuration
	maxConcurrentSync := config.GetInt("episodes.max_concurrent_sync")
	if maxConcurrentSync <= 0 {
		maxConcurrentSync = 5
	}
	syncTimeout := config.GetDuration("episodes.sync_timeout")
	if syncTimeout <= 0 {
		syncTimeout = 30 * time.Second
	}

	// Create service
	deps.EpisodeService = episodesService.NewService(
		episodeFetcher,
		episodeRepo,
		episodeCache,
		episodesService.WithMaxConcurrentSync(maxConcurrentSync),
		episodesService.WithSyncTimeout(syncTimeout),
	)

	deps.EpisodeTransformer = episodesService.NewTransformer()
}

// initializeWaveformService creates and configures the waveform service
func initializeWaveformService(deps *types.Dependencies) {
	// Create dependencies
	waveformRepo := waveforms.NewRepository(deps.DB.DB)

	// Create service
	deps.WaveformService = waveforms.NewService(waveformRepo)
}

// initializeTranscriptionService creates and configures the transcription service
func initializeTranscriptionService(deps *types.Dependencies) {
	// Create dependencies
	transcriptionRepo := transcription.NewRepository(deps.DB.DB)

	// Create service
	deps.TranscriptionService = transcription.NewService(transcriptionRepo)
}

// initializeAnnotationService creates and configures the annotation service
func initializeAnnotationService(deps *types.Dependencies) {
	// Create dependencies
	annotationRepo := annotationsService.NewRepository(deps.DB.DB)

	// Create service
	deps.AnnotationService = annotationsService.NewService(annotationRepo)
}

// initializeJobService creates and configures the job service
func initializeJobService(deps *types.Dependencies) {
	// Create dependencies
	jobRepo := jobs.NewRepository(deps.DB.DB)

	// Create service
	deps.JobService = jobs.NewService(jobRepo)
}

// initializeITunesClient creates and configures the iTunes client
func initializeITunesClient(deps *types.Dependencies) {
	// Create iTunes client with configuration
	itunesConfig := itunes.Config{
		RequestsPerMinute: 250, // Conservative rate limit
		BurstSize:         5,
		Timeout:           10 * time.Second,
		MaxRetries:        3,
		RetryBackoff:      time.Second,
	}

	deps.ITunesClient = itunes.NewClient(itunesConfig)
	log.Printf("[INFO] iTunes client initialized with rate limit: %d req/min", itunesConfig.RequestsPerMinute)
}

// NotFoundHandler handles 404 errors
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "The requested endpoint was not found",
			"path":    c.Request.URL.Path,
		})
	}
}
