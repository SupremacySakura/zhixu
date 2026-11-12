package repository

import (
	"context"
	"testing"
	"time"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestAlert(dedupeKey string) *model.EvolutionAlert {
	alert := &model.EvolutionAlert{
		UserID: 2, AlertType: model.EvolutionAlertTypeDuplicate, Status: model.EvolutionAlertPending,
		Severity: model.EvolutionSeverityReview, Payload: `{}`, DedupeKey: dedupeKey,
	}
	_ = alert.SetSubjectIDs([]uint{10, 11})
	return alert
}

// 指纹首次出现：新建一条待处理提醒。
func TestEvolutionUpsertCreatesNewAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE user_id = .*dedupe_key").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `evolution_alerts`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()

	outcome, err := NewEvolutionAlertRepository(db).Upsert(context.Background(), newTestAlert("k1"))

	require.NoError(t, err)
	require.Equal(t, EvolutionAlertCreated, outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 指纹已处于终态：抑制本次提醒，不新建也不改状态。
func TestEvolutionUpsertSuppressesResolvedAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE user_id = .*dedupe_key").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(7, 2, "resolved"))

	outcome, err := NewEvolutionAlertRepository(db).Upsert(context.Background(), newTestAlert("k1"))

	require.NoError(t, err)
	require.Equal(t, EvolutionAlertSuppressed, outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 指纹处于「暂不处理」：重新检查命中后回到待处理，并清掉上一次的裁决痕迹。
func TestEvolutionUpsertRevivesDeferredAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE user_id = .*dedupe_key").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(7, 2, "deferred"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `evolution_alerts`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	outcome, err := NewEvolutionAlertRepository(db).Upsert(context.Background(), newTestAlert("k1"))

	require.NoError(t, err)
	require.Equal(t, EvolutionAlertRevived, outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 指纹已在待处理：只刷新更新时间，不产生第二行。
func TestEvolutionUpsertRefreshesPendingAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE user_id = .*dedupe_key").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(7, 2, "pending"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `evolution_alerts`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	outcome, err := NewEvolutionAlertRepository(db).Upsert(context.Background(), newTestAlert("k1"))

	require.NoError(t, err)
	require.Equal(t, EvolutionAlertRefreshed, outcome)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 摘要只统计待办视野（待处理 + 暂不处理）。
func TestEvolutionSummaryGroupsOpenAlerts(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(`SELECT status, alert_type, COUNT\(\*\) AS count FROM `+"`evolution_alerts`"+` WHERE user_id = .*`).
		WithArgs(uint(2), "pending", "deferred").
		WillReturnRows(sqlmock.NewRows([]string{"status", "alert_type", "count"}).
			AddRow("pending", "duplicate", 3).AddRow("deferred", "conflict", 1))

	rows, err := NewEvolutionAlertRepository(db).Summary(context.Background(), 2)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, int64(3), rows[0].Count)
	require.Equal(t, model.EvolutionAlertDeferred, rows[1].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 精确重复查询：只认未归档、非自身的条目，取 ID 最小的一条。
func TestKnowledgeFindByContentHash(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` WHERE user_id = .*content_hash = .*archived_at IS NULL AND id <> .*ORDER BY id ASC LIMIT").
		WithArgs(uint(2), "hash-a", uint(10), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title"}).AddRow(11, 2, "原件"))

	item, err := NewKnowledgeRepository(db).FindByContentHash(context.Background(), 2, "hash-a", 10)

	require.NoError(t, err)
	require.Equal(t, uint(11), item.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 空指纹不查库：空正文不该和另一条空正文互相命中。
func TestKnowledgeFindByContentHashSkipsEmpty(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_, err := NewKnowledgeRepository(db).FindByContentHash(context.Background(), 2, "", 10)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 提醒列表默认按创建时间倒序，且能按类型筛选。
func TestEvolutionListFiltersByType(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	created := time.Now()
	mock.ExpectQuery(`SELECT count\(\*\) FROM `+"`evolution_alerts`"+` WHERE user_id = .*alert_type = `).
		WithArgs(uint(2), "conflict").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .* FROM `+"`evolution_alerts`"+` WHERE user_id = .*alert_type = .*ORDER BY created_at DESC, id DESC`).
		WithArgs(uint(2), "conflict", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "alert_type", "status", "created_at"}).
			AddRow(9, 2, "conflict", "pending", created))

	alerts, total, err := NewEvolutionAlertRepository(db).List(context.Background(), 2, EvolutionAlertFilter{
		Page: 1, Size: 20, Type: model.EvolutionAlertTypeConflict,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, alerts, 1)
	require.Equal(t, model.EvolutionAlertTypeConflict, alerts[0].AlertType)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 多状态筛选走 IN 条件，且按裁决时间倒序：「处理记录」页签要同时看已处理与已忽略。
func TestEvolutionListFiltersByMultipleStatuses(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	resolved := time.Now()
	mock.ExpectQuery(`SELECT count\(\*\) FROM `+"`evolution_alerts`"+` WHERE user_id = .*status IN`).
		WithArgs(uint(2), "resolved", "dismissed").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT .* FROM `+"`evolution_alerts`"+` WHERE user_id = .*status IN .*ORDER BY resolved_at DESC, id DESC`).
		WithArgs(uint(2), "resolved", "dismissed", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "alert_type", "status", "resolved_at"}).
			AddRow(9, 2, "conflict", "resolved", resolved))

	alerts, total, err := NewEvolutionAlertRepository(db).List(context.Background(), 2, EvolutionAlertFilter{
		Page: 1, Size: 20,
		Statuses: []model.EvolutionAlertStatus{model.EvolutionAlertResolved, model.EvolutionAlertDismissed},
	})

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, alerts, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 单值筛选仍是等值匹配，排序也保持创建时间倒序（与改造前完全一致）。
func TestEvolutionListFiltersBySingleStatus(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	created := time.Now()
	mock.ExpectQuery(`SELECT count\(\*\) FROM `+"`evolution_alerts`"+` WHERE user_id = .*status = `).
		WithArgs(uint(2), "resolved").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .* FROM `+"`evolution_alerts`"+` WHERE user_id = .*status = .*ORDER BY created_at DESC, id DESC`).
		WithArgs(uint(2), "resolved", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "alert_type", "status", "created_at"}).
			AddRow(9, 2, "conflict", "resolved", created))

	_, total, err := NewEvolutionAlertRepository(db).List(context.Background(), 2, EvolutionAlertFilter{
		Page: 1, Size: 20, Statuses: []model.EvolutionAlertStatus{model.EvolutionAlertResolved},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 裁决在单事务内锁行、执行副作用、写回裁决字段。
func TestEvolutionResolveWritesVerdict(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE id = .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "alert_type", "subject_ids"}).
			AddRow(7, 2, "pending", "duplicate", "[10,11]"))
	mock.ExpectExec("UPDATE `evolution_alerts` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	applied := false
	alert, changed, err := NewEvolutionAlertRepository(db).Resolve(context.Background(), 2, 7,
		model.EvolutionActionKeepBoth, "都保留", "", func(_ *gorm.DB, _ *model.EvolutionAlert) error {
			applied = true
			return nil
		})

	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, applied)
	require.Equal(t, model.EvolutionAlertResolved, alert.Status)
	require.Equal(t, model.EvolutionActionKeepBoth, alert.ResolvedAction)
	require.Equal(t, "都保留", alert.ResolutionNote)
	require.NotNil(t, alert.ResolvedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 已不在待办视野的提醒再提交：不执行副作用、不写回，返回 changed=false（幂等）。
func TestEvolutionResolveSkipsClosedAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE id = .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "alert_type", "subject_ids"}).
			AddRow(7, 2, "resolved", "duplicate", "[10,11]"))
	mock.ExpectCommit()

	applied := false
	alert, changed, err := NewEvolutionAlertRepository(db).Resolve(context.Background(), 2, 7,
		model.EvolutionActionArchiveOne, "", "", func(_ *gorm.DB, _ *model.EvolutionAlert) error {
			applied = true
			return nil
		})

	require.NoError(t, err)
	require.False(t, changed)
	require.False(t, applied)
	require.Equal(t, model.EvolutionAlertResolved, alert.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 暂不处理是状态迁移：写 deferred 且不写裁决时间。
func TestEvolutionResolveDefersAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE id = .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "alert_type", "subject_ids"}).
			AddRow(7, 2, "pending", "duplicate", "[10,11]"))
	mock.ExpectExec("UPDATE `evolution_alerts` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	alert, changed, err := NewEvolutionAlertRepository(db).Resolve(context.Background(), 2, 7,
		model.EvolutionActionDefer, "稍后看", "", nil)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.EvolutionAlertDeferred, alert.Status)
	require.Nil(t, alert.ResolvedAt, "暂不处理不算裁决")
	require.Empty(t, alert.ResolvedAction)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 「忽略」是独立的终态：处理记录里与「已处理」分两个页签，状态必须落 dismissed。
func TestEvolutionResolveDismissesAlert(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `evolution_alerts` WHERE id = .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "alert_type", "subject_ids"}).
			AddRow(7, 2, "pending", "duplicate", "[10,11]"))
	mock.ExpectExec("UPDATE `evolution_alerts` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	alert, changed, err := NewEvolutionAlertRepository(db).Resolve(context.Background(), 2, 7,
		model.EvolutionActionDismiss, "不是重复", "", nil)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.EvolutionAlertDismissed, alert.Status)
	require.NotNil(t, alert.ResolvedAt, "忽略也是一次裁决")
	require.Equal(t, model.EvolutionActionDismiss, alert.ResolvedAction)
	require.NoError(t, mock.ExpectationsWereMet())
}
