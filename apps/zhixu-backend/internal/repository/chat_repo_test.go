package repository

import (
	"context"
	"strings"
	"sync"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func expectConversationLock(mock sqlmock.Sqlmock, user uint, id string, deleted, pending bool) {
	mock.ExpectExec("INSERT INTO `chat_conversation_states`.*ON DUPLICATE KEY UPDATE").
		WithArgs(user, id, false, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `chat_conversation_states` WHERE .*FOR UPDATE").
		WithArgs(user, id, 1).WillReturnRows(sqlmock.NewRows([]string{"user_id", "conversation_id", "tombstoned", "cleanup_pending"}).AddRow(user, id, deleted, pending))
}

func expectSourceDelete(mock sqlmock.Sqlmock, user uint, id string) {
	mock.ExpectExec("DELETE FROM `chat_turn_sources` WHERE conversation_id = .* AND user_id =").
		WithArgs(id, user).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestChatSourceSaveUsesPersistentConversationLock(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	expectConversationLock(mock, 7, "c1", false, false)
	query := strings.Repeat("知", 512)
	mock.ExpectExec("INSERT INTO `chat_turn_sources`").WithArgs("c1", uint(7), "m1", query, "[]", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err := NewChatSourceRepository(db).SaveTurnSource(context.Background(), &model.ChatTurnSource{UserID: 7, ConversationID: "c1", MessageID: "m1", Query: query, Sources: "[]"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatSourceSaveAfterDeletedOrCleanupNeverInserts(t *testing.T) {
	for _, pending := range []bool{true, false} {
		t.Run(map[bool]string{true: "pending", false: "completed"}[pending], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			expectConversationLock(mock, 7, "c1", true, pending)
			mock.ExpectRollback()
			err := NewChatSourceRepository(db).SaveTurnSource(context.Background(), &model.ChatTurnSource{UserID: 7, ConversationID: "c1", MessageID: "late", Sources: "[]"})
			require.ErrorIs(t, err, model.ErrConversationDeleted)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestChatSourceMissingMessageIDDoesNotTouchDatabase(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	repo := NewChatSourceRepository(db)
	for _, id := range []string{"", "  "} {
		require.NoError(t, repo.SaveTurnSource(context.Background(), &model.ChatTurnSource{ConversationID: "c1", UserID: 7, MessageID: id}))
	}
	require.NoError(t, repo.SaveTurnSource(context.Background(), nil))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatDeletionMarksAndClearsSourcesInSameTransaction(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	expectConversationLock(mock, 7, "c1", false, false)
	mock.ExpectExec("UPDATE `chat_conversation_states` SET .* WHERE `user_id` = .* AND `conversation_id` =").
		WithArgs(true, true, sqlmock.AnyArg(), uint(7), "c1").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSourceDelete(mock, 7, "c1")
	mock.ExpectCommit()
	require.NoError(t, NewChatSourceRepository(db).MarkDeleted(context.Background(), "c1", 7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatDeletionRollbackIfSourceRemovalFails(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	expectConversationLock(mock, 7, "c1", false, false)
	mock.ExpectExec("UPDATE `chat_conversation_states`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `chat_turn_sources`").WillReturnError(assertionError{})
	mock.ExpectRollback()
	require.Error(t, NewChatSourceRepository(db).MarkDeleted(context.Background(), "c1", 7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatCleanupPreservesTombstoneAndClearsSources(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	expectConversationLock(mock, 7, "c1", true, true)
	expectSourceDelete(mock, 7, "c1")
	mock.ExpectExec("UPDATE `chat_conversation_states` SET `cleanup_pending`=.* WHERE `user_id` = .* AND `conversation_id` =").
		WithArgs(false, sqlmock.AnyArg(), uint(7), "c1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, NewChatSourceRepository(db).CompleteCleanup(context.Background(), "c1", 7))
	// Repeated remote deletion failures must also leave recoverable pending work.
	mock.ExpectBegin()
	expectConversationLock(mock, 7, "c1", true, false)
	mock.ExpectExec("UPDATE `chat_conversation_states` SET .* WHERE `user_id` = .* AND `conversation_id` =").
		WithArgs(true, true, sqlmock.AnyArg(), uint(7), "c1").WillReturnResult(sqlmock.NewResult(0, 1))
	expectSourceDelete(mock, 7, "c1")
	mock.ExpectCommit()
	require.NoError(t, NewChatSourceRepository(db).MarkDeleted(context.Background(), "c1", 7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatPendingCleanupAndDeletedLookupsArePersistedAndScoped(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	repo := NewChatSourceRepository(db)
	mock.ExpectQuery("SELECT .* FROM `chat_conversation_states` WHERE tombstoned = .* AND cleanup_pending = .*ORDER BY updated_at ASC").
		WithArgs(true, true).WillReturnRows(sqlmock.NewRows([]string{"user_id", "conversation_id", "tombstoned", "cleanup_pending"}).AddRow(7, "c1", true, true))
	rows, err := repo.PendingCleanups(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	mock.ExpectQuery("SELECT .* FROM `chat_conversation_states` WHERE user_id = .* AND conversation_id = .*LIMIT").
		WithArgs(uint(7), "c1", 1).WillReturnRows(sqlmock.NewRows([]string{"tombstoned"}).AddRow(true))
	deleted, err := repo.IsDeleted(context.Background(), "c1", 7)
	require.NoError(t, err)
	require.True(t, deleted)
	mock.ExpectQuery("SELECT .* FROM `chat_conversation_states` WHERE user_id = .* AND conversation_id = .*LIMIT").
		WithArgs(uint(8), "c1", 1).WillReturnRows(sqlmock.NewRows([]string{"tombstoned"}))
	deleted, err = repo.IsDeleted(context.Background(), "c1", 8)
	require.NoError(t, err)
	require.False(t, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatConversationStateHasCompositePrimaryKeyAndQueryCapacity(t *testing.T) {
	state, err := schema.Parse(&model.ChatConversationState{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	require.Equal(t, []string{"user_id", "conversation_id"}, state.PrimaryFieldDBNames)
	require.False(t, state.LookUpField("UserID").AutoIncrement)
	source, err := schema.Parse(&model.ChatTurnSource{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	require.Equal(t, 512, source.LookUpField("Query").Size)
	require.Equal(t, 191, source.LookUpField("MessageID").Size)
}
