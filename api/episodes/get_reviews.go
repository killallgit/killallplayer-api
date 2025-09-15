package episodes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetReviews fetches podcast reviews for the podcast containing this episode
// @Summary      Get podcast reviews
// @Description  Fetch customer reviews from Apple Podcasts for the podcast that contains this episode
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode ID (Podcast Index ID)"
// @Param        sort query string false "Sort order: mostrecent or mosthelpful" default(mostrecent)
// @Param        page query int false "Page number (1-10)" minimum(1) maximum(10) default(1)
// @Success      200 {object} episodes.ReviewsResponse "Reviews data"
// @Failure      404 {object} types.ErrorResponse "Episode not found or no iTunes ID available"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/reviews [get]
func GetReviews(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID
		episodeID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return
		}

		// Get episode to extract iTunes ID
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), episodeID)
		if err != nil {
			if IsNotFound(err) {
				types.SendNotFound(c, "Episode not found")
			} else {
				log.Printf("[ERROR] Failed to fetch episode %d: %v", episodeID, err)
				types.SendInternalError(c, "Failed to fetch episode")
			}
			return
		}

		// Check if we have an iTunes ID
		if episode.FeedItunesID == nil || *episode.FeedItunesID == 0 {
			c.JSON(http.StatusOK, ReviewsResponse{
				Status:    "success",
				EpisodeID: episodeID,
				Message:   "No iTunes ID available for this podcast",
			})
			return
		}

		itunesID := *episode.FeedItunesID

		// Parse query parameters
		sort := c.DefaultQuery("sort", "mostrecent")
		if sort != "mostrecent" && sort != "mosthelpful" {
			sort = "mostrecent"
		}

		page := 1
		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p >= 1 && p <= 10 {
				page = p
			}
		}

		// Check if iTunes client is available
		if deps.ITunesClient == nil {
			log.Printf("[WARN] iTunes client not configured")
			c.JSON(http.StatusOK, ReviewsResponse{
				Status:    "success",
				EpisodeID: episodeID,
				ITunesID:  itunesID,
				Message:   "Apple reviews service not available",
			})
			return
		}

		// Fetch reviews from Apple RSS feed
		reviewData, err := fetchReviews(itunesID, sort, page)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch Apple reviews for iTunes ID %d: %v", itunesID, err)
			c.JSON(http.StatusOK, ReviewsResponse{
				Status:    "success",
				EpisodeID: episodeID,
				ITunesID:  itunesID,
				Message:   "Could not fetch reviews at this time",
			})
			return
		}

		now := time.Now()
		c.JSON(http.StatusOK, ReviewsResponse{
			Status:    "success",
			EpisodeID: episodeID,
			ITunesID:  itunesID,
			Reviews:   reviewData,
			CachedAt:  &now,
		})
	}
}

// fetchReviews fetches reviews from Apple's RSS feed
func fetchReviews(itunesID int64, sort string, page int) (*ReviewData, error) {
	// Construct Apple RSS URL
	url := fmt.Sprintf("https://itunes.apple.com/us/rss/customerreviews/page=%d/id=%d/sortby=%s/json",
		page, itunesID, sort)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviews: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Apple API returned status %d", resp.StatusCode)
	}

	// Parse Apple RSS response
	var rssResp appleRSSResponse
	if err := json.NewDecoder(resp.Body).Decode(&rssResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our format
	return convertAppleReviews(&rssResp, sort), nil
}

// appleRSSResponse represents the Apple RSS feed structure
type appleRSSResponse struct {
	Feed struct {
		Entry []appleRSSEntry `json:"entry"`
	} `json:"feed"`
}

type appleRSSEntry struct {
	Author struct {
		Name struct {
			Label string `json:"label"`
		} `json:"name"`
	} `json:"author"`
	Title struct {
		Label string `json:"label"`
	} `json:"title"`
	Content struct {
		Label string `json:"label"`
	} `json:"content"`
	IMRating struct {
		Label string `json:"label"`
	} `json:"im:rating"`
	IMVoteCount struct {
		Label string `json:"label"`
	} `json:"im:voteCount"`
	IMVoteSum struct {
		Label string `json:"label"`
	} `json:"im:voteSum"`
	ID struct {
		Label string `json:"label"`
	} `json:"id"`
	Updated struct {
		Label string `json:"label"`
	} `json:"updated"`
}

// convertAppleReviews converts Apple RSS format to our AppleReviewData format
func convertAppleReviews(rssResp *appleRSSResponse, sort string) *ReviewData {
	reviewData := &ReviewData{
		TotalCount:         len(rssResp.Feed.Entry),
		RatingDistribution: make(map[string]int),
		RecentReviews:      []Review{},
		MostHelpful:        []Review{},
	}

	var totalRating float64
	for _, entry := range rssResp.Feed.Entry {
		// Parse rating
		rating, _ := strconv.Atoi(entry.IMRating.Label)
		totalRating += float64(rating)

		// Update rating distribution
		reviewData.RatingDistribution[entry.IMRating.Label]++

		// Parse vote counts
		voteCount, _ := strconv.Atoi(entry.IMVoteCount.Label)
		voteSum, _ := strconv.Atoi(entry.IMVoteSum.Label)

		// Parse updated time
		updatedAt, _ := time.Parse(time.RFC3339, entry.Updated.Label)

		// Extract review ID from URL
		idParts := strings.Split(entry.ID.Label, "/")
		reviewID := ""
		if len(idParts) > 0 {
			reviewID = idParts[len(idParts)-1]
		}

		review := Review{
			ID:        reviewID,
			Author:    entry.Author.Name.Label,
			Rating:    rating,
			Title:     entry.Title.Label,
			Content:   entry.Content.Label,
			VoteCount: voteCount,
			VoteSum:   voteSum,
			UpdatedAt: updatedAt,
		}

		// Add to appropriate list based on sort
		if sort == "mosthelpful" {
			reviewData.MostHelpful = append(reviewData.MostHelpful, review)
		} else {
			reviewData.RecentReviews = append(reviewData.RecentReviews, review)
		}

		// Limit to 10 reviews in response
		if len(reviewData.RecentReviews) >= 10 || len(reviewData.MostHelpful) >= 10 {
			break
		}
	}

	// Calculate average rating
	if reviewData.TotalCount > 0 {
		reviewData.AverageRating = totalRating / float64(reviewData.TotalCount)
	}

	return reviewData
}
