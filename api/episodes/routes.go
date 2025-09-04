package episodes

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers episode routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/episodes - Get all episodes with pagination
	router.GET("", GetAll(deps))

	// GET /api/v1/episodes/byfeedid
	router.GET("/byfeedid", GetByFeedID(deps))

	// GET /api/v1/episodes/byguid
	router.GET("/byguid", GetByGUID(deps))

	// GET /api/v1/episodes/recent
	router.GET("/recent", GetRecent(deps))

	// GET /api/v1/episodes/by-feed-url?url=<feed_url>
	router.GET("/by-feed-url", GetByFeedURL(deps))

	// GET /api/v1/episodes/by-itunes-id?id=<itunes_id>
	router.GET("/by-itunes-id", GetByiTunesID(deps))

	// GET /api/v1/episodes/:id - Note: This must come after other specific routes
	router.GET("/:id", GetByID(deps))

	// PUT /api/v1/episodes/:id/playback
	router.PUT("/:id/playback", PutPlayback(deps))
}
