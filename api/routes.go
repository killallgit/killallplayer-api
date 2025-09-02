package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-contrib/swagger"
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/episodes"
	"github.com/killallgit/player-api/api/health"
	"github.com/killallgit/player-api/api/podcasts"
	"github.com/killallgit/player-api/api/search"
	"github.com/killallgit/player-api/api/stream"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/api/version"
	"github.com/killallgit/player-api/docs"
	episodesService "github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/viper"
	"github.com/swaggo/files"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(engine *gin.Engine, deps *types.Dependencies, rateLimiters *sync.Map, cleanupStop chan struct{}, cleanupInitialized *sync.Once) error {
	// Register public routes (no rate limiting)
	health.RegisterRoutes(engine, deps)
	version.RegisterRoutes(engine, deps)

	// Swagger documentation with static token authentication
	docs.SwaggerInfo.BasePath = "/"
	swaggerGroup := engine.Group("/swagger")
	swaggerGroup.Use(SwaggerAuthMiddleware())
	swaggerGroup.GET("/*any", swagger.WrapHandler(files.Handler))

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

	// Register search routes with dedicated rate limiting (5 req/s, burst of 10)
	searchGroup := v1.Group("/search")
	searchGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 5, 10))
	search.RegisterRoutes(searchGroup, deps)

	// Initialize episode service if database is available
	if deps.DB != nil && deps.DB.DB != nil {
		if deps.EpisodeService == nil || deps.EpisodeTransformer == nil {
			initializeEpisodeService(deps, cfg)
		}

		// Register episode routes with general rate limiting (10 req/s, burst of 20)
		episodeGroup := v1.Group("/episodes")
		episodeGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 10, 20))
		episodes.RegisterRoutes(episodeGroup, deps)

		// Register streaming routes with moderate rate limiting (20 req/s, burst of 30)
		// Higher limits for streaming to allow seeking/scrubbing
		streamGroup := v1.Group("/stream")
		streamGroup.Use(PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 20, 30))
		stream.RegisterRoutes(streamGroup, deps)

		// Register podcast routes with mixed rate limiting
		podcastGroup := v1.Group("/podcasts")
		// Create middleware for different rate limits
		episodesMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 10, 20)
		syncMiddleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 1, 2)
		podcasts.RegisterRoutes(podcastGroup, deps, episodesMiddleware, syncMiddleware)
	}

	return nil
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
