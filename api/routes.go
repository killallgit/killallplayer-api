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
	"github.com/killallgit/player-api/internal/services/audiocache"
	authService "github.com/killallgit/player-api/internal/services/auth"
	"github.com/killallgit/player-api/internal/services/cache"
	clipsService "github.com/killallgit/player-api/internal/services/clips"
	episodeanalysis "github.com/killallgit/player-api/internal/services/episode_analysis"
	episodesService "github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/itunes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	podcastsService "github.com/killallgit/player-api/internal/services/podcasts"
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

func RegisterRoutes(engine *gin.Engine, deps *types.Dependencies, rateLimiters *sync.Map, cleanupStop chan struct{}, cleanupInitialized *sync.Once) error {
	health.RegisterRoutes(engine, deps)
	version.RegisterRoutes(engine, deps)

	engine.GET("/docs", func(c *gin.Context) {
		c.Redirect(301, "/docs/index.html")
	})
	docsGroup := engine.Group("/docs")
	docsGroup.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	engine.NoRoute(NotFoundHandler())

	v1 := engine.Group("/api/v1")

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if deps == nil {
		deps = &types.Dependencies{}
	}

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

	if deps.AuthService != nil {
		authHandler = authAPI.NewHandler(deps.AuthService)

		v1.Use(authHandler.AuthMiddleware())
		v1.GET("/me", authHandler.Me)
	}

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

	var cacheMiddleware gin.HandlerFunc
	if viper.GetBool("cache.enabled") {
		maxSizeMB := viper.GetInt64("cache.max_size_mb")
		memCache := cache.NewMemoryCache(maxSizeMB)

		ttlByPath := make(map[string]time.Duration)
		ttlByPath["/api/v1/search"] = time.Duration(viper.GetInt("cache.ttl_search")) * time.Minute
		ttlByPath["/api/v1/trending"] = time.Duration(viper.GetInt("cache.ttl_trending")) * time.Minute
		ttlByPath["/api/v1/podcasts"] = time.Duration(viper.GetInt("cache.ttl_podcast")) * time.Minute
		ttlByPath["/api/v1/episodes"] = time.Duration(viper.GetInt("cache.ttl_episode")) * time.Minute
		ttlByPath["/api/v1/categories"] = time.Duration(viper.GetInt("cache.ttl_categories")) * time.Minute

		cacheConfig := middleware.CacheConfig{
			Cache:      memCache,
			DefaultTTL: viper.GetDuration("cache.default_ttl"),
			TTLByPath:  ttlByPath,
			Enabled:    true,
		}

		cacheMiddleware = middleware.CacheMiddleware(cacheConfig)
	}

	searchGroup := v1.Group("/search")
	searchGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, SearchRateLimit, SearchRateLimitBurst))
	if cacheMiddleware != nil {
		searchGroup.Use(cacheMiddleware)
	}
	search.RegisterRoutes(searchGroup, deps)

	trendingGroup := v1.Group("/trending")
	trendingGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	if cacheMiddleware != nil {
		trendingGroup.Use(cacheMiddleware)
	}
	trending.RegisterRoutes(trendingGroup, deps)

	categoriesGroup := v1.Group("/categories")
	categoriesGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	if cacheMiddleware != nil {
		categoriesGroup.Use(cacheMiddleware)
	}
	categories.RegisterRoutes(categoriesGroup, deps)

	randomGroup := v1.Group("/random")
	randomGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
	random.RegisterRoutes(randomGroup, deps)

	if deps.DB != nil && deps.DB.DB != nil {
		initializeAllServices(deps, cfg)

		episodeGroup := v1.Group("/episodes")
		episodeGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst))
		if cacheMiddleware != nil {
			episodeGroup.Use(cacheMiddleware)
		}

		episodes.RegisterRoutes(episodeGroup, deps)
		waveform.RegisterRoutes(episodeGroup, deps)

		if viper.GetBool("transcription.enabled") {
			transcriptionAPI.RegisterRoutes(episodeGroup, deps)
			log.Println("[INFO] Transcription routes enabled")
		}

		podcastGroup := v1.Group("/podcasts")
		podcastMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst)
		episodesMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, GeneralRateLimit, GeneralRateLimitBurst)
		if cacheMiddleware != nil {
			podcastGroup.Use(cacheMiddleware)
		}
		podcasts.RegisterRoutes(podcastGroup, deps, podcastMiddleware, episodesMiddleware)

		// Clips are now handled under /episodes/:id/clips (see episodes routes)
	}

	return nil
}

func initializeAllServices(deps *types.Dependencies, cfg *config.Config) {
	// Initialize job service FIRST - other services depend on it
	if deps.JobService == nil {
		initializeJobService(deps)
	}

	// Initialize podcast service before episode service (episode service depends on it)
	if deps.PodcastService == nil {
		initializePodcastService(deps)
	}

	if deps.EpisodeService == nil || deps.EpisodeTransformer == nil {
		initializeEpisodeService(deps, cfg)
	}

	// Initialize audio cache service (required by waveform, transcription, episode analysis)
	if deps.AudioCacheService == nil {
		initializeAudioCacheService(deps)
	}

	if deps.WaveformService == nil {
		initializeWaveformService(deps)
	}

	if deps.TranscriptionService == nil {
		initializeTranscriptionService(deps)
	}

	// Initialize clip service if not set (depends on JobService)
	if deps.ClipService == nil {
		initializeClipService(deps)
	}

	// Initialize episode analysis service if not set (depends on AudioCacheService, ClipService, EpisodeService)
	if deps.EpisodeAnalysisService == nil {
		initializeEpisodeAnalysisService(deps)
	}

	if deps.ITunesClient == nil {
		initializeITunesClient(deps)
	}
}

