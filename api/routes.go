package api

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	authAPI "github.com/killallgit/player-api/api/auth"
	"github.com/killallgit/player-api/api/categories"
	"github.com/killallgit/player-api/api/clips"
	"github.com/killallgit/player-api/api/episodes"
	"github.com/killallgit/player-api/api/health"
	"github.com/killallgit/player-api/api/middleware"
	"github.com/killallgit/player-api/api/podcasts"
	"github.com/killallgit/player-api/api/random"
	"github.com/killallgit/player-api/api/search"
	transcriptionAPI "github.com/killallgit/player-api/api/transcription"
	"github.com/killallgit/player-api/api/trending"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/api/version"
	"github.com/killallgit/player-api/api/waveform"
	_ "github.com/killallgit/player-api/docs"
	authService "github.com/killallgit/player-api/internal/services/auth"
	"github.com/killallgit/player-api/internal/services/cache"
	clipsService "github.com/killallgit/player-api/internal/services/clips"
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

	// Initialize auth service if not set
	var authHandler *authAPI.Handler
	if deps.AuthService == nil {
		jwksURL := viper.GetString("supabase.jwks_url")

		if jwksURL != "" {
			authSvc, err := authService.NewService(jwksURL)
			if err != nil {
				log.Printf("Failed to initialize auth service: %v", err)
			} else {
				// Configure development authentication if enabled
				devAuthEnabled := viper.GetBool("dev.auth_enabled")
				devAuthToken := viper.GetString("dev.auth_token")
				if devAuthEnabled && devAuthToken != "" {
					authSvc.SetDevAuth(true, devAuthToken)
					log.Printf("WARNING: Development authentication enabled - DO NOT USE IN PRODUCTION")
				}

				deps.AuthService = authSvc
				log.Println("Auth service initialized successfully with JWKS")
			}
		} else {
			log.Println("Supabase JWKS URL not configured - skipping auth service initialization")
		}
	}

	// Setup auth handler and routes if service is available
	if deps.AuthService != nil {
		authHandler = authAPI.NewHandler(deps.AuthService)

		// Configure dev auth in handler if enabled
		devAuthEnabled := viper.GetBool("dev.auth_enabled")
		devAuthToken := viper.GetString("dev.auth_token")
		if devAuthEnabled && devAuthToken != "" {
			authHandler.SetDevAuth(true, devAuthToken)
		}

		// Apply auth middleware to ALL v1 routes
		v1.Use(authHandler.AuthMiddleware())

		// Register /me endpoint at v1 level (protected by middleware)
		v1.GET("/me", authHandler.Me)
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

	// Initialize cache if enabled
	var cacheMiddleware gin.HandlerFunc
	if viper.GetBool("cache.enabled") {
		maxSizeMB := viper.GetInt64("cache.max_size_mb")
		if maxSizeMB == 0 {
			maxSizeMB = 100 // Default 100MB
		}

		memCache := cache.NewMemoryCache(maxSizeMB)

		// Build TTL configuration map
		ttlByPath := make(map[string]time.Duration)
		ttlByPath["/api/v1/search"] = time.Duration(viper.GetInt("cache.ttl_search")) * time.Minute
		ttlByPath["/api/v1/trending"] = time.Duration(viper.GetInt("cache.ttl_trending")) * time.Minute
		ttlByPath["/api/v1/podcasts"] = time.Duration(viper.GetInt("cache.ttl_podcast")) * time.Minute
		ttlByPath["/api/v1/episodes"] = time.Duration(viper.GetInt("cache.ttl_episode")) * time.Minute
		ttlByPath["/api/v1/categories"] = time.Duration(viper.GetInt("cache.ttl_categories")) * time.Minute

		// Create cache configuration
		cacheConfig := middleware.CacheConfig{
			Cache:      memCache,
			DefaultTTL: viper.GetDuration("cache.default_ttl"),
			TTLByPath:  ttlByPath,
			Enabled:    true,
		}

		cacheMiddleware = middleware.CacheMiddleware(cacheConfig)
	}

	// Register search routes with dedicated rate limiting and caching
	searchGroup := v1.Group("/search")
	searchGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, SearchRateLimit, SearchRateLimitBurst))
	if cacheMiddleware != nil {
		searchGroup.Use(cacheMiddleware)
	}
	search.RegisterRoutes(searchGroup, deps)

	// Register trending routes with general rate limiting and caching
	trendingGroup := v1.Group("/trending")
	trendingGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	if cacheMiddleware != nil {
		trendingGroup.Use(cacheMiddleware)
	}
	trending.RegisterRoutes(trendingGroup, deps)

	// Register categories routes with general rate limiting and caching
	categoriesGroup := v1.Group("/categories")
	categoriesGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	if cacheMiddleware != nil {
		categoriesGroup.Use(cacheMiddleware)
	}
	categories.RegisterRoutes(categoriesGroup, deps)

	// Register random routes with general rate limiting
	randomGroup := v1.Group("/random")
	randomGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	random.RegisterRoutes(randomGroup, deps)

	// Initialize all services if database is available
	if deps.DB != nil && deps.DB.DB != nil {
		initializeAllServices(deps, cfg)

		// Register all episode-related routes with general rate limiting and caching
		// All episode features share the same rate limits since they operate on the same resource
		episodeGroup := v1.Group("/episodes")
		episodeGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
		if cacheMiddleware != nil {
			// Apply cache only to GET endpoints (other middleware will filter out non-GET)
			episodeGroup.Use(cacheMiddleware)
		}

		// Register all episode-related routes under the same group
		episodes.RegisterRoutes(episodeGroup, deps)
		waveform.RegisterRoutes(episodeGroup, deps)         // Waveform generation may be CPU intensive
		transcriptionAPI.RegisterRoutes(episodeGroup, deps) // Transcription generation may be CPU intensive

		// Register podcast routes with rate limiting and caching
		podcastGroup := v1.Group("/podcasts")
		// Create middleware for rate limiting
		episodesMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst)
		if cacheMiddleware != nil {
			podcastGroup.Use(cacheMiddleware)
		}
		podcasts.RegisterRoutes(podcastGroup, deps, episodesMiddleware)

		// Register clips routes with rate limiting
		clipsGroup := v1.Group("/clips")
		clipsGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
		clips.RegisterRoutes(clipsGroup, deps)

	}

	return nil
}

