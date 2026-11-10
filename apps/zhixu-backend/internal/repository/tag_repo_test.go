package repository

import (
	"context"
	"testing"
	"time"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTagRepositoryList(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectQuery("SELECT tags\\.\\*, COUNT\\(knowledge_tags\\.knowledge_id\\) AS usage_count FROM `tags` LEFT JOIN knowledge_tags ON knowledge_tags\\.tag_id = tags\\.id AND knowledge_tags\\.user_id = tags\\.user_id WHERE tags\\.user_id = \\?").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "created_at", "usage_count"}).
			AddRow(1, 2, "工作", now, 3).AddRow(2, 2, "学习", now, 0))

	rows, err := NewTagRepository(db).List(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "工作", rows[0].Name)
	require.Equal(t, int64(3), rows[0].UsageCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRepositoryFindOrCreateReusesExisting(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `tags` WHERE user_id = \\? AND name = \\? LIMIT \\?").
		WithArgs(uint(2), "工作", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "created_at"}).AddRow(5, 2, "工作", now))

	tag, err := NewTagRepository(db).FindOrCreate(context.Background(), 2, "工作")
	require.NoError(t, err)
	require.Equal(t, uint(5), tag.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRepositoryFindOrCreateCreates(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `tags` WHERE user_id = \\? AND name = \\? LIMIT \\?").
		WithArgs(uint(2), "工作", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `tags`").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()
	// 插入成功后 GORM 会回填主键，回读因此带上 id 条件（重复冲突时 LastInsertId 为 0，则只按名称回读）。
	mock.ExpectQuery("SELECT \\* FROM `tags` WHERE \\(user_id = \\? AND name = \\?\\) AND `tags`\\.`id` = \\? LIMIT \\?").
		WithArgs(uint(2), "工作", uint(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "created_at"}).AddRow(9, 2, "工作", now))

	tag, err := NewTagRepository(db).FindOrCreate(context.Background(), 2, "工作")
	require.NoError(t, err)
	require.Equal(t, uint(9), tag.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRepositoryRenameConflict(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `tags` SET `name`=\\?").
		WithArgs("学习", uint(5), uint(2)).
		WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()

	_, err := NewTagRepository(db).Rename(context.Background(), 2, 5, "学习")
	require.ErrorIs(t, err, ErrTagNameConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRepositoryRenameMissing(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `tags` SET `name`=\\?").
		WithArgs("学习", uint(5), uint(2)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err := NewTagRepository(db).Rename(context.Background(), 2, 5, "学习")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRepositoryDelete(t *testing.T) {
	t.Run("removes tag and bindings", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM `tags` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(5), uint(2)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM `knowledge_tags` WHERE tag_id = \\? AND user_id = \\?").
			WithArgs(uint(5), uint(2)).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()

		require.NoError(t, NewTagRepository(db).Delete(context.Background(), 2, 5))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing tag rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM `tags` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(5), uint(2)).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		require.ErrorIs(t, NewTagRepository(db).Delete(context.Background(), 2, 5), gorm.ErrRecordNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTagRepositoryTagsByKnowledge(t *testing.T) {
	t.Run("returns bound tags", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		now := time.Now()
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `knowledge_items` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(10), uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT `tags`\\.`id`.*FROM `tags` JOIN knowledge_tags ON knowledge_tags\\.tag_id = tags\\.id WHERE knowledge_tags\\.user_id = \\? AND knowledge_tags\\.knowledge_id = \\? AND tags\\.user_id = \\? ORDER BY tags\\.name").
			WithArgs(uint(2), uint(10), uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "created_at"}).AddRow(1, 2, "工作", now))

		tags, err := NewTagRepository(db).TagsByKnowledge(context.Background(), 2, 10)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "工作", tags[0].Name)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing knowledge", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `knowledge_items` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(99), uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		_, err := NewTagRepository(db).TagsByKnowledge(context.Background(), 2, 99)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTagRepositoryReplaceForKnowledge(t *testing.T) {
	t.Run("replaces bindings", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		now := time.Now()
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `knowledge_items` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(10), uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectExec("DELETE FROM `knowledge_tags` WHERE user_id = \\? AND knowledge_id = \\?").
			WithArgs(uint(2), uint(10)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT \\* FROM `tags` WHERE user_id = \\? AND name = \\? LIMIT \\?").
			WithArgs(uint(2), "工作", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "created_at"}).AddRow(5, 2, "工作", now))
		mock.ExpectExec("INSERT INTO `knowledge_tags`").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tags, err := NewTagRepository(db).ReplaceForKnowledge(context.Background(), 2, 10, []string{"工作"}, model.TagSourceManual)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "工作", tags[0].Name)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing knowledge rolls back", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `knowledge_items` WHERE id = \\? AND user_id = \\?").
			WithArgs(uint(99), uint(2)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectRollback()

		_, err := NewTagRepository(db).ReplaceForKnowledge(context.Background(), 2, 99, []string{"工作"}, model.TagSourceManual)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
