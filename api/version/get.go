package version

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Get handles version requests
// @Summary      Get API version
// @Description  Get version and basic information about the Podcast Player API
// @Tags         version
// @Accept       json
// @Produce      json
// @Success      200 {object} object{name=string,version=string,description=string,status=string} "API version information"
// @Router       / [get]
func Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        "Podcast Player API",
			"version":     "1.0.0",
			"description": "API for managing podcasts and episodes",
			"status":      "running",
		})
	}
}
