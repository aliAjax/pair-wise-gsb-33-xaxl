package service

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

func newAnswerService(db *gorm.DB) *AnswerService {
	return NewAnswerService(db,
		repository.NewAnswerRepository(db),
		repository.NewQuestionRepository(db),
		repository.NewCareArticleRepository(db),
		newTestLogger())
}

var (
	adoptCloseSQL    = regexp.QuoteMeta("UPDATE `questions` SET `status`=? WHERE id = ? AND status = ?")
	questionByIDSQL  = regexp.QuoteMeta("SELECT * FROM `questions` WHERE `questions`.`id` = ? ORDER BY `questions`.`id` LIMIT ?")
	answerByIDSQL    = regexp.QuoteMeta("SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ?")
	clearBestSQL     = regexp.QuoteMeta("UPDATE `answers` SET `is_best`=? WHERE question_id = ?")
	saveAnswerSQL    = regexp.QuoteMeta("UPDATE `answers` SET")
	insertArticleSQL = regexp.QuoteMeta("INSERT INTO `care_articles`")
	invalidateSQL    = regexp.QuoteMeta("UPDATE `care_articles` SET")
)

func questionRow(id, userID uint, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "title", "content", "images", "status", "created_at"}).
		AddRow(id, userID, "月季黄叶怎么办", "叶片边缘发黄", "[]", status, time.Now())
}

func answerRow(id, questionID, userID uint, version int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "question_id", "user_id", "content", "is_best", "like_count", "version", "created_at"}).
		AddRow(id, questionID, userID, "先缓苗再见干见湿浇水", false, 3, version, time.Now())
}

// 采纳成功：关问题 + 标最佳 + 生成草稿在同一事务提交，草稿记录回答版本快照。
func TestAnswerServiceAdoptSuccess(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectBegin()
	mock.ExpectExec(adoptCloseSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(questionByIDSQL).WillReturnRows(questionRow(7, 42, constants.QuestionStatusClosed))
	mock.ExpectQuery(answerByIDSQL).WillReturnRows(answerRow(9, 7, 1001, 2))
	mock.ExpectExec(clearBestSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(saveAnswerSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(insertArticleSQL).WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectCommit()

	a, draft, err := svc.Adopt(42, 7, 9)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if !a.IsBest {
		t.Error("expected answer marked as best")
	}
	if draft == nil {
		t.Fatal("expected sedimented draft")
	}
	if draft.ID != 100 || draft.QuestionID != 7 || draft.AnswerID != 9 {
		t.Errorf("unexpected draft linkage: %+v", draft)
	}
	if draft.Status != constants.ArticleStatusDraft || draft.TopicTag != constants.TopicTagQA {
		t.Errorf("unexpected draft state: status=%s topic=%s", draft.Status, draft.TopicTag)
	}
	if draft.AnswerVersion != 2 {
		t.Errorf("expected answer version snapshot 2, got %d", draft.AnswerVersion)
	}
	if draft.UserID != 42 {
		t.Errorf("expected draft owned by question owner 42, got %d", draft.UserID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 并发采纳：问题已被先到的事务关闭时，后到者只能拿到 409，且不产生任何写入。
func TestAnswerServiceAdoptConflictWhenAlreadyClosed(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectBegin()
	mock.ExpectExec(adoptCloseSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(questionByIDSQL).WillReturnRows(questionRow(7, 42, constants.QuestionStatusClosed))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(42, 7, 9)
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 409 {
		t.Fatalf("expected 409 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 采纳不存在的问题：条件更新 0 行且查不到问题，返回 404 并回滚。
func TestAnswerServiceAdoptQuestionNotFound(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectBegin()
	mock.ExpectExec(adoptCloseSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(questionByIDSQL).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(42, 7, 9)
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 404 {
		t.Fatalf("expected 404 AppError, got %v", err)
	}
}

// 非问题主人采纳：抢占关闭后校验归属失败，事务回滚，问题保持 open。
func TestAnswerServiceAdoptForbiddenRollsBack(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectBegin()
	mock.ExpectExec(adoptCloseSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(questionByIDSQL).WillReturnRows(questionRow(7, 42, constants.QuestionStatusClosed))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(999, 7, 9)
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 403 {
		t.Fatalf("expected 403 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 草稿生成失败：整个采纳事务回滚，不留"最佳回答已换、文章缺失"的半套数据。
func TestAnswerServiceAdoptDraftFailureRollsBack(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectBegin()
	mock.ExpectExec(adoptCloseSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(questionByIDSQL).WillReturnRows(questionRow(7, 42, constants.QuestionStatusClosed))
	mock.ExpectQuery(answerByIDSQL).WillReturnRows(answerRow(9, 7, 1001, 1))
	mock.ExpectExec(clearBestSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(saveAnswerSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(insertArticleSQL).WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(42, 7, 9)
	if err == nil {
		t.Fatal("expected adopt error when draft creation fails")
	}
	if _, ok := err.(*util.AppError); ok {
		t.Fatalf("expected raw wrapped error, got AppError: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 编辑回答：版本号 +1，同事务把该回答衍生的未发布草稿置为失效。
func TestAnswerServiceUpdateInvalidatesDrafts(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectQuery(answerByIDSQL).WillReturnRows(answerRow(9, 7, 1001, 1))
	mock.ExpectBegin()
	mock.ExpectExec(saveAnswerSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(invalidateSQL).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	a, err := svc.Update(1001, 9, "修改后的回答内容")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if a.Version != 2 {
		t.Errorf("expected version bumped to 2, got %d", a.Version)
	}
	if a.Content != "修改后的回答内容" {
		t.Errorf("unexpected content: %s", a.Content)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 非作者编辑回答：403，不开启任何事务。
func TestAnswerServiceUpdateForbidden(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectQuery(answerByIDSQL).WillReturnRows(answerRow(9, 7, 1001, 1))

	_, err := svc.Update(2002, 9, "越权修改")
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 403 {
		t.Fatalf("expected 403 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// 已关闭问题固定问答对，拒绝新回答。
func TestAnswerServiceCreateOnClosedQuestion(t *testing.T) {
	db, mock := newServiceDB(t)
	svc := newAnswerService(db)

	mock.ExpectQuery(questionByIDSQL).WillReturnRows(questionRow(7, 42, constants.QuestionStatusClosed))

	_, err := svc.Create(1001, 7, "新回答")
	appErr, ok := err.(*util.AppError)
	if !ok || appErr.HTTPStatus != 409 {
		t.Fatalf("expected 409 AppError, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
