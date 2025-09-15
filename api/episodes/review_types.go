package episodes

import "time"

// ReviewData contains aggregated review information from Apple Podcasts
type ReviewData struct {
	TotalCount         int            `json:"totalCount" example:"487"`
	AverageRating      float64        `json:"averageRating" example:"4.2"`
	RatingDistribution map[string]int `json:"ratingDistribution,omitempty"`
	RecentReviews      []Review       `json:"recentReviews,omitempty"`
	MostHelpful        []Review       `json:"mostHelpful,omitempty"`
}

// Review represents a single review from Apple Podcasts
type Review struct {
	ID        string    `json:"id,omitempty"`
	Author    string    `json:"author" example:"User123"`
	Rating    int       `json:"rating" example:"5"`
	Title     string    `json:"title" example:"Best podcast ever!"`
	Content   string    `json:"content" example:"Love this show..."`
	VoteCount int       `json:"voteCount,omitempty" example:"42"`
	VoteSum   int       `json:"voteSum,omitempty" example:"38"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// ReviewsResponse is the response format for the reviews endpoint
type ReviewsResponse struct {
	Status    string      `json:"status" example:"success"`
	EpisodeID int64       `json:"episodeId" example:"123456789"`
	ITunesID  int64       `json:"itunesId,omitempty" example:"1535809341"`
	Reviews   *ReviewData `json:"reviews,omitempty"`
	CachedAt  *time.Time  `json:"cachedAt,omitempty"`
	Message   string      `json:"message,omitempty"`
}
