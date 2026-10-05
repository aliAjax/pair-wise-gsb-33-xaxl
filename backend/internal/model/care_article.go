package model

import "time"

// CareArticle is a gardening knowledge article.
//
// Articles may be authored directly or be distilled from an adopted Q&A
// answer. When SourceQuestionID > 0 the article carries frozen snapshots of
// the question and answer at adoption time; publishing never rewrites those
// snapshots, while editing the adopted answer only invalidates an
// unpublished draft (StatusInvalidated) instead of touching published text.
type CareArticle struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"index;not null" json:"user_id"`
	Title     string `gorm:"size:255;not null" json:"title"`
	Content   string `gorm:"type:longtext" json:"content"`
	Cover     string `gorm:"size:255" json:"cover"`
	TopicTag  string `gorm:"size:32;index;not null" json:"topic_tag"`
	Status    string `gorm:"size:16;default:published;index" json:"status"`
	ViewCount int    `gorm:"default:0" json:"view_count"`

	// Q&A provenance. Zero values mean the article was authored directly.
	SourceQuestionID uint   `gorm:"index;not null;default:0" json:"source_question_id"`
	SourceAnswerID   uint   `gorm:"index;not null;default:0" json:"source_answer_id"`
	QuestionSnapshot string `gorm:"type:longtext" json:"question_snapshot"`
	AnswerSnapshot   string `gorm:"type:longtext" json:"answer_snapshot"`
	// AnswerVersion records the answer Version this draft/snapshot was built
	// from; a mismatch means the adopted answer was edited afterwards.
	AnswerVersion int `gorm:"not null;default:0" json:"answer_version"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
