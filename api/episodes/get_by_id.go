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
		log.Printf("[DEBUG] GetByID called with ID: %s", episodeIDStr)
		
		episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
		if err != nil {
			log.Printf("[ERROR] Invalid episode ID '%s': %v", episodeIDStr, err)
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid episode ID"))
			return
		}

		log.Printf("[DEBUG] Fetching episode with ID: %d", episodeID)
		episode, err := deps.EpisodeService.GetEpisodeByID(c.Request.Context(), uint(episodeID))
		if err != nil {
			if IsNotFound(err) {
				log.Printf("[WARN] Episode not found - ID: %d, Error: %v", episodeID, err)
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode %d: %v", episodeID, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		log.Printf("[DEBUG] Episode found - ID: %d, Title: %s, AudioURL: %s", 
			episodeID, episode.Title, episode.AudioURL)
		
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
