package repository

import (
	"context"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestRetrievalLogSave(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `retrieval_logs`").WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()

	repo := NewRetrievalLogRepository(db)
	err := repo.Save(context.Background(), &model.RetrievalLog{
		UserID: 7, ConversationID: "conv-1", Query: "回滚步骤", Mode: "hybrid", Hits: "[]",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