func initializeEpisodeService(deps *types.Dependencies, _ *config.Config) {
	if deps.PodcastClient == nil {
		log.Printf("[ERROR] PodcastClient is nil - episode service will not be able to fetch from API")
		// Still create the service but with nil fetcher - it will only use database
		episodeRepo := episodesService.NewRepository(deps.DB.DB)
		episodeCache := episodesService.NewCache(time.Hour)

		deps.EpisodeService = episodesService.NewService(
			nil,
			episodeRepo,
			episodeCache,
			deps.PodcastService, // May be nil
		)
		deps.EpisodeTransformer = episodesService.NewTransformer()
		return
	}

	podcastClient, ok := deps.PodcastClient.(*podcastindex.Client)
	if !ok {
		log.Printf("[ERROR] PodcastClient is not of expected type *podcastindex.Client")
		return
	}

	episodeFetcher := episodesService.NewPodcastIndexAdapter(podcastClient)
	episodeRepo := episodesService.NewRepository(deps.DB.DB)
	episodeCache := episodesService.NewCache(time.Hour)

	maxConcurrentSync := config.GetInt("episodes.max_concurrent_sync")
	syncTimeout := config.GetDuration("episodes.sync_timeout")

	deps.EpisodeService = episodesService.NewService(
		episodeFetcher,
		episodeRepo,
		episodeCache,
		deps.PodcastService,
		episodesService.WithMaxConcurrentSync(maxConcurrentSync),
		episodesService.WithSyncTimeout(syncTimeout),
	)

	deps.EpisodeTransformer = episodesService.NewTransformer()
}

func initializePodcastService(deps *types.Dependencies) {
	if deps.PodcastClient == nil {
		log.Printf("[WARN] PodcastClient is nil - podcast service will not be able to fetch from API")
		return
	}

	podcastClient, ok := deps.PodcastClient.(*podcastindex.Client)
	if !ok {
		log.Printf("[ERROR] PodcastClient is not of expected type *podcastindex.Client")
		return
	}

	podcastRepo := podcastsService.NewRepository(deps.DB.DB)
	deps.PodcastService = podcastsService.NewService(podcastRepo, podcastClient)
	log.Printf("[INFO] Podcast service initialized successfully")
}

func initializeWaveformService(deps *types.Dependencies) {
	waveformRepo := waveforms.NewRepository(deps.DB.DB)
	deps.WaveformService = waveforms.NewService(waveformRepo)
}

func initializeTranscriptionService(deps *types.Dependencies) {
	transcriptionRepo := transcription.NewRepository(deps.DB.DB)
	deps.TranscriptionService = transcription.NewService(transcriptionRepo)
}

func initializeClipService(deps *types.Dependencies) {
	clipsBasePath := viper.GetString("clips.storage_path")
	tempDir := viper.GetString("temp_dir")
	targetDuration := viper.GetFloat64("clips.target_duration")

	extractor, err := clipsService.NewFFmpegExtractor(tempDir, targetDuration)
	if err != nil {
		log.Printf("[ERROR] Failed to create FFmpeg extractor: %v", err)
		return
	}

	storage, err := clipsService.NewLocalClipStorage(clipsBasePath)
	if err != nil {
		log.Printf("[ERROR] Failed to create clip storage: %v", err)
		return
	}

	if deps.JobService == nil {
		log.Printf("[WARN] JobService not initialized, clip service will not be able to process clips")
		return
	}

	// Episode service is required to fetch episode URLs, audio cache is optional but improves performance
	if deps.EpisodeService == nil {
		log.Printf("[ERROR] EpisodeService not initialized, clip service requires it to fetch episode URLs")
		return
	}

	deps.ClipService = clipsService.NewService(
		deps.DB.DB,
		storage,
		extractor,
		deps.JobService,
		deps.EpisodeService,
		deps.AudioCacheService,
	)
	log.Printf("[INFO] Clip service initialized with storage at %s", clipsBasePath)
}

func initializeAudioCacheService(deps *types.Dependencies) {
	audioCacheRepo := audiocache.NewRepository(deps.DB.DB)
	cacheDir := viper.GetString("audio_cache.directory")
	storage, err := audiocache.NewFilesystemStorage(cacheDir)
	if err != nil {
		log.Printf("[ERROR] Failed to initialize audio cache storage: %v", err)
		return
	}

	deps.AudioCacheService = audiocache.NewService(audioCacheRepo, storage)
	log.Printf("[INFO] Audio cache service initialized with storage at %s", cacheDir)
}

func initializeEpisodeAnalysisService(deps *types.Dependencies) {
	if deps.AudioCacheService == nil {
		log.Printf("[ERROR] AudioCacheService not initialized, episode analysis service requires it")
		return
	}
	if deps.ClipService == nil {
		log.Printf("[ERROR] ClipService not initialized, episode analysis service requires it")
		return
	}
	if deps.EpisodeService == nil {
		log.Printf("[ERROR] EpisodeService not initialized, episode analysis service requires it")
		return
	}

	deps.EpisodeAnalysisService = episodeanalysis.NewService(
		deps.AudioCacheService,
		deps.ClipService,
		deps.EpisodeService,
	)
	log.Printf("[INFO] Episode analysis service initialized")
}

func initializeJobService(deps *types.Dependencies) {
	jobRepo := jobs.NewRepository(deps.DB.DB)
	deps.JobService = jobs.NewService(jobRepo)
}

func initializeITunesClient(deps *types.Dependencies) {
	itunesConfig := itunes.Config{
		RequestsPerMinute: 250,
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
