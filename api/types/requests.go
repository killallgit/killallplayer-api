package types

// SearchRequest represents a podcast search request
type SearchRequest struct {
	Query    string `json:"query" binding:"required" example:"technology"`
	Limit    int    `json:"limit,omitempty" example:"10"`
	FullText bool   `json:"fullText,omitempty" example:"false"`
	Val      string `json:"val,omitempty" example:"any"`      // Filter by value block type (e.g., "any", "lightning")
	ApOnly   bool   `json:"apOnly,omitempty" example:"false"` // Only return podcasts with iTunes ID
	Clean    bool   `json:"clean,omitempty" example:"false"`  // Only return non-explicit content
}

// TrendingRequest represents a trending podcasts request
type TrendingRequest struct {
	Max        int      `json:"max,omitempty" validate:"min=1,max=100" example:"10"`
	Since      int      `json:"since,omitempty" validate:"min=1,max=720" example:"24"` // Hours ago (max 30 days)
	Categories []string `json:"categories,omitempty" example:"News,Technology"`        // Category names/IDs to filter
	Lang       string   `json:"lang,omitempty" validate:"max=10" example:"en"`         // Language code
	FullText   bool     `json:"fullText,omitempty" example:"false"`                    // Return full descriptions
}
