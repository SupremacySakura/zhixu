package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
	"zhixu-backend/internal/model"
)

func TestRagCollectionsUseEmbeddingVersionNotChatConfig(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT DISTINCT `collection` FROM `vectorize_tasks` WHERE user_id = .*embedding_version = .*collection <>").WithArgs(uint(7), uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"collection"}).AddRow("zhixu_u7_v3_c1").AddRow("zhixu_u7_v3_c5"))
	names, err := (&RagRepository{DB: db}).Collections(context.Background(), 7, 3)
	require.NoError(t, err)
	require.Len(t, names, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestRagChunkHydrationRequiresLiveOwnedKnowledge(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM `knowledge_chunks` WHERE user_id = .*state = .*EXISTS .*knowledge_items.user_id = knowledge_chunks.user_id.*knowledge_items.status").WithArgs(uint(7), uint(11), "ready", model.KnowledgeReady).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	chunks, err := (&RagRepository{DB: db}).ChunksByIDs(context.Background(), 7, []uint{11})
	require.NoError(t, err)
	require.Empty(t, chunks)
	require.NoError(t, mock.ExpectationsWereMet())
}
