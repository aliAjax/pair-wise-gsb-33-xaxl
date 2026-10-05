package service

import (
	"errors"
	"regexp"
	"testing"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// TestPublishDraftRequiresRegeneration: an invalidated draft cannot be
// published until it is regenerated from the edited answer.
func TestPublishDraftInvalidatedRejected(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE `care_articles`.`id` = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(50), 1).WillReturnRows(draftRows(50, 1, 1, 2, constants.ArticleStatusInvalidated, 1))
	mock.ExpectRollback()

	_, err := draftSvc.PublishDraft(1, 50, constants.TopicTagPruning)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 409 {
		t.Fatalf("want 409 publishing invalidated draft, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestPublishDraftSetsTopicAndStatus: a valid draft becomes published with the
// chosen topic tag while snapshots stay frozen.
func TestPublishDraftSuccess(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_articles` WHERE `care_articles`.`id` = ? ORDER BY `care_articles`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint(50), 1).WillReturnRows(draftRows(50, 1, 1, 2, constants.ArticleStatusDraft, 2))
	mock.ExpectExec("UPDATE `care_articles` SET").WillReturnResult(res(0, 1))
	mock.ExpectCommit()

	art, err := draftSvc.PublishDraft(1, 50, constants.TopicTagPruning)
	if err != nil {
		t.Fatalf("PublishDraft: %v", err)
	}
	if art.Status != constants.ArticleStatusPublished || art.TopicTag != constants.TopicTagPruning {
		t.Fatalf("unexpected published article: %+v", art)
	}
	if art.AnswerSnapshot != "旧回答" || art.SourceAnswerID != 2 {
		t.Fatalf("published article lost frozen snapshot: %+v", art)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestPublishDraftBadTopicTag rejected before touching the database.
func TestPublishDraftBadTopicTag(t *testing.T) {
	db, mock := newSilentDB(t)
	_, draftSvc := newAdoptionBundle(db)

	_, err := draftSvc.PublishDraft(1, 50, "watering")
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 422 {
		t.Fatalf("want 422 for bad topic tag, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
