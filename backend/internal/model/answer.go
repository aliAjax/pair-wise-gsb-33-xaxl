package model

import "time"

// Answer is a reply to a Question.
type Answer struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	QuestionID uint   `gorm:"index;not null" json:"question_id"`
	UserID     uint   `gorm:"index;not null" json:"user_id"`
	Content    string `gorm:"type:text;not null" json:"content"`
	IsBest     bool   `gorm:"default:false" json:"is_best"`
	LikeCount  int    `gorm:"default:0" json:"like_count"`
	// Version bumps every time the content is edited so that generated drafts
	// can tell whether they still reflect the adopted answer.
	Version   int       `gorm:"not null;default:1" json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
