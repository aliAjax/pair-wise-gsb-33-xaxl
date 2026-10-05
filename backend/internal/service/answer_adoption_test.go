package service

import (
	"database/sql/driver"
	"errors"
	"log/slog"
	"os"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

func newSilentDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, mock
}

func res(lastID, rows int64) driver.Result { return sqlmock.NewResult(lastID, rows) }

func questionRows(id, userID uint, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "title", "content", "images", "status"}).
		AddRow(id, userID, "月季黄叶怎么办", "叶子边缘发黄", "[]", status)
}

func answerRows(id, qid, uid uint, content string, best bool, version int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "question_id", "user_id", "content", "is_best", "like_count", "version"}).
		AddRow(id, qid, uid, content, best, 0, version)
}

func draftRows(id, userID, qid, aid uint, status string, answerVersion int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "title", "content", "cover", "topic_tag", "status",
		"view_count", "source_question_id", "source_answer_id", "question_snapshot", "answer_snapshot", "answer_version"}).
		AddRow(id, userID, "【问答沉淀】月季黄叶怎么办", "旧草稿内容", "", "", status,
			0, qid, aid, "快照问题", "旧回答", answerVersion)
}

const lockQuestionSQL = "SELECT * FROM `questions` WHERE `questions`.`id` = ? ORDER BY `questions`.`id` LIMIT ? FOR UPDATE"
const lockAnswerSQL = "SELECT * FROM `answers` WHERE `answers`.`id` = ? ORDER BY `answers`.`id` LIMIT ? FOR UPDATE"

func newAdoptionBundle(db *gorm.DB) (*AnswerService, *KnowledgeDraftService) {
	qRepo := repository.NewQuestionRepository(db)
	aRepo := repository.NewAnswerRepository(db)
	artRepo := repository.NewCareArticleRepository(db)
	draftSvc := NewKnowledgeDraftService(db, artRepo, qRepo, aRepo, newTestLogger())
	return NewAnswerService(db, aRepo, qRepo, draftSvc, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))), draftSvc
}

