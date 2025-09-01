package version

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Get handles version requests
func Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        "Podcast Player API",
			"version":     "1.0.0",
			"description": "API for managing and streaming podcasts",
			"status":      "running",
		})
	}
}
