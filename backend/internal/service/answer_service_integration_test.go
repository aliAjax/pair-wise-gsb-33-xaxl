package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// Integration tests run against a real (in-memory) SQL engine so the adopt
// transaction, the first-writer-wins race and the draft invalidation behave
// exactly as the SQL statements dictate, not as a mock expects them to.

func newIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Shared-cache in-memory database with a busy timeout so concurrent
	// transactions queue on the writer lock instead of failing spuriously.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(10000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Question{}, &model.Answer{}, &model.CareArticle{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newIntegrationAnswerService(db *gorm.DB) *AnswerService {
	return NewAnswerService(db,
		repository.NewAnswerRepository(db),
		repository.NewQuestionRepository(db),
		repository.NewCareArticleRepository(db),
		newTestLogger())
}

func seedQuestion(t *testing.T, db *gorm.DB, ownerID uint) *model.Question {
	t.Helper()
	q := &model.Question{UserID: ownerID, Title: "月季黄叶怎么办", Content: "叶片边缘发黄", Images: "[]", Status: constants.QuestionStatusOpen}
	if err := db.Create(q).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
	return q
}

func seedAnswer(t *testing.T, db *gorm.DB, questionID, userID uint, content string) *model.Answer {
	t.Helper()
	a := &model.Answer{QuestionID: questionID, UserID: userID, Content: content}
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("seed answer: %v", err)
	}
	return a
}

// 采纳成功：问题关闭、最佳回答落定、文章草稿同事务生成并记录回答版本。
func TestIntegrationAdoptGeneratesDraft(t *testing.T) {
	db := newIntegrationDB(t)
	svc := newIntegrationAnswerService(db)
	q := seedQuestion(t, db, 42)
	a := seedAnswer(t, db, q.ID, 1001, "先缓苗，见干见湿浇水")

	adopted, draft, err := svc.Adopt(42, q.ID, a.ID)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if !adopted.IsBest {
		t.Error("answer not marked best")
	}
	if draft.Status != constants.ArticleStatusDraft || draft.QuestionID != q.ID || draft.AnswerID != a.ID || draft.AnswerVersion != 1 {
		t.Errorf("unexpected draft: %+v", draft)
	}
	var gotQ model.Question
	if err := db.First(&gotQ, q.ID).Error; err != nil {
		t.Fatalf("reload question: %v", err)
	}
	if gotQ.Status != constants.QuestionStatusClosed {
		t.Errorf("question status = %s, want closed", gotQ.Status)
	}
	// 已关闭的问题拒绝新回答（问答对已固定）。
	if _, err := svc.Create(1002, q.ID, "再来一条"); err == nil {
		t.Error("expected create answer on closed question to fail")
	}
}

// 两个窗口同时采纳不同回答：只有先到的一版成为最佳回答并生成草稿，
// 后到者拿到 409，且不会留下第二个草稿或被覆盖的最佳回答。
func TestIntegrationConcurrentAdoptFirstWins(t *testing.T) {
	db := newIntegrationDB(t)
	svc := newIntegrationAnswerService(db)
	q := seedQuestion(t, db, 42)
	a1 := seedAnswer(t, db, q.ID, 1001, "回答一")
	a2 := seedAnswer(t, db, q.ID, 1002, "回答二")

	answerIDs := []uint{a1.ID, a2.ID}
	var wg sync.WaitGroup
	results := make([]error, len(answerIDs))
	start := make(chan struct{})
	for i := range answerIDs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, results[i] = svc.Adopt(42, q.ID, answerIDs[i])
		}(i)
	}
	close(start)
	wg.Wait()

	var successes, conflicts int
	for _, err := range results {
		if err == nil {
			successes++
			continue
		}
		var appErr *util.AppError
		if errors.As(err, &appErr) && appErr.HTTPStatus == 409 {
			conflicts++
			continue
		}
		t.Fatalf("unexpected adopt error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected 1 success + 1 conflict, got %d success %d conflict", successes, conflicts)
	}

	var bestCount int64
	if err := db.Model(&model.Answer{}).Where("question_id = ? AND is_best = ?", q.ID, true).Count(&bestCount).Error; err != nil {
		t.Fatalf("count best: %v", err)
	}
	if bestCount != 1 {
		t.Errorf("expected exactly 1 best answer, got %d", bestCount)
	}
	var draftCount int64
	if err := db.Model(&model.CareArticle{}).Where("question_id = ?", q.ID).Count(&draftCount).Error; err != nil {
		t.Fatalf("count drafts: %v", err)
	}
	if draftCount != 1 {
		t.Errorf("expected exactly 1 sedimented draft, got %d", draftCount)
	}
}

