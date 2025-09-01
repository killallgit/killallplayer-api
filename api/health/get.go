package health

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// Get handles health check requests
func Get(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}

		// Add database status
		if deps != nil && deps.DB != nil {
			response["database"] = getDatabaseStatus(deps)
		} else {
			response["database"] = gin.H{"status": "not configured"}
		}

		c.JSON(http.StatusOK, response)
	}
}

// getDatabaseStatus returns the database connection status
func getDatabaseStatus(deps *types.Dependencies) gin.H {
	if deps.DB == nil || deps.DB.DB == nil {
		return gin.H{"status": "not configured"}
	}

	if err := deps.DB.HealthCheck(); err != nil {
		return gin.H{"status": "unhealthy", "error": err.Error()}
	}

	return gin.H{"status": "healthy"}
}
