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

// draftTitlePrefix marks care articles distilled from an adopted answer.
const draftTitlePrefix = "【问答沉淀】"

// KnowledgeDraftService manages care article drafts distilled from adopted
// Q&A answers. It owns the whole sedimentation lifecycle:
//
//   - adoption atomically closes the question, marks the best answer and
//     generates a draft carrying frozen question/answer snapshots;
//   - editing the adopted answer invalidates an unpublished draft only,
//     published articles keep their frozen original text;
//   - failed/skipped generation can be retried atomically.
type KnowledgeDraftService struct {
	db           *gorm.DB
	articleRepo  *repository.CareArticleRepository
	questionRepo *repository.QuestionRepository
	answerRepo   *repository.AnswerRepository
	logger       *slog.Logger
}

// NewKnowledgeDraftService creates a KnowledgeDraftService.
func NewKnowledgeDraftService(db *gorm.DB, articleRepo *repository.CareArticleRepository,
	questionRepo *repository.QuestionRepository, answerRepo *repository.AnswerRepository, logger *slog.Logger) *KnowledgeDraftService {
	return &KnowledgeDraftService{
		db:           db,
		articleRepo:  articleRepo,
		questionRepo: questionRepo,
		answerRepo:   answerRepo,
		logger:       logger,
	}
}

// GetDraftForQuestion returns the Q&A-sedimented article (draft, invalidated
// or published) for a question. Only the question owner may view it; the
// public only sees published articles through the regular article endpoints.
func (s *KnowledgeDraftService) GetDraftForQuestion(userID, questionID uint) (*model.CareArticle, error) {
	q, err := s.questionRepo.FindByID(questionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
		}
		return nil, fmt.Errorf("draft get question find: %w", err)
	}
	if q.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("Question[id=%d] draft get failed: user_id=%d not question owner", questionID, userID))
	}
	art, err := s.articleRepo.FindByQuestionID(questionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("Question[id=%d] draft not found: adoption sedimentation not generated", questionID))
		}
		return nil, fmt.Errorf("draft get article find: %w", err)
	}
	return art, nil
}

// GenerateForAdoptionTx creates the draft inside the adoption transaction so
// that a draft failure rolls back the best-answer/question update as well —
// there must never be a new best answer whose article still points at the old
// answer or is missing.
func (s *KnowledgeDraftService) GenerateForAdoptionTx(tx *gorm.DB, q *model.Question, a *model.Answer) (*model.CareArticle, error) {
	draft := BuildDraftFromAdoption(q, a)
	if err := s.articleRepo.CreateTx(tx, draft); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogArticleDraftGenerateFailed, q.ID, a.ID), "error", err)
		return nil, fmt.Errorf("adoption draft create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftGenerated, q.ID, a.ID, draft.ID))
	return draft, nil
}

