package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByGUID returns episode by GUID
func GetByGUID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		guid := c.Query("guid")
		if guid == "" {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("GUID parameter is required"))
			return
		}

		episode, err := deps.EpisodeService.GetEpisodeByGUID(c.Request.Context(), guid)
		if err != nil {
			if IsNotFound(err) {
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode by GUID %s: %v", guid, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		response := deps.EpisodeTransformer.CreateSingleEpisodeResponse(episode)
		c.JSON(http.StatusOK, response)
	}
}
