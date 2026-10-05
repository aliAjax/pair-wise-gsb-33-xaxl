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

// AnswerService implements answer creation, edit, like and adoption.
type AnswerService struct {
	db           *gorm.DB
	repo         *repository.AnswerRepository
	questionRepo *repository.QuestionRepository
	draftSvc     *KnowledgeDraftService
	logger       *slog.Logger
}

// NewAnswerService creates an AnswerService.
func NewAnswerService(db *gorm.DB, repo *repository.AnswerRepository, questionRepo *repository.QuestionRepository,
	draftSvc *KnowledgeDraftService, logger *slog.Logger) *AnswerService {
	return &AnswerService{db: db, repo: repo, questionRepo: questionRepo, draftSvc: draftSvc, logger: logger}
}

// Create adds an answer to a question.
func (s *AnswerService) Create(userID, questionID uint, content string) (*model.Answer, error) {
	if _, err := s.questionRepo.FindByID(questionID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
		}
		return nil, fmt.Errorf("answer create question find: %w", err)
	}
	a := &model.Answer{QuestionID: questionID, UserID: userID, Content: content, Version: 1}
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

// Update edits an answer owned by the current user. When the answer was
// adopted, the still-unpublished draft generated from it is invalidated in the
// same transaction; a published care article keeps its frozen original text.
func (s *AnswerService) Update(userID, answerID uint, content string) (*model.Answer, error) {
	var updated *model.Answer
	err := s.db.Transaction(func(tx *gorm.DB) error {
		a, err := s.repo.FindByIDForUpdateTx(tx, answerID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Answer[id=%d] not found", answerID))
			}
			return fmt.Errorf("answer update find: %w", err)
		}
		if a.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("Answer[id=%d] update failed: user_id=%d is not author", answerID, userID))
		}
		a.Content = content
		a.Version++
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("answer update: %w", err)
		}
		if a.IsBest {
			if err := s.draftSvc.InvalidateForAnswerEditTx(tx, a); err != nil {
				return err
			}
		}
		updated = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerUpdateSuccess, answerID, updated.Version), "id", answerID)
	return updated, nil
}

// Adopt marks an answer as the best answer and generates a care article draft.
// Only the question owner may adopt.
//
// The question row is locked for the duration of the transaction: when two
// adoption requests for different answers race, the first one closes the
// question and generates its draft, the second one observes the closed state
// and is rejected with 409. Best-answer switch, question close and draft
// creation share one transaction, so a draft failure rolls everything back and
// never leaves a half-sedimented state.
func (s *AnswerService) Adopt(userID, questionID, answerID uint) (*model.Answer, *model.CareArticle, error) {
	var adopted *model.Answer
	var draft *model.CareArticle
	err := s.db.Transaction(func(tx *gorm.DB) error {
		q, err := s.questionRepo.FindByIDForUpdateTx(tx, questionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
			}
			return fmt.Errorf("answer adopt question find: %w", err)
		}
		if q.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("Answer[question_id=%d] adopt failed: user_id=%d not question owner", questionID, userID))
		}
		if q.Status == model.QuestionStatusClosed {
			s.logger.Warn(fmt.Sprintf(constants.LogAnswerAdoptConflict, questionID))
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("Question[id=%d] adopt failed: a best answer was already adopted", questionID))
		}
		a, err := s.repo.FindByIDForUpdateTx(tx, answerID)
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

		a.IsBest = true
		q.Status = model.QuestionStatusClosed
		if err := s.repo.ClearBestForQuestionTx(tx, questionID); err != nil {
			return fmt.Errorf("answer adopt clear best: %w", err)
		}
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("answer adopt update: %w", err)
		}
		if err := s.questionRepo.UpdateTx(tx, q); err != nil {
			return fmt.Errorf("answer adopt question update: %w", err)
		}
		draft, err = s.draftSvc.GenerateForAdoptionTx(tx, q, a)
		if err != nil {
			return err
		}
		adopted = a
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerAdoptSuccess, answerID), "question_id", questionID, "article_id", draft.ID)
	return adopted, draft, nil
}

// Like increments the like count of an answer.
func (s *AnswerService) Like(answerID uint) (*model.Answer, error) {
	if err := s.repo.IncrementLike(answerID); err != nil {
		return nil, fmt.Errorf("answer like: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogAnswerLikeSuccess, answerID), "id", answerID)
	return s.repo.FindByID(answerID)
}
