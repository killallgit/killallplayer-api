package models

import (
	"time"
)

// Region represents a marked segment in an episode (bookmark, note, etc.)
type Region struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	EpisodeID  int64     `json:"episodeId" gorm:"not null;index"`
	StartTime  float64   `json:"startTime" gorm:"not null"`
	EndTime    float64   `json:"endTime" gorm:"not null"`
	Label      string    `json:"label"`
	Color      string    `json:"color"`
	IsBookmark bool      `json:"isBookmark" gorm:"default:false"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
