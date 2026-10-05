package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// QuestionRepository handles persistence of questions.
type QuestionRepository struct {
	db *gorm.DB
}

// NewQuestionRepository creates a QuestionRepository.
func NewQuestionRepository(db *gorm.DB) *QuestionRepository {
	return &QuestionRepository{db: db}
}

// Create inserts a question.
func (r *QuestionRepository) Create(q *model.Question) error {
	return r.db.Create(q).Error
}

// FindByID locates a question by id.
func (r *QuestionRepository) FindByID(id uint) (*model.Question, error) {
	var q model.Question
	if err := r.db.First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &q, nil
}

// List paginates over questions.
func (r *QuestionRepository) List(page, pageSize int) ([]model.Question, int64, error) {
	var items []model.Question
	var total int64
	q := r.db.Model(&model.Question{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update persists a question (e.g. adopt answer / close).
func (r *QuestionRepository) Update(q *model.Question) error {
	return r.db.Save(q).Error
}

// UpdateTx persists a question within an outer transaction.
func (r *QuestionRepository) UpdateTx(tx *gorm.DB, q *model.Question) error {
	return tx.Save(q).Error
}

// FindByIDTx locates a question by id within an outer transaction.
func (r *QuestionRepository) FindByIDTx(tx *gorm.DB, id uint) (*model.Question, error) {
	var q model.Question
	if err := tx.First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &q, nil
}

// CloseIfOpenTx atomically flips a question from open to closed within an outer
// transaction. It reports false when the question was already closed, which is
// how concurrent adopt requests lose the race: only the first transaction to
// flip the status proceeds to mark the best answer and generate the draft.
func (r *QuestionRepository) CloseIfOpenTx(tx *gorm.DB, id uint) (bool, error) {
	res := tx.Model(&model.Question{}).
		Where("id = ? AND status = ?", id, constants.QuestionStatusOpen).
		Update("status", constants.QuestionStatusClosed)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
