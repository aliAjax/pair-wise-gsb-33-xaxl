package dto

import "github.com/gbplantwiki/gbplantwiki/internal/model"

// QuestionCreateRequest creates a Q&A question.
type QuestionCreateRequest struct {
	Title   string `json:"title" binding:"required,max=255"`
	Content string `json:"content" binding:"required"`
	Images  string `json:"images"`
}

// AnswerCreateRequest creates an answer.
type AnswerCreateRequest struct {
	Content string `json:"content" binding:"required"`
}

// AnswerUpdateRequest edits an answer owned by the caller.
type AnswerUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

// AdoptRequest marks an answer as best.
type AdoptRequest struct {
	AnswerID uint `json:"answer_id" binding:"required"`
}

// AdoptResponse returns the adopted answer together with the sedimented
// article draft so the frontend can link straight to the draft.
type AdoptResponse struct {
	Answer *model.Answer      `json:"answer"`
	Draft  *model.CareArticle `json:"draft"`
}