// TestAdoptGeneratesDraftAtomically: first adoption closes the question, marks
// the best answer and inserts a draft inside one committed transaction.
func TestAdoptGeneratesDraftAtomically(t *testing.T) {
	db, mock := newSilentDB(t)
	svc, _ := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "open"))
	mock.ExpectQuery(regexp.QuoteMeta(lockAnswerSQL)).
		WithArgs(uint(2), 1).WillReturnRows(answerRows(2, 1, 9, "见干见湿浇水", false, 1))
	mock.ExpectExec("UPDATE `answers` SET `is_best`=").WillReturnResult(res(0, 2))
	mock.ExpectExec("UPDATE `answers` SET").WillReturnResult(res(0, 1))
	mock.ExpectExec("UPDATE `questions` SET").WillReturnResult(res(0, 1))
	mock.ExpectExec("INSERT INTO `care_articles`").
		WillReturnResult(res(50, 1))
	mock.ExpectCommit()

	a, draft, err := svc.Adopt(1, 1, 2)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if !a.IsBest || draft.ID != 50 || draft.SourceQuestionID != 1 || draft.SourceAnswerID != 2 {
		t.Fatalf("unexpected adoption result: answer=%+v draft=%+v", a, draft)
	}
	if draft.Status != constants.ArticleStatusDraft {
		t.Fatalf("draft status = %q, want draft", draft.Status)
	}
	if draft.QuestionSnapshot == "" || draft.AnswerSnapshot != "见干见湿浇水" {
		t.Fatalf("frozen snapshots wrong: %+v", draft)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAdoptRacesSecondWindowLoses: two windows adopting different answers at
// once. The second sees the question already closed (row lock serializes the
// transactions) and gets 409; nothing is written or committed by the loser.
func TestAdoptRacesSecondWindowLoses(t *testing.T) {
	db, mock := newSilentDB(t)
	svc, _ := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "closed"))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(1, 1, 3)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 409 || appErr.Code != constants.CodeConflict {
		t.Fatalf("want 409 conflict for closed question, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestAdoptDraftFailureRollsBack: a draft insert error must roll back the
// best-answer and question updates too, so no half-sedimented state remains.
func TestAdoptDraftFailureRollsBack(t *testing.T) {
	db, mock := newSilentDB(t)
	svc, _ := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "open"))
	mock.ExpectQuery(regexp.QuoteMeta(lockAnswerSQL)).
		WithArgs(uint(2), 1).WillReturnRows(answerRows(2, 1, 9, "见干见湿浇水", false, 1))
	mock.ExpectExec("UPDATE `answers` SET `is_best`=").WillReturnResult(res(0, 2))
	mock.ExpectExec("UPDATE `answers` SET").WillReturnResult(res(0, 1))
	mock.ExpectExec("UPDATE `questions` SET").WillReturnResult(res(0, 1))
	mock.ExpectExec("INSERT INTO `care_articles`").WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	_, _, err := svc.Adopt(1, 1, 2)
	if err == nil {
		t.Fatal("want error when draft insert fails")
	}
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		t.Fatalf("draft failure must surface as wrapped infra error, not app error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestUpdateBestAnswerInvalidatesDraftOnly: editing an adopted answer bumps
// its version and invalidates the live draft in one transaction; a published
// article would not be found by the draft lookup and stays untouched.
func TestUpdateBestAnswerInvalidatesDraftOnly(t *testing.T) {
	db, mock := newSilentDB(t)
	svc, _ := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockAnswerSQL)).
		WithArgs(uint(2), 1).WillReturnRows(answerRows(2, 1, 9, "旧回答", true, 1))
	mock.ExpectExec("UPDATE `answers` SET").WillReturnResult(res(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE source_answer_id = ? AND status = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(2), constants.ArticleStatusDraft, 1).WillReturnRows(draftRows(50, 1, 1, 2, constants.ArticleStatusDraft, 1))
	mock.ExpectExec("UPDATE `care_articles` SET").WillReturnResult(res(0, 1))
	mock.ExpectCommit()

	a, err := svc.Update(9, 2, "新回答内容")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if a.Content != "新回答内容" || a.Version != 2 {
		t.Fatalf("answer not bumped: %+v", a)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestUpdateBestAnswerPublishedArticleUntouched: when the article was already
// published, editing the answer performs no article update at all.
func TestUpdateBestAnswerPublishedArticleUntouched(t *testing.T) {
	db, mock := newSilentDB(t)
	svc, _ := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockAnswerSQL)).
		WithArgs(uint(2), 1).WillReturnRows(answerRows(2, 1, 9, "旧回答", true, 1))
	mock.ExpectExec("UPDATE `answers` SET").WillReturnResult(res(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE source_answer_id = ? AND status = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(2), constants.ArticleStatusDraft, 1).
		WillReturnRows(sqlmock.NewRows(nil))
	mock.ExpectCommit()

	if _, err := svc.Update(9, 2, "新回答内容"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestRetryDraftRebuildsInvalidated: retry locks question + best answer, finds
// an invalidated draft and rebuilds it atomically from the current answer.
func TestRetryDraftRebuildsInvalidated(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "closed"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE question_id = ? AND is_best = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint(1), true, 1).WillReturnRows(answerRows(2, 1, 9, "新回答内容", true, 2))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE source_question_id = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(1), 1).WillReturnRows(draftRows(50, 1, 1, 2, constants.ArticleStatusInvalidated, 1))
	mock.ExpectExec("UPDATE `care_articles` SET").WillReturnResult(res(0, 1))
	mock.ExpectCommit()

	draft, err := draftSvc.RetryDraft(1, 1)
	if err != nil {
		t.Fatalf("RetryDraft: %v", err)
	}
	if draft.Status != constants.ArticleStatusDraft {
		t.Fatalf("rebuilt draft status = %q, want draft", draft.Status)
	}
	if draft.AnswerSnapshot != "新回答内容" || draft.AnswerVersion != 2 {
		t.Fatalf("rebuilt draft keeps stale snapshot: %+v", draft)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestRetryDraftKeepsPublishedOriginal: a published article is never
// overwritten by a retry; the owner receives 409.
func TestRetryDraftKeepsPublishedOriginal(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "closed"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `answers` WHERE question_id = ? AND is_best = ? ORDER BY `answers`.`id` LIMIT ?")).
		WithArgs(uint(1), true, 1).WillReturnRows(answerRows(2, 1, 9, "新回答内容", true, 2))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE source_question_id = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(1), 1).WillReturnRows(draftRows(50, 1, 1, 2, constants.ArticleStatusPublished, 1))
	mock.ExpectRollback()

	_, err := draftSvc.RetryDraft(1, 1)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 409 {
		t.Fatalf("want 409 for published article retry, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestRetryDraftOnOpenQuestionRejected: retry before adoption is invalid.
func TestRetryDraftOnOpenQuestionRejected(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockQuestionSQL)).
		WithArgs(uint(1), 1).WillReturnRows(questionRows(1, 1, "open"))
	mock.ExpectRollback()

	_, err := draftSvc.RetryDraft(1, 1)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 422 {
		t.Fatalf("want 422 for open question retry, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
