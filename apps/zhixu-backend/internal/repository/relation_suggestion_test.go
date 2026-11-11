package repository

import (
	"context"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

const relationPairCountQuery = "SELECT count.*FROM .*knowledge_relations.* WHERE user_id = .*"

// AI 建议走 CreateSuggestion：同一对知识只要已有任意状态的关系就不再插入，
// 既不覆盖用户已确认的行，也不重复堆积建议。
func TestCreateSuggestionInsertsNewPair(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairCountQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .knowledge_relations.").WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectCommit()

	repo := NewRelationRepository(db)
	created, err := repo.CreateSuggestion(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 12, TargetID: 21, RelationType: model.RelationTypeRelated,
		Status: model.RelationSuggested, Source: model.RelationSourceAI,
		Confidence: 0.78, Evidence: "{\"reason\":\"相关\"}",
	})

	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 对称类型先归一，再按归一后的对查询；这样从任一方向触发都落在同一对。
func TestCreateSuggestionLooksUpNormalizedPair(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairCountQuery).
		WithArgs(uint(2), uint(12), uint(21), "related").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	repo := NewRelationRepository(db)
	created, err := repo.CreateSuggestion(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 21, TargetID: 12, RelationType: model.RelationTypeRelated,
		Status: model.RelationSuggested, Source: model.RelationSourceAI,
	})
	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSuggestionSkipsExistingRejectedPair(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairCountQuery).
		WithArgs(uint(2), uint(12), uint(21), "depends_on").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	repo := NewRelationRepository(db)
	created, err := repo.CreateSuggestion(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 12, TargetID: 21, RelationType: model.RelationTypeDependsOn,
		Status: model.RelationSuggested, Source: model.RelationSourceAI,
	})

	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 去重按唯一键 (user, source, target, relation_type)：同一对的不同类型可以各自插入。
func TestCreateSuggestionDedupesByRelationType(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairCountQuery).
		WithArgs(uint(2), uint(12), uint(21), "depends_on").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .knowledge_relations.").WillReturnResult(sqlmock.NewResult(52, 1))
	mock.ExpectCommit()

	created, err := NewRelationRepository(db).CreateSuggestion(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 12, TargetID: 21, RelationType: model.RelationTypeDependsOn,
		Status: model.RelationSuggested, Source: model.RelationSourceAI,
	})

	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 并发任务同时写同一对：唯一键冲突视为“已有关系”，不向任务抛错。
func TestCreateSuggestionToleratesDuplicateKey(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery(relationPairCountQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .knowledge_relations.").
		WillReturnError(&mysqlDriver.MySQLError{Number: 1062})
	mock.ExpectRollback()

	repo := NewRelationRepository(db)
	created, err := repo.CreateSuggestion(context.Background(), &model.KnowledgeRelation{
		UserID: 2, SourceID: 12, TargetID: 21, RelationType: model.RelationTypeRelated,
		Status: model.RelationSuggested, Source: model.RelationSourceAI,
	})

	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}
