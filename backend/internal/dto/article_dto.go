package dto

// ArticleCreateRequest is the payload for creating/updating articles.
type ArticleCreateRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag" binding:"required"`
}

// ArticleUpdateRequest edits an article; every field is optional. Status only
// accepts the draft→published transition used to publish a sedimented draft.
type ArticleUpdateRequest struct {
	Title    string `json:"title" binding:"omitempty,max=255"`
	Content  string `json:"content"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag"`
	Status   string `json:"status" binding:"omitempty,oneof=draft published"`
}
