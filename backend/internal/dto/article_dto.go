package dto

// ArticleCreateRequest is the payload for creating articles.
type ArticleCreateRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag" binding:"required"`
}

// ArticleUpdateRequest is the payload for editing articles. TopicTag is
// optional because a Q&A-generated draft has no tag until it is published.
type ArticleUpdateRequest struct {
	Title    string `json:"title" binding:"omitempty,max=255"`
	Content  string `json:"content" binding:"omitempty"`
	Cover    string `json:"cover" binding:"omitempty,max=255"`
	TopicTag string `json:"topic_tag" binding:"omitempty"`
}
