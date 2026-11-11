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

func graphNodeRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "title", "source_type", "summary", "updated_at"})
}

func TestGraphGetNodeFiltersOwnerAndArchived(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	updated := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. WHERE id = \\? AND user_id = \\? AND archived_at IS NULL LIMIT \\?").
		WithArgs(uint(7), uint(2), 1).
		WillReturnRows(graphNodeRows().AddRow(7, "中心知识", "markdown", "摘要", updated))

	row, err := NewGraphRepository(db).GetNode(context.Background(), 2, 7)
	require.NoError(t, err)
	require.Equal(t, uint(7), row.ID)
	require.Equal(t, "中心知识", row.Title)
	require.Equal(t, "markdown", row.SourceType)
	require.Equal(t, "摘要", row.Summary)
	require.Equal(t, updated, row.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 已归档、属于他人或不存在的知识都必须表现为「找不到」，由上层映射为 404。
func TestGraphGetNodeMissingOrArchived(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. WHERE id = \\? AND user_id = \\? AND archived_at IS NULL LIMIT \\?").
		WithArgs(uint(9), uint(2), 1).
		WillReturnRows(graphNodeRows())

	_, err := NewGraphRepository(db).GetNode(context.Background(), 2, 9)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphListRecentNodesExcludesArchived(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. WHERE knowledge_items.user_id = \\? AND knowledge_items.archived_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT \\?").
		WithArgs(uint(2), 50).
		WillReturnRows(graphNodeRows().
			AddRow(3, "较新的知识", "web", "", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)).
			AddRow(1, "较旧的知识", "manual", "", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)))

	rows, err := NewGraphRepository(db).ListRecentNodes(context.Background(), 2, GraphNodeFilter{}, 50)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, uint(3), rows[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphListNodesByIDsFiltersOwnerAndArchived(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. WHERE knowledge_items.user_id = \\? AND knowledge_items.archived_at IS NULL AND knowledge_items.id IN \\(\\?,\\?\\) ORDER BY id ASC").
		WithArgs(uint(2), uint(7), uint(8)).
		WillReturnRows(graphNodeRows().AddRow(7, "中心知识", "manual", "", time.Now()))

	rows, err := NewGraphRepository(db).ListNodesByIDs(context.Background(), 2, GraphNodeFilter{}, []uint{7, 8})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 默认只要已确认关系；打开建议边后追加 suggested；rejected 永不进入图谱。
func TestGraphListRelationsStatusScope(t *testing.T) {
	t.Run("confirmed only by default", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery("SELECT \\* FROM .knowledge_relations. WHERE \\(user_id = \\? AND status IN \\(\\?\\)\\) AND \\(source_id IN \\(\\?\\) OR target_id IN \\(\\?\\)\\) ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_id", "target_id", "relation_type", "status", "source", "confidence"}).
				AddRow(11, 2, 7, 8, "depends_on", "confirmed", "manual", 0))

		rows, err := NewGraphRepository(db).ListRelations(context.Background(), 2, []uint{7}, false)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, model.RelationConfirmed, rows[0].Status)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("suggested appended on request", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery("SELECT \\* FROM .knowledge_relations. WHERE \\(user_id = \\? AND status IN \\(\\?,\\?\\)\\) AND \\(source_id IN \\(\\?\\) OR target_id IN \\(\\?\\)\\) ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_id", "target_id", "relation_type", "status", "source", "confidence"}).
				AddRow(12, 2, 7, 9, "related", "suggested", "ai", 0.82))

		rows, err := NewGraphRepository(db).ListRelations(context.Background(), 2, []uint{7}, true)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, model.RelationSuggested, rows[0].Status)
		require.Equal(t, model.RelationSourceAI, rows[0].Source)
		require.Equal(t, 0.82, rows[0].Confidence)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// 标签筛选通过 knowledge_tags 连接命中，来源类型精确匹配；两者同时给出时取交集。
func TestGraphListRecentNodesAppliesFilters(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. JOIN knowledge_tags ON knowledge_tags.knowledge_id = knowledge_items.id AND knowledge_tags.user_id = knowledge_items.user_id WHERE \\(knowledge_items.user_id = \\? AND knowledge_items.archived_at IS NULL\\) AND knowledge_tags.tag_id = \\? AND source_type = \\? ORDER BY updated_at DESC, id DESC LIMIT \\?").
		WithArgs(uint(2), uint(5), "web", 50).
		WillReturnRows(graphNodeRows().AddRow(9, "网页知识", "web", "", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)))

	rows, err := NewGraphRepository(db).ListRecentNodes(context.Background(), 2, GraphNodeFilter{TagID: 5, SourceType: "web"}, 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, uint(9), rows[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphListNodesByIDsAppliesFilters(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, title, source_type, summary, updated_at FROM .knowledge_items. JOIN knowledge_tags ON knowledge_tags.knowledge_id = knowledge_items.id AND knowledge_tags.user_id = knowledge_items.user_id WHERE \\(knowledge_items.user_id = \\? AND knowledge_items.archived_at IS NULL AND knowledge_items.id IN \\(\\?,\\?\\)\\) AND knowledge_tags.tag_id = \\? ORDER BY id ASC").
		WithArgs(uint(2), uint(7), uint(8), uint(5)).
		WillReturnRows(graphNodeRows().AddRow(7, "中心知识", "manual", "", time.Now()))

	rows, err := NewGraphRepository(db).ListNodesByIDs(context.Background(), 2, GraphNodeFilter{TagID: 5}, []uint{7, 8})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 最终取边时两端都必须在节点集合内；只有一个端点的关系由 SQL 排除，避免越界边进入子图。
func TestGraphListRelationsAmongRequiresBothEnds(t *testing.T) {
	t.Run("confirmed only by default", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery("SELECT \\* FROM .knowledge_relations. WHERE \\(user_id = \\? AND status IN \\(\\?\\)\\) AND \\(source_id IN \\(\\?,\\?\\) AND target_id IN \\(\\?,\\?\\)\\) ORDER BY id ASC").
			WithArgs(uint(2), model.RelationConfirmed, uint(7), uint(8), uint(7), uint(8)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_id", "target_id", "relation_type", "status", "source", "confidence"}).
				AddRow(11, 2, 7, 8, "depends_on", "confirmed", "manual", 0))

		rows, err := NewGraphRepository(db).ListRelationsAmong(context.Background(), 2, []uint{7, 8}, false)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, uint(11), rows[0].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("suggested appended on request", func(t *testing.T) {
		db, mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery("SELECT \\* FROM .knowledge_relations. WHERE \\(user_id = \\? AND status IN \\(\\?,\\?\\)\\) AND \\(source_id IN \\(\\?,\\?\\) AND target_id IN \\(\\?,\\?\\)\\) ORDER BY id ASC").
			WithArgs(uint(2), model.RelationConfirmed, model.RelationSuggested, uint(7), uint(9), uint(7), uint(9)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "source_id", "target_id", "relation_type", "status", "source", "confidence"}).
				AddRow(12, 2, 7, 9, "related", "suggested", "ai", 0.82))

		rows, err := NewGraphRepository(db).ListRelationsAmong(context.Background(), 2, []uint{7, 9}, true)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, model.RelationSuggested, rows[0].Status)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
