package constants

// CareTopicTag enumerates care article topic tags.
const (
	TopicTagFertilizing = "fertilizing"  // 施肥
	TopicTagPruning     = "pruning"      // 修剪
	TopicTagRepotting   = "repotting"    // 换盆
	TopicTagPestControl = "pest_control" // 病虫害
	TopicTagPropagation = "propagation"  // 繁殖
)

// ValidTopicTags returns all accepted topic tag values.
func ValidTopicTags() []string {
	return []string{TopicTagFertilizing, TopicTagPruning, TopicTagRepotting, TopicTagPestControl, TopicTagPropagation}
}

// IsValidTopicTag reports whether the tag is known.
func IsValidTopicTag(t string) bool {
	for _, v := range ValidTopicTags() {
		if v == t {
			return true
		}
	}
	return false
}

// Article statuses.
const (
	ArticleStatusDraft     = "draft"
	ArticleStatusPublished = "published"
	// ArticleStatusInvalidated marks a Q&A-generated draft whose adopted
	// answer was edited before the draft was published. Published articles
	// never enter this state; their frozen text is preserved.
	ArticleStatusInvalidated = "invalidated"
)

// IsValidArticleStatus reports whether the status is one of the known values.
func IsValidArticleStatus(s string) bool {
	switch s {
	case ArticleStatusDraft, ArticleStatusPublished, ArticleStatusInvalidated:
		return true
	}
	return false
}
