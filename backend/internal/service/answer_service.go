package service

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// AnswerService implements answer creation, like and adoption.
type AnswerService struct {
	db           *gorm.DB
	repo         *repository.AnswerRepository
	questionRepo *repository.QuestionRepository
	articleRepo  *repository.CareArticleRepository
	logger       *slog.Logger
}

// NewAnswerService creates an AnswerService.
func NewAnswerService(db *gorm.DB, repo *repository.AnswerRepository, questionRepo *repository.QuestionRepository, articleRepo *repository.CareArticleRepository, logger *slog.Logger) *AnswerService {
	return &AnswerService{db: db, repo: repo, questionRepo: questionRepo, articleRepo: articleRepo, logger: logger}
}

// Create adds an answer to a question. Closed questions are pinned together
// with their best answer and no longer accept new answers.
func (s *AnswerService) Create(userID, questionID uint, content string) (*model.Answer, error) {
	q, err := s.questionRepo.FindByID(questionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
		}
		return nil, fmt.Errorf("answer create question find: %w", err)
	}
	if q.Status == constants.QuestionStatusClosed {
		s.logger.Warn(fmt.Sprintf(constants.LogAnswerCreateRejected, questionID, q.Status), "question_id", questionID)
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("Question[id=%d] answer create failed: question already closed", questionID))
	}
	a := &model.Answer{QuestionID: questionID, UserID: userID, Content: content}
	if err := s.repo.Create(a); err != nil {
		return nil, fmt.Errorf("answer create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerCreateSuccess, questionID), "id", a.ID)
	return a, nil
}

// ListByQuestion returns answers for a question.
func (s *AnswerService) ListByQuestion(questionID uint) ([]model.Answer, error) {
	items, err := s.repo.ListByQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("answer list: %w", err)
	}
	return items, nil
}

// Adopt marks an answer as the best answer and sediments the Q&A pair into a
// care article draft. Only the question owner may adopt, and only once: the
// whole outcome (close question + mark best + generate draft) commits in a
// single transaction, so a failed draft generation rolls the best answer back
// and the owner can simply retry. Concurrent adopt requests race on the
// open→closed status flip; the first transaction to flip it wins and later
// ones get a 409 conflict.
func (s *AnswerService) Adopt(userID, questionID, answerID uint) (*model.Answer, *model.CareArticle, error) {
	var adopted *model.Answer
	var draft *model.CareArticle
	err := s.db.Transaction(func(tx *gorm.DB) error {
		closed, err := s.questionRepo.CloseIfOpenTx(tx, questionID)
		if err != nil {
			return fmt.Errorf("answer adopt close question: %w", err)
		}
		if !closed {
			if _, findErr := s.questionRepo.FindByIDTx(tx, questionID); errors.Is(findErr, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
			}
			s.logger.Warn(fmt.Sprintf(constants.LogAnswerAdoptConflict, questionID), "question_id", questionID)
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("Question[id=%d] adopt failed: question already closed", questionID))
		}
		q, err := s.questionRepo.FindByIDTx(tx, questionID)
		if err != nil {
			return fmt.Errorf("answer adopt question find: %w", err)
		}
		if q.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("Answer[question_id=%d] adopt failed: user_id=%d not question owner", questionID, userID))
		}
		a, err := s.repo.FindByIDTx(tx, answerID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
			}
			return fmt.Errorf("answer adopt find: %w", err)
		}
		if a.QuestionID != questionID {
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("Answer[id=%d] adopt failed: question_id mismatch", answerID))
		}
		if err := s.repo.ClearBestForQuestionTx(tx, questionID); err != nil {
			return fmt.Errorf("answer adopt clear best: %w", err)
		}
		a.IsBest = true
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("answer adopt update: %w", err)
		}
		d := &model.CareArticle{
			UserID:        q.UserID,
			Title:         q.Title,
			Content:       buildQADraftContent(q, a),
			TopicTag:      constants.TopicTagQA,
			Status:        constants.ArticleStatusDraft,
			QuestionID:    q.ID,
			AnswerID:      a.ID,
			AnswerVersion: a.Version,
		}
		if err := s.articleRepo.CreateTx(tx, d); err != nil {
			return fmt.Errorf("answer adopt draft create: %w", err)
		}
		adopted = a
		draft = d
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerAdoptSuccess, answerID), "question_id", questionID)
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftGenerated, questionID, answerID), "article_id", draft.ID)
	return adopted, draft, nil
}

// buildQADraftContent snapshots the question and the adopted answer into the
// draft body. Published articles keep this original snapshot even if the
// answer is edited afterwards.
func buildQADraftContent(q *model.Question, a *model.Answer) string {
	return fmt.Sprintf("## 问题\n\n%s\n\n## 最佳回答\n\n%s", q.Content, a.Content)
}

// Update edits an answer owned by the current user. The version bump and the
// invalidation of unpublished QA drafts derived from this answer commit in one
// transaction; published articles keep their original snapshot.
func (s *AnswerService) Update(userID, answerID uint, content string) (*model.Answer, error) {
	a, err := s.repo.FindByID(answerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
		}
		return nil, fmt.Errorf("answer update find: %w", err)
	}
	if a.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("Answer[id=%d] update failed: user_id=%d is not owner", answerID, userID))
	}
	a.Content = content
	a.Version++
	var invalidated int64
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("answer update save: %w", err)
		}
		n, err := s.articleRepo.InvalidateDraftsByAnswerTx(tx, a.ID)
		if err != nil {
			return fmt.Errorf("answer update invalidate drafts: %w", err)
		}
		invalidated = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerUpdateSuccess, a.ID, a.Version), "id", a.ID)
	if invalidated > 0 {
		s.logger.Info(fmt.Sprintf(constants.LogArticleDraftInvalidated, a.ID, invalidated), "answer_id", a.ID)
	}
	return a, nil
}

// Like increments the like count of an answer.
func (s *AnswerService) Like(answerID uint) (*model.Answer, error) {
	if err := s.repo.IncrementLike(answerID); err != nil {
		return nil, fmt.Errorf("answer like: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerLikeSuccess, answerID), "id", answerID)
	return s.repo.FindByID(answerID)
}
