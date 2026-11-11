package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// UpdateMetadata 只允许写 summary / keywords（外加 updated_at），正文、版本号与向量状态保持不变。
func TestUpdateMetadataTouchesOnlyMetadataColumns(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .id. FROM .knowledge_items. WHERE id = \\? AND user_id = \\?").
		WithArgs(uint(5), uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectBegin()
	// SET 只包含 keywords、summary 与 updated_at：正文、版本号、向量状态都不在更新列表里。
	mock.ExpectExec("UPDATE .knowledge_items. SET .keywords.=.+,.summary.=.+ WHERE id = .+ AND user_id = .+").
		WithArgs(`["分块","检索"]`, "一段摘要", sqlmock.AnyArg(), uint(5), uint(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, NewKnowledgeRepository(db).UpdateMetadata(context.Background(), 2, 5, "一段摘要", `["分块","检索"]`))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateMetadataMissingItem(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .id. FROM .knowledge_items. WHERE id = \\? AND user_id = \\?").
		WithArgs(uint(5), uint(2), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	err := NewKnowledgeRepository(db).UpdateMetadata(context.Background(), 2, 5, "摘要", "")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
