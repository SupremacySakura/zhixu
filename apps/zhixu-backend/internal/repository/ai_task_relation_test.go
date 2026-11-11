package repository

import (
	"context"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// relation 任务自带 knowledge 维度的幂等键，仓储不得覆盖调用方提供的键。
func TestAiTaskRepositoryEnqueueHonorsExplicitDedupeKey(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .ai_tasks.").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	key := "relation:12"
	task, err := (&AiTaskRepository{DB: db}).Enqueue(context.Background(), model.AiTask{
		UserID: 2, TaskType: model.AiTaskTypeRelation, TargetID: 12, DedupeKey: &key,
		Payload: `{"knowledge_id":12,"base_version":3}`,
	})

	require.NoError(t, err)
	require.NotNil(t, task.DedupeKey)
	require.Equal(t, key, *task.DedupeKey)
	require.NoError(t, mock.ExpectationsWereMet())
}
