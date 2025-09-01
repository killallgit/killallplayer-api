package version

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers version routes
func RegisterRoutes(engine *gin.Engine, deps *types.Dependencies) {
	engine.GET("/", Get())
}
