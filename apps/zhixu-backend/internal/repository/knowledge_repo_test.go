package repository

import (
	"bytes"
	"context"
	"log"
	"regexp"
	"testing"
	"time"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestKnowledgeClaimIdleQueueAndDatabaseErrorLogs(t *testing.T) {
	for _, databaseError := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle queue is silent", true: "database errors remain visible"}[databaseError], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			var output bytes.Buffer
			db = db.Session(&gorm.Session{Logger: logger.New(log.New(&output, "", 0), logger.Config{
				LogLevel: logger.Error,
			})})
			now := time.Now()
			mock.ExpectBegin()
			query := mock.ExpectQuery("SELECT .* FROM `knowledge_import_tasks` WHERE .*lease_expires_at.*ORDER BY id ASC LIMIT .*FOR UPDATE SKIP LOCKED").
				WithArgs(model.KnowledgePending, model.KnowledgeRunning, now, 1)
			if databaseError {
				query.WillReturnError(assertionError{})
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"id"}))
			}
			mock.ExpectRollback()

			_, err := NewKnowledgeRepository(db).Claim(context.Background(), "token", now)
			if databaseError {
				require.ErrorContains(t, err, "test database failure")
				require.Contains(t, output.String(), "test database failure")
			} else {
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				require.Empty(t, output.String())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestKnowledgeCreateAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "task failure rolls back"}[fail], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `file_models` .*FOR UPDATE").
				WithArgs(uint(1), uint(2), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectExec("INSERT INTO `knowledge_items`").WillReturnResult(sqlmock.NewResult(10, 1))
			if fail {
				mock.ExpectExec("INSERT INTO `knowledge_import_tasks`").WillReturnError(assertionError{})
				mock.ExpectRollback()
			} else {
				mock.ExpectExec("INSERT INTO `knowledge_import_tasks`").WillReturnResult(sqlmock.NewResult(20, 1))
				mock.ExpectCommit()
			}
			item, created, err := NewKnowledgeRepository(db).Create(context.Background(), &model.KnowledgeItem{UserID: 2, SourceFileID: 1, Status: model.KnowledgePending})
			if fail {
				require.Error(t, err)
				require.False(t, created)
			} else {
				require.NoError(t, err)
				require.True(t, created)
				require.Equal(t, uint(10), item.ID)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestKnowledgeDeleteAtomicAndScoped(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "deletes task then item", true: "item failure rolls back task deletion"}[fail], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectExec("DELETE FROM `knowledge_import_tasks` WHERE user_id = .*knowledge_id IN .*user_id").
				WithArgs(uint(2), uint(10), uint(2)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))
			mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
			mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
			for _, table := range []string{"vectorize_tasks", "knowledge_chunks"} {
				mock.ExpectExec("DELETE FROM `"+table+"` WHERE user_id = .*knowledge_id IN .*user_id").
					WithArgs(uint(2), uint(10), uint(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			query := mock.ExpectExec("DELETE FROM `knowledge_items` WHERE id = .*user_id").
				WithArgs(uint(10), uint(2))
			if fail {
				query.WillReturnError(assertionError{})
				mock.ExpectRollback()
			} else {
				query.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err := NewKnowledgeRepository(db).Delete(context.Background(), 2, 10)
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestKnowledgeCreateRejectsDeletedSource(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `file_models` .*deleted_at.*FOR UPDATE").
		WithArgs(uint(1), uint(2), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	_, _, err := NewKnowledgeRepository(db).Create(context.Background(), &model.KnowledgeItem{UserID: 2, SourceFileID: 1})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

type assertionError struct{}

func (assertionError) Error() string { return "test database failure" }

func TestKnowledgeCreateConcurrentWinner(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `file_models` .*FOR UPDATE").
		WithArgs(uint(1), uint(2), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec("INSERT INTO `knowledge_items`").WillReturnError(&mysqlDriver.MySQLError{Number: 1062})
	mock.ExpectRollback()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `knowledge_items` WHERE user_id = ? AND source_file_id = ? LIMIT ?")).
		WithArgs(uint(2), uint(1), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow(99, "winner"))
	item, created, err := NewKnowledgeRepository(db).Create(context.Background(), &model.KnowledgeItem{UserID: 2, SourceFileID: 1})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, uint(99), item.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestKnowledgeOwnershipQueries(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	r := NewKnowledgeRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `knowledge_items` WHERE id = ? AND user_id = ? LIMIT ?")).
		WithArgs(uint(10), uint(2), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(10, 2))
	_, err := r.Get(context.Background(), 2, 10)
	require.NoError(t, err)
	mock.ExpectQuery("SELECT .* FROM `file_models` WHERE .*id = .*user_id = .*deleted_at.*IS NULL").
		WithArgs(uint(1), uint(2), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(1, 2))
	_, err = r.File(context.Background(), 2, 1)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestKnowledgeClaimExpiredLease(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `knowledge_import_tasks` WHERE .*lease_expires_at.*ORDER BY id ASC LIMIT .*FOR UPDATE SKIP LOCKED").
		WithArgs(model.KnowledgePending, model.KnowledgeRunning, now, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "knowledge_id", "user_id", "status", "attempts"}).AddRow(20, 10, 2, "running", 1))
	mock.ExpectExec("UPDATE `knowledge_import_tasks`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `knowledge_items`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	task, err := NewKnowledgeRepository(db).Claim(context.Background(), "new-token", now)
	require.NoError(t, err)
	require.Equal(t, "new-token", task.LeaseToken)
	require.Equal(t, model.KnowledgeRunning, task.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestKnowledgeFinishFencing(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "winner commits", true: "stale worker rejected"}[stale], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			affected := int64(1)
			if stale {
				affected = 0
			}
			mock.ExpectExec("UPDATE `knowledge_import_tasks` SET .*WHERE id = .*status = .*lease_token = .*").
				WillReturnResult(sqlmock.NewResult(0, affected))
			if stale {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec("UPDATE `knowledge_items`").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO `vectorize_tasks`").WillReturnResult(sqlmock.NewResult(30, 1))
				mock.ExpectExec("UPDATE `knowledge_items`.*vector_state").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err := NewKnowledgeRepository(db).Finish(context.Background(), &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2, LeaseToken: "token"}, "# text", "", "")
			if stale {
				require.ErrorIs(t, err, ErrKnowledgeConflict)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestKnowledgeRetryState(t *testing.T) {
	for _, state := range []model.KnowledgeStatus{model.KnowledgeFailed, model.KnowledgeReady} {
		t.Run(string(state), func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `knowledge_import_tasks` WHERE knowledge_id = .*user_id = .*FOR UPDATE").
				WithArgs(uint(10), uint(2), 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(20, state))
			if state == model.KnowledgeFailed {
				mock.ExpectExec("UPDATE `knowledge_import_tasks`").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE `knowledge_items`").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err := NewKnowledgeRepository(db).Retry(context.Background(), 2, 10)
			if state == model.KnowledgeFailed {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrKnowledgeConflict)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestKnowledgeListLiteralTitle(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `knowledge_items` WHERE user_id = .*LOCATE.*status =").
		WithArgs(uint(2), "%_", model.KnowledgeReady).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` WHERE user_id = .*LOCATE.*status = .*ORDER BY created_at DESC, id DESC LIMIT").
		WithArgs(uint(2), "%_", model.KnowledgeReady, 20).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	items, count, err := NewKnowledgeRepository(db).List(context.Background(), 2, 1, 20, "%_", model.KnowledgeReady)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, count)
	require.NoError(t, mock.ExpectationsWereMet())
}
