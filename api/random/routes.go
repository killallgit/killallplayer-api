package random

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/random
	router.GET("", Get(deps))
}
