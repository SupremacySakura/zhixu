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

func aiTaskColumns() []string {
	return []string{
		"id", "user_id", "task_type", "target_id", "status", "payload", "error_message",
		"retry_count", "dedupe_key", "lease_token", "lease_expires_at", "next_run_at",
		"created_at", "updated_at",
	}
}

func TestAiTaskRepositoryEnqueueCreates(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .ai_tasks.").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()

	task, err := (&AiTaskRepository{DB: db}).Enqueue(context.Background(), model.AiTask{
		UserID: 2, TaskType: model.AiTaskTypeOrganize, TargetID: 12,
	})
	require.NoError(t, err)
	require.Equal(t, uint(7), task.ID)
	require.Equal(t, model.AiTaskPending, task.Status)
	require.Equal(t, "{}", task.Payload)
	require.NotNil(t, task.DedupeKey)
	// 幂等键带用户维度：不同用户的同号目标不会互相阻塞。
	require.Equal(t, "2:organize:12", *task.DedupeKey)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAiTaskRepositoryEnqueueReusesActiveTask(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .ai_tasks.").WillReturnError(&mysqlDriver.MySQLError{Number: 1062})
	mock.ExpectRollback()
	mock.ExpectQuery("SELECT \\* FROM .ai_tasks. WHERE user_id = \\? AND task_type = \\? AND target_id = \\? AND status IN \\(\\?,\\?\\) ORDER BY id DESC LIMIT \\?").
		WithArgs(uint(2), model.AiTaskTypeOrganize, uint(12), model.AiTaskPending, model.AiTaskRunning, 1).
		WillReturnRows(sqlmock.NewRows(aiTaskColumns()).
			AddRow(9, 2, "organize", 12, "running", "{}", "", 0, "2:organize:12", "tok", now, now, now, now))

	task, err := (&AiTaskRepository{DB: db}).Enqueue(context.Background(), model.AiTask{
		UserID: 2, TaskType: model.AiTaskTypeOrganize, TargetID: 12,
	})
	require.NoError(t, err)
	require.Equal(t, uint(9), task.ID)
	require.Equal(t, model.AiTaskRunning, task.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAiTaskRepositorySucceedRejectsStaleLease(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := (&AiTaskRepository{DB: db}).Succeed(context.Background(), &model.AiTask{ID: 3, LeaseToken: "old"}, "{}")
	require.ErrorIs(t, err, ErrAiTaskLeaseLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAiTaskRepositoryBackoff(t *testing.T) {
	require.Equal(t, 30*time.Second, AiTaskBackoff(1))
	require.Equal(t, 60*time.Second, AiTaskBackoff(2))
	require.Equal(t, 120*time.Second, AiTaskBackoff(3))
	require.Equal(t, 30*time.Second, AiTaskBackoff(0))
}

func TestAiTaskRepositoryRecycleExpired(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	// 先收敛超过重试上限的任务，再把其余过期任务退回 pending。
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET .*retry_count \\+ 1 > \\?.*").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	require.NoError(t, (&AiTaskRepository{DB: db}).RecycleExpired(context.Background(), now))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 失败重试按 30s/60s/120s 退避：前三次失败保持 pending，第 4 次才进入终态。
func TestAiTaskRepositoryFailSchedulesBackoff(t *testing.T) {
	for _, tc := range []struct {
		name       string
		retryCount int
		want       time.Duration
	}{
		{"第一次失败", 0, 30 * time.Second},
		{"第二次失败", 1, 60 * time.Second},
		{"第三次失败", 2, 120 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			now := time.Now()
			task := &model.AiTask{ID: 5, UserID: 2, RetryCount: tc.retryCount, LeaseToken: "tok"}
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .ai_tasks. SET").
				WithArgs("boom", nil, "", now.Add(tc.want), tc.retryCount+1, model.AiTaskPending, sqlmock.AnyArg(), uint(5), model.AiTaskRunning, "tok").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			require.NoError(t, (&AiTaskRepository{DB: db}).Fail(context.Background(), task, "boom", true, now))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 第 4 次失败（首次执行 + 3 次重试之后）终止任务并释放幂等键。
func TestAiTaskRepositoryFailStopsAfterThreeRetries(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	task := &model.AiTask{ID: 5, UserID: 2, RetryCount: 3, LeaseToken: "tok"}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET").
		WithArgs(nil, "boom", nil, "", 4, model.AiTaskFailed, sqlmock.AnyArg(), uint(5), model.AiTaskRunning, "tok").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, (&AiTaskRepository{DB: db}).Fail(context.Background(), task, "boom", true, now))
	require.NoError(t, mock.ExpectationsWereMet())
}

// HasTaskForTarget 统计该知识在任意状态下的任务，供自动入队判断是否已经跑过。
func TestAiTaskRepositoryHasTaskForTarget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		want  bool
	}{
		{"已有任务", 1, true},
		{"从未有任务", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectQuery("SELECT count.*FROM .ai_tasks.").
				WithArgs(uint(2), model.AiTaskTypeRelation, uint(12)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))

			found, err := (&AiTaskRepository{DB: db}).HasTaskForTarget(context.Background(), 2, model.AiTaskTypeRelation, 12)
			require.NoError(t, err)
			require.Equal(t, tc.want, found)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAiTaskRepositoryUpdatePayload(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET .payload.=.,.updated_at.=.+ WHERE id = .+ AND user_id = .+").
		WithArgs(`{"accepted_at":"2026-11-11T13:00:00Z"}`, sqlmock.AnyArg(), uint(9), uint(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := (&AiTaskRepository{DB: db}).UpdatePayload(context.Background(), 2, 9, `{"accepted_at":"2026-11-11T13:00:00Z"}`)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAiTaskRepositoryUpdatePayloadMissingTask(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .ai_tasks. SET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := (&AiTaskRepository{DB: db}).UpdatePayload(context.Background(), 2, 99, "{}")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
