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

const relationPairQuery = "SELECT \\* FROM `knowledge_relations` WHERE user_id = \\? AND source_id = \\? AND target_id = \\? AND relation_type = \\?.*"

func TestNormalizeRelationPair(t *testing.T) {
	for _, tc := range []struct {
		name       string
		relation   model.RelationType
		source     uint
		target     uint
		wantSource uint
		wantTarget uint
	}{
		{"related 归一为小 ID 在前", model.RelationTypeRelated, 9, 3, 3, 9},
		{"related 已有序保持不变", model.RelationTypeRelated, 3, 9, 3, 9},
		{"contrasts 归一为小 ID 在前", model.RelationTypeContrasts, 8, 2, 2, 8},
		{"depends_on 保持方向", model.RelationTypeDependsOn, 9, 3, 9, 3},
		{"supplements 保持方向", model.RelationTypeSupplements, 9, 3, 9, 3},
		{"updates 保持方向", model.RelationTypeUpdates, 9, 3, 9, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, target := normalizeRelationPair(tc.relation, tc.source, tc.target)
			require.Equal(t, tc.wantSource, source)
			require.Equal(t, tc.wantTarget, target)
		})
	}
}

// 对称关系无论从哪一侧发起，都按 min/max 写同一行。
func TestCreateRelationNormalizesSymmetricPair(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairQuery).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `knowledge_relations`").WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	row, err := repo.Create(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 9, TargetID: 3, RelationType: model.RelationTypeRelated,
		Status: model.RelationConfirmed, Source: model.RelationSourceManual,
	})

	require.NoError(t, err)
	require.Equal(t, uint(3), row.SourceID)
	require.Equal(t, uint(9), row.TargetID)
	require.Equal(t, uint(5), row.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 有向类型保持入参方向。
func TestCreateRelationKeepsDirectedPair(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairQuery).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `knowledge_relations`").WillReturnResult(sqlmock.NewResult(6, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	row, err := repo.Create(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 9, TargetID: 3, RelationType: model.RelationTypeDependsOn,
		Status: model.RelationConfirmed, Source: model.RelationSourceManual,
	})

	require.NoError(t, err)
	require.Equal(t, uint(9), row.SourceID)
	require.Equal(t, uint(3), row.TargetID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 同一对知识同类型已存在时复用旧行：旧的建议/拒绝行被提升为已确认，不插入第二行。
func TestCreateRelationReusesExistingRow(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairQuery).WillReturnRows(sqlmock.NewRows([]string{
		"id", "user_id", "source_id", "target_id", "relation_type", "status", "source",
	}).AddRow(8, 2, 3, 9, "related", "rejected", "ai"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `knowledge_relations` SET .* WHERE id = \\? AND user_id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	row, err := repo.Create(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 3, TargetID: 9, RelationType: model.RelationTypeRelated,
		Status: model.RelationConfirmed, Source: model.RelationSourceManual,
	})

	require.NoError(t, err)
	require.Equal(t, uint(8), row.ID)
	require.Equal(t, model.RelationConfirmed, row.Status)
	require.Equal(t, model.RelationSourceManual, row.Source)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 并发插入命中唯一键时回读已有行，不把 1062 抛给上层。
func TestCreateRelationToleratesDuplicateKey(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairQuery).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `knowledge_relations`").
		WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectRollback()
	mock.ExpectQuery(relationPairQuery).WillReturnRows(sqlmock.NewRows([]string{
		"id", "user_id", "source_id", "target_id", "relation_type", "status", "source",
	}).AddRow(9, 2, 3, 9, "related", "suggested", "ai"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `knowledge_relations` SET .* WHERE id = \\? AND user_id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	row, err := repo.Create(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 3, TargetID: 9, RelationType: model.RelationTypeRelated,
		Status: model.RelationConfirmed, Source: model.RelationSourceManual,
	})

	require.NoError(t, err)
	require.Equal(t, uint(9), row.ID)
	require.Equal(t, model.RelationConfirmed, row.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListRelationsByKnowledge(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	now := time.Now()
	mock.ExpectQuery("SELECT r\\.\\*, k\\.id AS counterpart_id, k\\.title AS counterpart_title FROM knowledge_relations AS r").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "source_id", "target_id", "relation_type", "status", "source",
			"confidence", "evidence", "created_at", "updated_at", "counterpart_id", "counterpart_title",
		}).
			AddRow(1, 2, 3, 9, "depends_on", "confirmed", "manual", 0.0, "", now, now, 9, "目标知识").
			AddRow(2, 2, 7, 3, "related", "confirmed", "manual", 0.0, "", now, now, 7, "对端知识"))

	repo := NewRelationRepository(db)
	rows, err := repo.ListByKnowledge(context.Background(), 2, 3)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "目标知识", rows[0].CounterpartTitle)
	require.Equal(t, uint(9), rows[0].CounterpartID)
	require.Equal(t, model.RelationTypeDependsOn, rows[0].RelationType)
	require.Equal(t, "对端知识", rows[1].CounterpartTitle)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteRelationReportsMissing(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `knowledge_relations` WHERE id = \\? AND user_id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	err := repo.Delete(context.Background(), 2, 7)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteRelationRemovesOwnedRow(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `knowledge_relations` WHERE id = \\? AND user_id = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	require.NoError(t, repo.Delete(context.Background(), 2, 7))
	require.NoError(t, mock.ExpectationsWereMet())
}
