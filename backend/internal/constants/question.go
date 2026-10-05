package constants

// QuestionStatus enumerates Q&A question lifecycle states. A question starts
// open; adopting a best answer closes it atomically and the closure is final,
// which pins the question/answer pair the sedimented article draft is built
// from.
const (
	QuestionStatusOpen   = "open"
	QuestionStatusClosed = "closed"
)

// ValidQuestionStatuses returns all accepted question status values.
func ValidQuestionStatuses() []string {
	return []string{QuestionStatusOpen, QuestionStatusClosed}
}

// IsValidQuestionStatus reports whether the status is known.
func IsValidQuestionStatus(s string) bool {
	for _, v := range ValidQuestionStatuses() {
		if v == s {
			return true
		}
	}
	return false
}
