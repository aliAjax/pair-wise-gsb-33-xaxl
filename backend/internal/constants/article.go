package constants

// CareTopicTag enumerates care article topic tags.
const (
	TopicTagFertilizing = "fertilizing" // 施肥
	TopicTagPruning     = "pruning"     // 修剪
	TopicTagRepotting   = "repotting"   // 换盆
	TopicTagPestControl = "pest_control" // 病虫害
	TopicTagPropagation = "propagation" // 繁殖
	TopicTagQA          = "qa"          // 问答精选
)

// ValidTopicTags returns all accepted topic tag values.
func ValidTopicTags() []string {
	return []string{TopicTagFertilizing, TopicTagPruning, TopicTagRepotting, TopicTagPestControl, TopicTagPropagation, TopicTagQA}
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
	ArticleStatusInvalid   = "invalid" // 未发布草稿因回答被修改而失效
)