// InvalidateForAnswerEditTx invalidates the unpublished draft distilled from an
// edited answer. Published articles are deliberately untouched: their frozen
// snapshots preserve the original text.
func (s *KnowledgeDraftService) InvalidateForAnswerEditTx(tx *gorm.DB, a *model.Answer) error {
	art, err := s.articleRepo.FindByAnswerIDDraftTx(tx, a.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // no live draft (already published, invalidated or never generated)
		}
		return fmt.Errorf("answer edit draft find: %w", err)
	}
	art.Status = constants.ArticleStatusInvalidated
	if err := s.articleRepo.UpdateTx(tx, art); err != nil {
		return fmt.Errorf("answer edit draft invalidate: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftInvalidated, art.ID, a.ID, a.Version))
	return nil
}

// RetryDraft regenerates the draft for a closed question. It rebuilds the
// frozen snapshots from the current best answer when the previous draft is
// invalidated or missing; an existing live draft is returned untouched and a
// published article is never overwritten.
func (s *KnowledgeDraftService) RetryDraft(userID, questionID uint) (*model.CareArticle, error) {
	var draft *model.CareArticle
	err := s.db.Transaction(func(tx *gorm.DB) error {
		q, err := s.questionRepo.FindByIDForUpdateTx(tx, questionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Question[id=%d] not found", questionID))
			}
			return fmt.Errorf("draft retry question find: %w", err)
		}
		if q.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("Question[id=%d] draft retry failed: user_id=%d not question owner", questionID, userID))
		}
		if q.Status != model.QuestionStatusClosed {
			return util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("Question[id=%d] draft retry failed: no adopted best answer yet", questionID))
		}
		a, err := s.answerRepo.FindBestForQuestionTx(tx, questionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(409, constants.CodeConflict,
					fmt.Sprintf("Question[id=%d] draft retry failed: closed question has no best answer", questionID))
			}
			return fmt.Errorf("draft retry best answer find: %w", err)
		}

		art, err := s.articleRepo.FindByQuestionIDTx(tx, questionID)
		switch {
		case err == nil:
			switch art.Status {
			case constants.ArticleStatusPublished:
				return util.NewAppError(409, constants.CodeConflict,
					fmt.Sprintf("CareArticle[id=%d] draft retry failed: article already published, original version is retained", art.ID))
			case constants.ArticleStatusDraft:
				draft = art
				return nil
			}
			// invalidated: rebuild from the current question/best answer.
			RebuildDraftFromAdoption(art, q, a)
			if err := s.articleRepo.UpdateTx(tx, art); err != nil {
				return fmt.Errorf("draft retry rebuild: %w", err)
			}
			draft = art
		case errors.Is(err, repository.ErrNotFound):
			// The adoption transaction rolled back before or the draft was lost.
			draft = BuildDraftFromAdoption(q, a)
			if err := s.articleRepo.CreateTx(tx, draft); err != nil {
				return fmt.Errorf("draft retry create: %w", err)
			}
		default:
			return fmt.Errorf("draft retry article find: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticleDraftRegenerated, questionID, draft.ID))
	return draft, nil
}

// PublishDraft turns an owner's Q&A draft into a public care article. The
// owner must pick a care topic tag at publish time. Invalidated drafts must be
// regenerated first and a published article is immutable through this path.
func (s *KnowledgeDraftService) PublishDraft(userID, articleID uint, topicTag string) (*model.CareArticle, error) {
	if !constants.IsValidTopicTag(topicTag) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareArticle[id=%d] publish failed: invalid topic tag=%s", articleID, topicTag))
	}
	var art *model.CareArticle
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		art, err = s.articleRepo.FindByIDForUpdateTx(tx, articleID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareArticle[id=%d] not found", articleID))
			}
			return fmt.Errorf("draft publish find: %w", err)
		}
		if art.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("CareArticle[id=%d] publish failed: user_id=%d is not owner", articleID, userID))
		}
		switch art.Status {
		case constants.ArticleStatusPublished:
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("CareArticle[id=%d] publish failed: already published", articleID))
		case constants.ArticleStatusInvalidated:
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("CareArticle[id=%d] publish failed: draft invalidated by answer edit, please regenerate", articleID))
		}
		art.TopicTag = topicTag
		art.Status = constants.ArticleStatusPublished
		if err := s.articleRepo.UpdateTx(tx, art); err != nil {
			return fmt.Errorf("draft publish update: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogArticlePublished, articleID))
	return art, nil
}

// BuildDraftFromAdoption assembles the draft for an adoption, freezing the
// question and answer exactly as they are at adoption time.
func BuildDraftFromAdoption(q *model.Question, a *model.Answer) *model.CareArticle {
	title := draftTitlePrefix + q.Title
	if len(title) > 255 {
		title = title[:255]
	}
	return &model.CareArticle{
		UserID:           q.UserID,
		Title:            title,
		Content:          buildDraftContent(q, a),
		TopicTag:         "",
		Status:           constants.ArticleStatusDraft,
		SourceQuestionID: q.ID,
		SourceAnswerID:   a.ID,
		QuestionSnapshot: freezeQuestion(q),
		AnswerSnapshot:   a.Content,
		AnswerVersion:    a.Version,
	}
}

// RebuildDraftFromAdoption refreshes an existing (invalidated) draft with the
// current question/best answer snapshots.
func RebuildDraftFromAdoption(art *model.CareArticle, q *model.Question, a *model.Answer) {
	title := draftTitlePrefix + q.Title
	if len(title) > 255 {
		title = title[:255]
	}
	art.UserID = q.UserID
	art.Title = title
	art.Content = buildDraftContent(q, a)
	art.SourceQuestionID = q.ID
	art.SourceAnswerID = a.ID
	art.QuestionSnapshot = freezeQuestion(q)
	art.AnswerSnapshot = a.Content
	art.AnswerVersion = a.Version
	art.Status = constants.ArticleStatusDraft
}

func buildDraftContent(q *model.Question, a *model.Answer) string {
	return fmt.Sprintf("【问题】%s\n\n%s\n\n【最佳回答】\n%s", q.Title, q.Content, a.Content)
}

func freezeQuestion(q *model.Question) string {
	return fmt.Sprintf("%s\n\n%s", q.Title, q.Content)
}
