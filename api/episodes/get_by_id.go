package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByID returns a single episode by ID
func GetByID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid episode ID"))
			return
		}

		episode, err := deps.EpisodeService.GetEpisodeByID(c.Request.Context(), uint(episodeID))
		if err != nil {
			if IsNotFound(err) {
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode %d: %v", episodeID, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		response := deps.EpisodeTransformer.CreateSingleEpisodeResponse(episode)
		c.JSON(http.StatusOK, response)
	}
}

// IsNotFound checks if the error indicates a not found condition
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "episode not found" || err.Error() == "record not found"
}
