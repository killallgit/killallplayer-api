package health

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers health check routes
func RegisterRoutes(engine *gin.Engine, deps *types.Dependencies) {
	engine.GET("/health", Get(deps))
}