// initializeAllServices creates and configures all services in one place
func initializeAllServices(deps *types.Dependencies, cfg *config.Config) {
	// Initialize job service FIRST - other services depend on it
	if deps.JobService == nil {
		initializeJobService(deps)
	}

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

	// Initialize clip service if not set (depends on JobService)
	if deps.ClipService == nil {
		initializeClipService(deps)
	}

	// Initialize iTunes client if not set
	if deps.ITunesClient == nil {
		initializeITunesClient(deps)
	}
}

// initializeEpisodeService creates and configures the episode service
func initializeEpisodeService(deps *types.Dependencies, _ *config.Config) {
	// Check if PodcastClient is available
	if deps.PodcastClient == nil {
		log.Printf("[ERROR] PodcastClient is nil - episode service will not be able to fetch from API")
		// Still create the service but with nil fetcher - it will only use database
		episodeRepo := episodesService.NewRepository(deps.DB.DB)
		episodeCache := episodesService.NewCache(time.Hour)

		deps.EpisodeService = episodesService.NewService(
			nil, // No fetcher available
			episodeRepo,
			episodeCache,
		)
		deps.EpisodeTransformer = episodesService.NewTransformer()
		return
	}

	// Create dependencies
	podcastClient, ok := deps.PodcastClient.(*podcastindex.Client)
	if !ok {
		log.Printf("[ERROR] PodcastClient is not of expected type *podcastindex.Client")
		return
	}

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

// initializeClipService creates and configures the clip service
func initializeClipService(deps *types.Dependencies) {
	// Get configuration for clips storage
	clipsBasePath := viper.GetString("clips.storage_path")
	if clipsBasePath == "" {
		clipsBasePath = "./clips" // Default to local clips directory
	}

	tempDir := viper.GetString("clips.temp_dir")
	if tempDir == "" {
		tempDir = "/tmp/clips" // Default temp directory
	}

	targetDuration := viper.GetFloat64("clips.target_duration")
	if targetDuration <= 0 {
		targetDuration = 15.0 // Default to 15 seconds
	}

	// Create FFmpeg extractor
	extractor, err := clipsService.NewFFmpegExtractor(tempDir, targetDuration)
	if err != nil {
		log.Printf("[ERROR] Failed to create FFmpeg extractor: %v", err)
		return
	}

	// Create local storage
	storage, err := clipsService.NewLocalClipStorage(clipsBasePath)
	if err != nil {
		log.Printf("[ERROR] Failed to create clip storage: %v", err)
		return
	}

	// Create service (requires JobService for background processing)
	if deps.JobService == nil {
		log.Printf("[WARN] JobService not initialized, clip service will not be able to process clips")
		return
	}
	deps.ClipService = clipsService.NewService(deps.DB.DB, storage, extractor, deps.JobService)
	log.Printf("[INFO] Clip service initialized with storage at %s", clipsBasePath)
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
