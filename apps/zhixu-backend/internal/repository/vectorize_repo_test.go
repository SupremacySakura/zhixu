package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"zhixu-backend/internal/model"
)

func TestVectorUpsertFencedBeforeExternalWrite(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "live lock order", true: "deleted task prevents external write"}[deleted], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"id"})
			if !deleted {
				rows.AddRow(1)
			}
			mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*lease_token.*FOR UPDATE").WillReturnRows(rows)
			if deleted {
				mock.ExpectRollback()
			} else {
				mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
				mock.ExpectExec("UPDATE `vectorize_tasks`.*lease_expires_at").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			called := false
			err := (&VectorizeRepository{DB: db}).WithLiveTask(context.Background(), &model.VectorizeTask{ID: 1, KnowledgeID: 9, UserID: 7, LeaseToken: "token"}, func() error { called = true; return nil })
			if deleted {
				require.ErrorIs(t, err, ErrKnowledgeConflict)
				require.False(t, called)
			} else {
				require.NoError(t, err)
				require.True(t, called)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVectorClaimRecoversOnlyHistoricalUnconfiguredFailure(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*next_attempt_at.*model_id = 0 AND collection = ''.*vector_error = .*FOR UPDATE SKIP LOCKED").WillReturnRows(sqlmock.NewRows([]string{"id", "knowledge_id", "user_id", "status", "model_id", "collection"}).AddRow(1, 9, 7, "partial", 0, ""))
	mock.ExpectExec("UPDATE `knowledge_chunks`.*state").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `vectorize_tasks`.*lease_token").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `knowledge_items`.*vector_state").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	task, err := (&VectorizeRepository{DB: db}).Claim(context.Background(), "token", time.Now())
	require.NoError(t, err)
	require.Equal(t, model.VectorRunning, task.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVectorWaitRetainsPendingWithoutFinishingChunks(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectExec("UPDATE `vectorize_tasks`.*next_attempt_at").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `knowledge_items`.*vector_error.*vector_state").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `vectorize_tasks`.*lease_expires_at").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := (&VectorizeRepository{DB: db}).Wait(context.Background(), &model.VectorizeTask{ID: 1, KnowledgeID: 9, UserID: 7, LeaseToken: "token"}, "waiting")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVectorRequeueResetsFailedChunksOnly(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "knowledge_id", "user_id", "status"}).AddRow(1, 9, 7, "partial"))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "vector_state"}).AddRow(9, "ready", "partial"))
	mock.ExpectExec("UPDATE `vectorize_tasks` SET .* WHERE id = . AND status = ").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `knowledge_chunks` SET .*state").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `knowledge_items` SET .*chunks_failed.*vector_state").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := (&VectorizeRepository{DB: db}).Requeue(context.Background(), 7, 9)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVectorRequeueRejectsNonPartialTask(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "knowledge_id", "user_id", "status"}).AddRow(1, 9, 7, "ready"))
	mock.ExpectRollback()
	err := (&VectorizeRepository{DB: db}).Requeue(context.Background(), 7, 9)
	require.ErrorIs(t, err, ErrKnowledgeConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVectorRequeueRejectsConcurrentStateChange(t *testing.T) {
	// 双击或 worker 竞态：任务仍为 partial，但条目状态已不在 partial。
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "knowledge_id", "user_id", "status"}).AddRow(1, 9, 7, "partial"))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "vector_state"}).AddRow(9, "ready", "pending"))
	mock.ExpectRollback()
	err := (&VectorizeRepository{DB: db}).Requeue(context.Background(), 7, 9)
	require.ErrorIs(t, err, ErrKnowledgeConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