// 回答修改后：未发布草稿同事务失效，已发布文章保留原版快照。
func TestIntegrationAnswerEditInvalidatesDraftsOnly(t *testing.T) {
	db := newIntegrationDB(t)
	svc := newIntegrationAnswerService(db)
	q1 := seedQuestion(t, db, 42)
	q2 := seedQuestion(t, db, 42)
	a1 := seedAnswer(t, db, q1.ID, 1001, "原版回答一")
	a2 := seedAnswer(t, db, q2.ID, 1001, "原版回答二")

	_, draft1, err := svc.Adopt(42, q1.ID, a1.ID)
	if err != nil {
		t.Fatalf("Adopt q1: %v", err)
	}
	_, draft2, err := svc.Adopt(42, q2.ID, a2.ID)
	if err != nil {
		t.Fatalf("Adopt q2: %v", err)
	}
	// draft2 发布为正式文章，内容是采纳时的快照。
	articleSvc := NewCareArticleService(repository.NewCareArticleRepository(db), newTestLogger())
	if _, err := articleSvc.Update(draft2.ID, 42, &model.CareArticle{Status: constants.ArticleStatusPublished}); err != nil {
		t.Fatalf("publish draft2: %v", err)
	}

	// 回答一被作者修改：只有它的未发布草稿失效。
	if _, err := svc.Update(1001, a1.ID, "修改后的回答一"); err != nil {
		t.Fatalf("Update answer: %v", err)
	}
	var gotDraft1 model.CareArticle
	if err := db.First(&gotDraft1, draft1.ID).Error; err != nil {
		t.Fatalf("reload draft1: %v", err)
	}
	if gotDraft1.Status != constants.ArticleStatusInvalid {
		t.Errorf("draft1 status = %s, want invalid", gotDraft1.Status)
	}
	var gotA1 model.Answer
	if err := db.First(&gotA1, a1.ID).Error; err != nil {
		t.Fatalf("reload answer: %v", err)
	}
	if gotA1.Version != 2 {
		t.Errorf("answer version = %d, want 2", gotA1.Version)
	}

	// 已发布文章不受任何回答修改影响，保留原版快照。
	if _, err := svc.Update(1001, a2.ID, "修改后的回答二"); err != nil {
		t.Fatalf("Update answer2: %v", err)
	}
	var gotArticle2 model.CareArticle
	if err := db.First(&gotArticle2, draft2.ID).Error; err != nil {
		t.Fatalf("reload published: %v", err)
	}
	if gotArticle2.Status != constants.ArticleStatusPublished {
		t.Errorf("published article status = %s, want published", gotArticle2.Status)
	}
	wantContent := buildQADraftContent(q2, a2)
	if gotArticle2.Content != wantContent {
		t.Errorf("published article content rewritten:\ngot  %q\nwant %q", gotArticle2.Content, wantContent)
	}
}

// 草稿生成失败时整个采纳回滚：最佳回答未换、问题未关闭，可以原样重试。
func TestIntegrationAdoptRollsBackWhenDraftFails(t *testing.T) {
	db := newIntegrationDB(t)
	svc := newIntegrationAnswerService(db)
	q := seedQuestion(t, db, 42)
	a := seedAnswer(t, db, q.ID, 1001, "回答")

	// 删掉文章表让草稿插入必然失败，模拟草稿生成异常。
	if err := db.Migrator().DropTable(&model.CareArticle{}); err != nil {
		t.Fatalf("drop articles: %v", err)
	}
	if _, _, err := svc.Adopt(42, q.ID, a.ID); err == nil {
		t.Fatal("expected adopt to fail when draft insert fails")
	}

	var gotQ model.Question
	if err := db.First(&gotQ, q.ID).Error; err != nil {
		t.Fatalf("reload question: %v", err)
	}
	if gotQ.Status != constants.QuestionStatusOpen {
		t.Errorf("question status = %s after rollback, want open", gotQ.Status)
	}
	var gotA model.Answer
	if err := db.First(&gotA, a.ID).Error; err != nil {
		t.Fatalf("reload answer: %v", err)
	}
	if gotA.IsBest {
		t.Error("answer still marked best after rollback")
	}

	// 恢复文章表后原样重试成功，不留半套数据。
	if err := db.AutoMigrate(&model.CareArticle{}); err != nil {
		t.Fatalf("remigrate: %v", err)
	}
	if _, _, err := svc.Adopt(42, q.ID, a.ID); err != nil {
		t.Fatalf("retry Adopt: %v", err)
	}
}
