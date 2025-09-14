package episodes

import "time"

// AppleMetadata contains Apple-specific podcast and episode metadata
type AppleMetadata struct {
	// Podcast-level data
	TrackCount       int              `json:"trackCount,omitempty" example:"616"`
	ArtworkURLs      *AppleArtwork    `json:"artworkUrls,omitempty"`
	ContentRating    string           `json:"contentRating,omitempty" example:"Explicit"`
	ChartPosition    *int             `json:"chartPosition,omitempty" example:"3"`
	GenreNames       []string         `json:"genres,omitempty" example:"Music,Society & Culture"`
	GenreIDs         []string         `json:"genreIds,omitempty" example:"1310,1324"`
	CollectionViewURL string          `json:"collectionViewUrl,omitempty" example:"https://podcasts.apple.com/us/podcast/..."`
	Country          string           `json:"country,omitempty" example:"USA"`

	// Review data
	Reviews          *AppleReviewData `json:"reviews,omitempty"`
}

// AppleArtwork contains various resolution artwork URLs from Apple
type AppleArtwork struct {
	Small   string `json:"small,omitempty" example:"https://...30x30bb.jpg"`
	Medium  string `json:"medium,omitempty" example:"https://...100x100bb.jpg"`
	Large   string `json:"large,omitempty" example:"https://...600x600bb.jpg"`
	Default string `json:"default,omitempty" example:"https://...600x600bb.jpg"`
}

// AppleReviewData contains aggregated review information from Apple Podcasts
type AppleReviewData struct {
	TotalCount        int                  `json:"totalCount" example:"487"`
	AverageRating     float64              `json:"averageRating" example:"4.2"`
	RatingDistribution map[string]int      `json:"ratingDistribution,omitempty"`
	RecentReviews     []AppleReview        `json:"recentReviews,omitempty"`
	MostHelpful       []AppleReview        `json:"mostHelpful,omitempty"`
}

// AppleReview represents a single review from Apple Podcasts
type AppleReview struct {
	ID           string    `json:"id,omitempty"`
	Author       string    `json:"author" example:"User123"`
	Rating       int       `json:"rating" example:"5"`
	Title        string    `json:"title" example:"Best podcast ever!"`
	Content      string    `json:"content" example:"Love this show..."`
	VoteCount    int       `json:"voteCount,omitempty" example:"42"`
	VoteSum      int       `json:"voteSum,omitempty" example:"38"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

// AppleReviewsResponse is the response format for the Apple reviews endpoint
type AppleReviewsResponse struct {
	Status      string           `json:"status" example:"success"`
	EpisodeID   int64            `json:"episodeId" example:"123456789"`
	ITunesID    int64            `json:"itunesId,omitempty" example:"1535809341"`
	Reviews     *AppleReviewData `json:"reviews,omitempty"`
	CachedAt    *time.Time       `json:"cachedAt,omitempty"`
	Message     string           `json:"message,omitempty"`
}

// AppleMetadataResponse is the response format for the Apple metadata endpoint
type AppleMetadataResponse struct {
	Status       string         `json:"status" example:"success"`
	EpisodeID    int64          `json:"episodeId" example:"123456789"`
	ITunesID     int64          `json:"itunesId,omitempty" example:"1535809341"`
	Metadata     *AppleMetadata `json:"metadata,omitempty"`
	CachedAt     *time.Time     `json:"cachedAt,omitempty"`
	Message      string         `json:"message,omitempty"`
}