package repository

import (
	"context"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func expectAIConfigRead(mock sqlmock.Sqlmock, user uint) {
	mock.ExpectQuery("SELECT .* FROM `model_providers` WHERE user_id = .*ORDER BY id ASC").
		WithArgs(user).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}))
	mock.ExpectQuery("SELECT .* FROM `provider_models` WHERE user_id = .*ORDER BY id ASC").
		WithArgs(user).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}))
	mock.ExpectQuery("SELECT .* FROM `user_ai_settings` WHERE user_id = .*LIMIT").
		WithArgs(user, user, 1).WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
}

func TestAIConfigRepositoryReadScoped(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	expectAIConfigRead(mock, 7)
	mock.ExpectCommit()
	state, err := (&AIConfigRepository{DB: db}).Read(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, state.Providers)
	require.Equal(t, uint(7), state.Settings.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 设置行的更新列清单必须包含演化候选门槛与待校准标记，否则保存成功但读回仍是旧值。
func TestAIConfigRepositoryPersistsEvolveCandidateGate(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `user_models` WHERE id = .*LIMIT .*FOR UPDATE").
		WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	expectAIConfigRead(mock, 7)
	mock.ExpectExec("INSERT INTO `user_ai_settings`.*ON DUPLICATE KEY UPDATE.*evolve_candidate_min_score.*evolve_threshold_stale").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := (&AIConfigRepository{DB: db}).Mutate(context.Background(), 7, func(state *AIConfigState) error {
		state.Settings.EvolveCandidateMinScore = 0.7
		state.Settings.EvolveThresholdStale = true
		return nil
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAIConfigRepositoryAtomicCreation(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[fail], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `user_models` WHERE id = .*LIMIT .*FOR UPDATE").
				WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			expectAIConfigRead(mock, 7)
			mock.ExpectExec("INSERT INTO `model_providers`").WillReturnResult(sqlmock.NewResult(10, 1))
			query := mock.ExpectExec("INSERT INTO `provider_models`")
			if fail {
				query.WillReturnError(assertionError{})
				mock.ExpectRollback()
			} else {
				query.WillReturnResult(sqlmock.NewResult(20, 1))
				mock.ExpectExec("INSERT INTO `user_ai_settings`.*ON DUPLICATE KEY UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			state, err := (&AIConfigRepository{DB: db}).Mutate(context.Background(), 7, func(state *AIConfigState) error {
				state.Providers = append(state.Providers, model.ModelProvider{ProviderID: "gateway"})
				state.Models = append(state.Models, model.ProviderModel{ModelID: "embed", Kind: "embedding", Dimension: 1024})
				return nil
			})
			if fail {
				require.Error(t, err)
				require.Nil(t, state)
			} else {
				require.NoError(t, err)
				require.Equal(t, uint(10), state.Models[0].ProviderID)
				require.Equal(t, uint(7), state.Models[0].UserID)
				require.Equal(t, uint64(1), state.Settings.ConfigVersion)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAIConfigRepositoryRejectedMutationDoesNotWrite(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `user_models` WHERE id = .*FOR UPDATE").
		WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	expectAIConfigRead(mock, 7)
	mock.ExpectRollback()
	_, err := (&AIConfigRepository{DB: db}).Mutate(context.Background(), 7, func(*AIConfigState) error { return assertionError{} })
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAIConfigRepositoryDeletionScopedAndAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[fail], func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `user_models` WHERE id = .*FOR UPDATE").
				WithArgs(uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectQuery("SELECT .* FROM `model_providers` WHERE user_id = .*ORDER BY").
				WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(10, 7))
			mock.ExpectQuery("SELECT .* FROM `provider_models` WHERE user_id = .*ORDER BY").
				WithArgs(uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "provider_id"}).AddRow(20, 7, 10))
			mock.ExpectQuery("SELECT .* FROM `user_ai_settings` WHERE user_id = .*LIMIT").
				WithArgs(uint(7), uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
			mock.ExpectExec("DELETE FROM `provider_models` WHERE id = .*user_id =").
				WithArgs(uint(20), uint(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			deletion := mock.ExpectExec("DELETE FROM `model_providers` WHERE id = .*user_id =").WithArgs(uint(10), uint(7))
			if fail {
				deletion.WillReturnError(assertionError{})
				mock.ExpectRollback()
			} else {
				deletion.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("INSERT INTO `user_ai_settings`.*ON DUPLICATE KEY UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			_, err := (&AIConfigRepository{DB: db}).Mutate(context.Background(), 7, func(state *AIConfigState) error {
				state.Providers = nil
				state.Models = nil
				return nil
			})
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
