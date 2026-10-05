package service

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

var (
	articleByIDSQL  = regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE `care_articles`.`id` = ? ORDER BY `care_articles`.`id` LIMIT ?")
	saveArticleSQL  = regexp.QuoteMeta("UPDATE `care_articles` SET")
	viewCountSQL    = regexp.QuoteMeta("UPDATE `care_articles` SET `view_count`")
	articleListSQL  = regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE user_id = ?")
	articleCountSQL = regexp.QuoteMeta("SELECT count(*) FROM `care_articles` WHERE user_id = ?")
)

func newCareArticleService(db *gorm.DB) *CareArticleService {
	return NewCareArticleService(repository.NewCareArticleRepository(db), newTestLogger())
}

func articleRow(id, userID uint, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "title", "content", "cover", "topic_tag", "status", "view_count", "question_id", "answer_id", "answer_version", "created_at", "updated_at"}).
		AddRow(id, userID, "月季黄叶怎么办", "## 问题\n\n……", "", constants.TopicTagQA, status, 0, 7, 9, 1, time.Now(), time.Now())
}

// 未发布的沉淀草稿不对公众开放，只能由作者在"我的文章"中续作。
func TestCareArticleServiceGetDraftNotPublic(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleByIDSQL).WillReturnRows(articleRow(5, 42, constants.ArticleStatusDraft))

	_, err := svc.Get(5)
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 404 {
		t.Fatalf("expected 404 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 已发布文章正常公开阅读并累计阅读量。
func TestCareArticleServiceGetPublished(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleByIDSQL).WillReturnRows(articleRow(5, 42, constants.ArticleStatusPublished))
	mock.ExpectBegin()
	mock.ExpectExec(viewCountSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	a, err := svc.Get(5)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.ViewCount != 1 {
		t.Errorf("expected view count incremented to 1, got %d", a.ViewCount)
	}
}

// 草稿经 draft→published 流转发布后保留原版快照。
func TestCareArticleServiceUpdatePublishDraft(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleByIDSQL).WillReturnRows(articleRow(5, 42, constants.ArticleStatusDraft))
	mock.ExpectBegin()
	mock.ExpectExec(saveArticleSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.Update(5, 42, &model.CareArticle{Status: constants.ArticleStatusPublished})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != constants.ArticleStatusPublished {
		t.Errorf("expected published, got %s", updated.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 已失效草稿被冻结：不可编辑也不可发布，只能删除。
func TestCareArticleServiceUpdateInvalidFrozen(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleByIDSQL).WillReturnRows(articleRow(5, 42, constants.ArticleStatusInvalid))

	_, err := svc.Update(5, 42, &model.CareArticle{Title: "试图改回", Status: constants.ArticleStatusPublished})
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 409 {
		t.Fatalf("expected 409 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 已发布文章不允许回退为草稿。
func TestCareArticleServiceUpdateInvalidTransition(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleByIDSQL).WillReturnRows(articleRow(5, 42, constants.ArticleStatusPublished))

	_, err := svc.Update(5, 42, &model.CareArticle{Status: constants.ArticleStatusDraft})
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 409 {
		t.Fatalf("expected 409 AppError, got %v", err)
	}
}

// 我的文章列表包含全部状态，供作者续作草稿。
func TestCareArticleServiceListMine(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newCareArticleService(db)

	mock.ExpectQuery(articleCountSQL).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(articleListSQL).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).
		AddRow(6, 42, constants.ArticleStatusDraft).
		AddRow(5, 42, constants.ArticleStatusPublished))

	items, total, err := svc.ListMine(42, 1, 10)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Errorf("unexpected result: total=%d len=%d", total, len(items))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
