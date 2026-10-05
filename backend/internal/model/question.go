package model

import "time"

// Question statuses.
const (
	// QuestionStatusOpen waits for answers / an adoption.
	QuestionStatusOpen = "open"
	// QuestionStatusClosed means a best answer was adopted; the adoption is
	// final and a care article draft has been generated.
	QuestionStatusClosed = "closed"
)

// Question is a gardening Q&A post.
type Question struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Images    string    `gorm:"type:json" json:"images"`
	Status    string    `gorm:"size:16;default:open" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
