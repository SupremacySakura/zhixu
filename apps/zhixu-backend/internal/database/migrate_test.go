package database

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestBuildUploadIdentityBackfills_PreservesOneCanonicalTaskAndSeparatesDuplicates(t *testing.T) {
	tasks := []model.UploadTask{
		{
			ID: 1, UserID: 7, FolderID: 9, FileName: " report.pdf ", FileHash: "ABCDEF",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		},
		{
			ID: 2, UserID: 7, FolderID: 9, FileName: " report.pdf ", FileHash: "ABCDEF",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		},
	}

	backfills, err := buildUploadIdentityBackfills(tasks)

	require.NoError(t, err)
	require.Len(t, backfills, 2)
	assert.NotEmpty(t, backfills[0].IdempotencyKey)
	assert.NotEmpty(t, backfills[0].RequestHash)
	assert.NotEqual(t, backfills[0].IdempotencyKey, backfills[1].IdempotencyKey)
	assert.Equal(t, backfills[0].RequestHash, backfills[1].RequestHash)
}

func TestBuildUploadIdentityBackfills_DoesNotReplaceAnExistingKey(t *testing.T) {
	tasks := []model.UploadTask{
		{
			ID: 1, UserID: 7, FileName: "report.pdf", FileHash: "abcdef",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
			IdempotencyKey: "already-assigned",
		},
	}

	backfills, err := buildUploadIdentityBackfills(tasks)

	require.NoError(t, err)
	require.Len(t, backfills, 1)
	assert.Equal(t, "already-assigned", backfills[0].IdempotencyKey)
	assert.NotEmpty(t, backfills[0].RequestHash)
}

func TestBuildUploadIdentityBackfills_SkipsTasksWhoseIdentityIsAlreadyBackfilled(t *testing.T) {
	task := model.UploadTask{
		ID: 1, UserID: 7, FileName: "report.pdf", FileHash: "abcdef",
		FileSize: 5, ChunkSize: 5, TotalChunks: 1, FileType: "text/plain",
	}
	key, requestHash, _, err := utils.BuildUploadIdentity(&task)
	require.NoError(t, err)
	task.IdempotencyKey = key
	task.RequestHash = requestHash

	backfills, err := buildUploadIdentityBackfills([]model.UploadTask{task})

	require.NoError(t, err)
	assert.Empty(t, backfills)
}

func TestNormalizeUploadIdentityIndexError_IgnoresDuplicateIndex(t *testing.T) {
	err := fmt.Errorf("create index: %w", &mysqlDriver.MySQLError{
		Number:  1061,
		Message: "Duplicate key name 'uniq_upload_tasks_idempotency_key'",
	})

	assert.NoError(t, normalizeUploadIdentityIndexError(err))
}

func TestNormalizeUploadIdentityIndexError_PreservesOtherErrors(t *testing.T) {
	want := errors.New("connection lost")

	assert.ErrorIs(t, normalizeUploadIdentityIndexError(want), want)
}

func TestEnsureQueryIndexes(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		existing int
		ddlError error
		wantErr  bool
	}{
		{"creates missing indexes", 0, nil, false},
		{"skips existing indexes", 1, nil, false},
		{"tolerates concurrent creation", 0, &mysqlDriver.MySQLError{Number: 1061}, false},
		{"propagates creation failures", 0, errors.New("permission denied"), true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{
				Conn: sqlDB, SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			require.NoError(t, err)

			indexes := append(queryIndexDDLs(), fulltextIndexDDLs()...)
			for _, index := range indexes {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?")).
					WithArgs(index.table, index.name).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(scenario.existing))
				if scenario.existing == 0 {
					exec := mock.ExpectExec(regexp.QuoteMeta(index.ddl))
					if scenario.ddlError != nil {
						exec.WillReturnError(scenario.ddlError)
					} else {
						exec.WillReturnResult(sqlmock.NewResult(0, 0))
					}
				}
				if scenario.wantErr {
					break
				}
			}
			err = ensureQueryIndexes(db)
			if err == nil {
				err = ensureFulltextIndexes(db)
			}
			if scenario.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
