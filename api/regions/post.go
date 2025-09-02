package regions

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// RegionRequest represents the request body for saving a region
type RegionRequest struct {
	EpisodeID  int64   `json:"episodeId" binding:"required"`
	StartTime  float64 `json:"startTime" binding:"required"`
	EndTime    float64 `json:"endTime" binding:"required"`
	Label      string  `json:"label"`
	Color      string  `json:"color"`
	IsBookmark bool    `json:"isBookmark"`
}

// SaveRegion handles saving a playback region for an episode
func SaveRegion(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":      "false",
				"description": "Invalid request body",
				"error":       err.Error(),
			})
			return
		}

		// Create region model
		region := models.Region{
			EpisodeID:  req.EpisodeID,
			StartTime:  req.StartTime,
			EndTime:    req.EndTime,
			Label:      req.Label,
			Color:      req.Color,
			IsBookmark: req.IsBookmark,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Save to database if available
		if deps.DB != nil && deps.DB.DB != nil {
			if err := deps.DB.DB.Create(&region).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":      "false",
					"description": "Failed to save region",
					"error":       err.Error(),
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      "true",
			"description": "Region saved successfully",
			"region":      region,
		})
	}
}

// GetRegions returns all regions for an episode
func GetRegions(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeID := c.Query("episodeId")
		if episodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":      "false",
				"description": "Episode ID is required",
			})
			return
		}

		var regions []models.Region
		
		// Fetch from database if available
		if deps.DB != nil && deps.DB.DB != nil {
			if err := deps.DB.DB.Where("episode_id = ?", episodeID).Find(&regions).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":      "false",
					"description": "Failed to fetch regions",
					"error":       err.Error(),
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      "true",
			"regions":     regions,
			"count":       len(regions),
			"description": "Regions fetched successfully",
		})
	}
}