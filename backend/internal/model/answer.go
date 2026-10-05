package model

import "time"

// Answer is a reply to a Question. Version starts at 1 and increments on every
// edit; QA-derived article drafts snapshot the version they were built from.
type Answer struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	QuestionID uint      `gorm:"index;not null" json:"question_id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	IsBest     bool      `gorm:"default:false" json:"is_best"`
	LikeCount  int       `gorm:"default:0" json:"like_count"`
	Version    int       `gorm:"default:1;not null" json:"version"`
	CreatedAt  time.Time `json:"created_at"`
}
