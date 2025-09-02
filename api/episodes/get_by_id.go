package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByID returns a single episode by Podcast Index ID
func GetByID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		log.Printf("[DEBUG] GetByID called with Podcast Index ID: %s", episodeIDStr)
		
		// Parse Podcast Index ID (int64)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			log.Printf("[ERROR] Invalid episode ID '%s': %v", episodeIDStr, err)
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid episode ID"))
			return
		}

		log.Printf("[DEBUG] Fetching episode with Podcast Index ID: %d", podcastIndexID)
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			if IsNotFound(err) {
				log.Printf("[WARN] Episode not found - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode with Podcast Index ID %d: %v", podcastIndexID, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		log.Printf("[DEBUG] Episode found - Podcast Index ID: %d, Title: %s, AudioURL: %s", 
			podcastIndexID, episode.Title, episode.AudioURL)
		
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
