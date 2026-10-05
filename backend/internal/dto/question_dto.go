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

// AnswerUpdateRequest edits an answer.
type AnswerUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

// AdoptRequest marks an answer as best.
type AdoptRequest struct {
	AnswerID uint `json:"answer_id" binding:"required"`
}

// AdoptResultResponse is returned by adoption: the best answer and the care
// article draft generated atomically together with it.
type AdoptResultResponse struct {
	BestAnswer *model.Answer      `json:"best_answer"`
	Draft      *model.CareArticle `json:"draft"`
}

// DraftPublishRequest publishes a Q&A-generated draft.
type DraftPublishRequest struct {
	TopicTag string `json:"topic_tag" binding:"required"`
}
