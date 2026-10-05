package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// CareArticleRepository handles persistence of care articles.
type CareArticleRepository struct {
	db *gorm.DB
}

// NewCareArticleRepository creates a CareArticleRepository.
func NewCareArticleRepository(db *gorm.DB) *CareArticleRepository {
	return &CareArticleRepository{db: db}
}

// Create inserts an article.
func (r *CareArticleRepository) Create(a *model.CareArticle) error {
	return r.db.Create(a).Error
}

// CreateTx inserts an article within an outer transaction (e.g. adoption that
// must atomically close the question, mark the best answer and add a draft).
func (r *CareArticleRepository) CreateTx(tx *gorm.DB, a *model.CareArticle) error {
	return tx.Create(a).Error
}

// FindByID locates an article by id.
func (r *CareArticleRepository) FindByID(id uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByIDForUpdateTx locates an article within a transaction with a row lock
// (used by draft regeneration/publish to serialize lifecycle transitions).
func (r *CareArticleRepository) FindByIDForUpdateTx(tx *gorm.DB, id uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByQuestionIDTx returns the care article distilled from a question (at
// most one exists per question), taking a lock within the transaction.
func (r *CareArticleRepository) FindByQuestionIDTx(tx *gorm.DB, questionID uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_question_id = ?", questionID).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByQuestionID returns the care article distilled from a question (at
// most one exists per question).
func (r *CareArticleRepository) FindByQuestionID(questionID uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := r.db.Where("source_question_id = ?", questionID).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByAnswerIDDraftTx returns the live (unpublished) draft distilled from an
// answer. Published and invalidated articles are intentionally not matched so
// that editing an answer never alters preserved content.
func (r *CareArticleRepository) FindByAnswerIDDraftTx(tx *gorm.DB, answerID uint) (*model.CareArticle, error) {
	var a model.CareArticle
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_answer_id = ? AND status = ?", answerID, constants.ArticleStatusDraft).
		First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListMine paginates over all articles owned by a user, optionally filtered by
// status (used for the owner's draft box, including invalidated drafts).
func (r *CareArticleRepository) ListMine(userID uint, status string, page, pageSize int) ([]model.CareArticle, int64, error) {
	var items []model.CareArticle
	var total int64
	q := r.db.Model(&model.CareArticle{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update persists an article.
func (r *CareArticleRepository) Update(a *model.CareArticle) error {
	return r.db.Save(a).Error
}

// UpdateTx persists an article within an outer transaction.
func (r *CareArticleRepository) UpdateTx(tx *gorm.DB, a *model.CareArticle) error {
	return tx.Save(a).Error
}

// Delete removes an article by id.
func (r *CareArticleRepository) Delete(id uint) error {
	res := r.db.Delete(&model.CareArticle{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementView bumps the view count of an article.
func (r *CareArticleRepository) IncrementView(id uint) error {
	return r.db.Model(&model.CareArticle{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// List filters articles by topic tag and keyword with pagination.
func (r *CareArticleRepository) List(topicTag, keyword string, page, pageSize int) ([]model.CareArticle, int64, error) {
	var items []model.CareArticle
	var total int64
	q := r.db.Model(&model.CareArticle{}).Where("status = ?", constants.ArticleStatusPublished)
	if topicTag != "" {
		q = q.Where("topic_tag = ?", topicTag)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListLatest returns the latest published articles for the home page.
func (r *CareArticleRepository) ListLatest(limit int) ([]model.CareArticle, error) {
	var items []model.CareArticle
	if err := r.db.Where("status = ?", constants.ArticleStatusPublished).
		Order("id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
