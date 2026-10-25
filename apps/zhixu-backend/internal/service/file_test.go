package service

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"zhixu-backend/internal/dto"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
)

func TestSanitizeFileName_ValidName(t *testing.T) {
	name := "正常的文件.txt"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "正常的文件.txt", result)
}

func TestSanitizeFileName_EmptyName(t *testing.T) {
	result, err := sanitizeFileName("")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFileName, err)
	assert.Empty(t, result)
}

func TestSanitizeFileName_PathTraversal(t *testing.T) {
	name := "../../../etc/passwd"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "passwd", result)
}

func TestSanitizeFileName_DangerousChars(t *testing.T) {
	dangerous := []string{
		"file|name",
		"file*name",
		"file?name",
		"file<name",
		"file>name",
		"file|name",
	}
	for _, name := range dangerous {
		result, err := sanitizeFileName(name)
		assert.Error(t, err, "Expected error for: %s", name)
		assert.Equal(t, ErrInvalidFileName, err)
		assert.Empty(t, result)
	}
}

func TestSanitizeFileName_ReservedName(t *testing.T) {
	reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "LPT1"}
	for _, name := range reserved {
		result, err := sanitizeFileName(name)
		assert.Error(t, err, "Expected error for reserved name: %s", name)
		assert.Equal(t, ErrInvalidFileName, err)
		assert.Empty(t, result)
	}
}

func TestSanitizeFileName_DotNames(t *testing.T) {
	dotNames := []string{".", ".."}
	for _, name := range dotNames {
		result, err := sanitizeFileName(name)
		assert.Error(t, err, "Expected error for dot name: %s", name)
		assert.Equal(t, ErrInvalidFileName, err)
		assert.Empty(t, result)
	}
}

func TestSanitizeFileName_WithExtension(t *testing.T) {
	name := "document.pdf"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "document.pdf", result)
}

func TestSanitizeFileName_WithDangerousExt(t *testing.T) {
	name := "file.PRN"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "file.PRN", result)
}

// Test IsAllowedMIMEType
func TestIsAllowedMIMEType_ValidTypes(t *testing.T) {
	svc := &fileService{}

	validTypes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"video/mp4",
		"video/avi",
		"text/plain",
		"text/html",
		"application/pdf",
		"application/zip",
	}

	for _, mimeType := range validTypes {
		result := svc.IsAllowedMIMEType(mimeType)
		assert.True(t, result, "Expected %s to be allowed", mimeType)
	}
}

func TestIsAllowedMIMEType_InvalidTypes(t *testing.T) {
	svc := &fileService{}

	invalidTypes := []string{
		"application/x-executable",
		"application/x-msdownload",
		"text/javascript",
		"application/javascript",
		"application/x-shockwave-flash",
		"",
	}

	for _, mimeType := range invalidTypes {
		result := svc.IsAllowedMIMEType(mimeType)
		assert.False(t, result, "Expected %s to be blocked", mimeType)
	}
}

// Test sanitizeStorageFileExt
func TestSanitizeStorageFileExt_ValidExt(t *testing.T) {
	ext := sanitizeStorageFileExt("document.pdf")
	assert.Equal(t, ".pdf", ext)
}

func TestSanitizeStorageFileExt_NoExt(t *testing.T) {
	ext := sanitizeStorageFileExt("document")
	assert.Equal(t, "", ext)
}

func TestSanitizeStorageFileExt_EmptyName(t *testing.T) {
	ext := sanitizeStorageFileExt("")
	assert.Equal(t, "", ext)
}

func TestSanitizeStorageFileExt_DangerousChars(t *testing.T) {
	ext := sanitizeStorageFileExt("file<script>.txt")
	assert.NotContains(t, ext, "<")
	assert.NotContains(t, ext, ">")
}

// Test validateZipEntryPath
func TestValidateZipEntryPath_ValidPath(t *testing.T) {
	paths := []string{
		"folder/file.txt",
		"file.txt",
		"folder/subfolder/file.txt",
	}

	for _, path := range paths {
		err := validateZipEntryPath(path)
		assert.NoError(t, err, "Path %s should be valid", path)
	}
}

func TestValidateZipEntryPath_PathTraversal(t *testing.T) {
	paths := []string{
		"../file.txt",
		"folder/../../file.txt",
		"folder/../file.txt",
	}

	for _, path := range paths {
		err := validateZipEntryPath(path)
		assert.Error(t, err, "Path %s should be invalid", path)
	}
}

func TestValidateZipEntryPath_AbsolutePath(t *testing.T) {
	err := validateZipEntryPath("/absolute/path/file.txt")
	assert.Error(t, err)
}

func TestValidateZipEntryPath_EmptyPath(t *testing.T) {
	err := validateZipEntryPath("")
	assert.Error(t, err)
}

// Test fileService errors
func TestFileService_Errors(t *testing.T) {
	// Verify error variables exist and have correct messages
	assert.NotNil(t, ErrPickupCodeExpired)
	assert.NotNil(t, ErrPickupTargetNotFound)
	assert.NotNil(t, ErrPickupEmptyFolder)
	assert.NotNil(t, ErrPublicShareNotFound)
	assert.NotNil(t, ErrInvalidFileName)
	assert.NotNil(t, ErrStorageQuotaExceeded)
	assert.NotNil(t, ErrChunkSizeMismatch)
	assert.NotNil(t, ErrInvalidMIMEType)
}

func TestBuildUploadIdentity_IsStableForTheSameLogicalUpload(t *testing.T) {
	task := &model.UploadTask{
		UserID:      7,
		FolderID:    9,
		FileName:    " report.pdf ",
		FileHash:    "abcdef0123456789",
		FileSize:    1024,
		ChunkSize:   512,
		TotalChunks: 2,
		FileType:    "application/pdf",
	}

	firstKey, firstRequestHash, firstName, err := buildUploadIdentity(task)
	assert.NoError(t, err)
	secondKey, secondRequestHash, secondName, err := buildUploadIdentity(task)

	assert.NoError(t, err)
	assert.Equal(t, firstKey, secondKey)
	assert.Equal(t, firstRequestHash, secondRequestHash)
	assert.Equal(t, "report.pdf", firstName)
	assert.Equal(t, firstName, secondName)
}

func TestBuildUploadIdentity_SeparatesResourceIdentityFromRequestParameters(t *testing.T) {
	base := &model.UploadTask{
		UserID:      7,
		FolderID:    9,
		FileName:    "report.pdf",
		FileHash:    "abcdef0123456789",
		FileSize:    1024,
		ChunkSize:   512,
		TotalChunks: 2,
		FileType:    "application/pdf",
	}
	changedChunking := *base
	changedChunking.ChunkSize = 1024
	changedChunking.TotalChunks = 1

	baseKey, baseRequestHash, _, err := buildUploadIdentity(base)
	assert.NoError(t, err)
	changedKey, changedRequestHash, _, err := buildUploadIdentity(&changedChunking)

	assert.NoError(t, err)
	assert.Equal(t, baseKey, changedKey)
	assert.NotEqual(t, baseRequestHash, changedRequestHash)
}

func TestInitUploadFile_ReusesTheTaskForTheSameRequest(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	req := &model.UploadTask{
		UserID:      7,
		FolderID:    0,
		FileName:    "report.pdf",
		FileHash:    "abcdef0123456789",
		FileSize:    1024,
		ChunkSize:   512,
		TotalChunks: 2,
		FileType:    "application/pdf",
	}
	idempotencyKey, requestHash, _, err := buildUploadIdentity(req)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(idempotencyKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_name", "file_hash", "file_size", "chunk_size", "total_chunks",
			"uploaded_chunks", "file_type", "folder_id", "user_id", "status",
			"idempotency_key", "request_hash",
		}).AddRow(
			41, "report.pdf", req.FileHash, req.FileSize, req.ChunkSize, req.TotalChunks,
			"[0]", req.FileType, req.FolderID, req.UserID, model.UploadStatusUploading,
			idempotencyKey, requestHash,
		))

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	task, err := svc.InitUploadFile(req)

	require.NoError(t, err)
	assert.Equal(t, uint(41), task.ID)
	assert.Equal(t, model.IntSlice{0}, task.UploadedChunks)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteFileWithKnowledgeIsAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "knowledge failure rolls back file deletion"}[fail], func(t *testing.T) {
			db, mock, cleanup := setupServiceMockDB(t)
			defer cleanup()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
				WithArgs(uint(5), uint(7), 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name"}).AddRow(5, 7, "note.md"))
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE `file_models`").WithArgs(sqlmock.AnyArg(), uint(5), uint(7)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			query := mock.ExpectExec("DELETE FROM `knowledge_import_tasks` WHERE user_id = .*knowledge_id IN").
				WithArgs(uint(7), uint(7), uint(5))
			if fail {
				query.WillReturnError(errors.New("knowledge delete failed"))
				mock.ExpectRollback()
			} else {
				query.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
					WithArgs(uint(7), uint(7), uint(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
					WithArgs(uint(7), uint(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(9, 7))
				mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
				for _, table := range []string{"vectorize_tasks", "knowledge_chunks"} {
					mock.ExpectExec("DELETE FROM `"+table+"` WHERE user_id = .*knowledge_id IN").
						WithArgs(uint(7), uint(7), uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectExec("DELETE FROM `knowledge_items` WHERE user_id = .*source_file_id").
					WithArgs(uint(7), uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			svc := &fileService{FileRepository: repository.NewFileRepository(db)}
			err := svc.DeleteByIDs(7, 5, 0, true)
			if fail {
				require.ErrorContains(t, err, "knowledge delete failed")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeleteFolderWithKnowledgeIncludesDescendants(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `folder_models`").
		WithArgs(uint(3), uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(3, 7))
	mock.ExpectQuery("SELECT .* FROM `folder_models`").
		WithArgs(uint(3), uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	mock.ExpectQuery("SELECT .* FROM `folder_models`").
		WithArgs(uint(4), uint(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("UPDATE `file_models`").WithArgs(sqlmock.AnyArg(), uint(3), uint(4), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM `knowledge_import_tasks` .*source_file_id IN .*folder_id IN").
		WithArgs(uint(7), uint(7), uint(7), uint(3), uint(4)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
		WithArgs(uint(7), uint(7), uint(7), uint(3), uint(4)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
		WithArgs(uint(7), uint(7), uint(3), uint(4)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(9, 7).AddRow(10, 7))
	mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(2, 1))
	for _, table := range []string{"vectorize_tasks", "knowledge_chunks"} {
		mock.ExpectExec("DELETE FROM `"+table+"` .*source_file_id IN .*folder_id IN").
			WithArgs(uint(7), uint(7), uint(7), uint(3), uint(4)).WillReturnResult(sqlmock.NewResult(0, 2))
	}
	mock.ExpectExec("DELETE FROM `knowledge_items` .*source_file_id IN .*folder_id IN").
		WithArgs(uint(7), uint(7), uint(3), uint(4)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `folder_models`").WithArgs(sqlmock.AnyArg(), uint(3), uint(4), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	require.NoError(t, svc.DeleteByIDs(7, 0, 3, true))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInitUploadFile_ReuploadsDeletedCompletedFiles(t *testing.T) {
	// Each file retains a completed task even after its logical file is soft-deleted.
	for _, name := range []string{"report.md", "report-copy.md", "report-copy-2.md"} {
		t.Run(name, func(t *testing.T) {
			db, mock, cleanup := setupServiceMockDB(t)
			defer cleanup()
			req := &model.UploadTask{
				UserID: 7, FileName: name, FileHash: "abcdef0123456789",
				FileSize: 1024, ChunkSize: 1024, TotalChunks: 1, FileType: "text/markdown",
				UploadedChunks: model.IntSlice{},
			}
			key, _, _, err := buildUploadIdentity(req)
			require.NoError(t, err)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
				WithArgs(key, 1).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "user_id", "status", "file_id", "idempotency_key", "request_hash",
				}).AddRow(41, 7, model.UploadStatusCompleted, 88, key, "old-chunk-parameters"))
			mock.ExpectQuery("SELECT .* FROM `file_models` WHERE .*deleted_at.*").
				WithArgs(uint(88), uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE `upload_tasks` SET .*idempotency_key.*").
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), uint(41), uint(7), key, model.UploadStatusCompleted).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
				WithArgs(req.FileHash, req.UserID, req.FolderID, 1).WillReturnError(gorm.ErrRecordNotFound)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
				WithArgs(req.FileHash, 1).WillReturnError(gorm.ErrRecordNotFound)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `upload_tasks`")).
				WillReturnResult(sqlmock.NewResult(55, 1))
			mock.ExpectCommit()
			chunkRoot := t.TempDir()
			svc := &fileService{
				FileRepository:     repository.NewFileRepository(db),
				FileServiceOptions: FileServiceOptions{ChunkStoragePath: chunkRoot},
			}
			task, err := svc.InitUploadFile(req)
			require.NoError(t, err)
			require.Equal(t, uint(55), task.ID)
			require.Equal(t, model.UploadStatusUploading, task.Status)
			require.Nil(t, task.FileID)
			require.Empty(t, task.UploadedChunks)
			_, err = os.Stat(filepath.Join(chunkRoot, "55"))
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestReusableUploadTask_CompletedFileValidation(t *testing.T) {
	for _, test := range []struct {
		name          string
		fileExists    bool
		databaseError bool
		conflict      bool
	}{
		{name: "live file remains idempotent", fileExists: true},
		{name: "live file still rejects parameter conflicts", fileExists: true, conflict: true},
		{name: "database failure never retires task", databaseError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, cleanup := setupServiceMockDB(t)
			defer cleanup()
			id := uint(88)
			task := &model.UploadTask{ID: 41, UserID: 7, Status: model.UploadStatusCompleted, FileID: &id, RequestHash: "same"}
			query := mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
				WithArgs(id, uint(7), 1)
			if test.databaseError {
				query.WillReturnError(errors.New("database unavailable"))
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
			}
			svc := &fileService{FileRepository: repository.NewFileRepository(db)}
			hash := "same"
			if test.conflict {
				hash = "different"
			}
			got, err := svc.reusableUploadTask(task, hash)
			switch {
			case test.databaseError:
				require.ErrorContains(t, err, "database unavailable")
			case test.conflict:
				require.ErrorIs(t, err, ErrUploadRequestConflict)
			default:
				require.NoError(t, err)
				require.Same(t, task, got)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestReusableUploadTask_ConcurrentRetirement(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()
	id := uint(88)
	task := &model.UploadTask{ID: 41, UserID: 7, Status: model.UploadStatusCompleted, FileID: &id, IdempotencyKey: "old-key"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(id, uint(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `upload_tasks` SET .*idempotency_key.*").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), uint(41), uint(7), "old-key", model.UploadStatusCompleted).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	got, err := svc.reusableUploadTask(task, "new-parameters")
	require.NoError(t, err)
	require.Nil(t, got, "another request already retired the key; never reuse the deleted file")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInitUploadFile_RejectsDifferentParametersForTheSameIdentity(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	req := &model.UploadTask{
		UserID:      7,
		FolderID:    0,
		FileName:    "report.pdf",
		FileHash:    "abcdef0123456789",
		FileSize:    1024,
		ChunkSize:   1024,
		TotalChunks: 1,
		FileType:    "application/pdf",
	}
	idempotencyKey, _, _, err := buildUploadIdentity(req)
	assert.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(idempotencyKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_name", "file_hash", "file_size", "chunk_size", "total_chunks",
			"uploaded_chunks", "file_type", "folder_id", "user_id", "status",
			"idempotency_key", "request_hash",
		}).AddRow(
			41, "report.pdf", req.FileHash, req.FileSize, 512, 2,
			"[0]", req.FileType, req.FolderID, req.UserID, model.UploadStatusUploading,
			idempotencyKey, "a-different-request-hash",
		))

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	task, err := svc.InitUploadFile(req)

	assert.Nil(t, task)
	assert.ErrorContains(t, err, "upload request conflicts")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitUploadFile_ReturnsTheWinningTaskAfterAConcurrentInsert(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	req := &model.UploadTask{
		UserID: 7, FolderID: 0, FileName: "report.pdf", FileHash: "abcdef0123456789",
		FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		UploadedChunks: model.IntSlice{},
	}
	idempotencyKey, requestHash, _, err := buildUploadIdentity(req)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(idempotencyKey, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(req.FileHash, req.UserID, req.FolderID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(req.FileHash, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(idempotencyKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_name", "file_hash", "file_size", "chunk_size", "total_chunks",
			"uploaded_chunks", "file_type", "folder_id", "user_id", "status",
			"idempotency_key", "request_hash",
		}).AddRow(
			55, "report.pdf", req.FileHash, req.FileSize, req.ChunkSize, req.TotalChunks,
			"[]", req.FileType, req.FolderID, req.UserID, model.UploadStatusUploading,
			idempotencyKey, requestHash,
		))

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	task, err := svc.InitUploadFile(req)

	require.NoError(t, err)
	assert.Equal(t, uint(55), task.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitUploadFile_RetainsTaskWhenChunkDirectoryCreationFails(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	req := &model.UploadTask{
		UserID: 7, FolderID: 0, FileName: "report.pdf", FileHash: "abcdef0123456789",
		FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		UploadedChunks: model.IntSlice{},
	}
	idempotencyKey, _, _, err := buildUploadIdentity(req)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(idempotencyKey, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(req.FileHash, req.UserID, req.FolderID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(req.FileHash, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectCommit()

	deleted := false
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:observe-upload-task-delete", func(*gorm.DB) {
		deleted = true
	}))
	blockedChunkRoot := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blockedChunkRoot, []byte("blocked"), 0600))
	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: blockedChunkRoot,
		},
	}

	task, err := svc.InitUploadFile(req)

	assert.Nil(t, task)
	require.Error(t, err)
	assert.False(t, deleted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadFileChunkStream_ReusesAnIdenticalChunk(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const chunkHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_size", "chunk_size", "total_chunks", "uploaded_chunks", "user_id", "status",
		}).AddRow(41, 5, 5, 1, "[0]", 7, model.UploadStatusUploading))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_chunks`")).
		WithArgs(uint(41), 0, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "chunk_index", "chunk_hash", "size"}).
			AddRow(3, 41, 0, chunkHash, 5))
	mock.ExpectCommit()

	chunkRoot := t.TempDir()
	chunkDir := filepath.Join(chunkRoot, "41")
	require.NoError(t, os.MkdirAll(chunkDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chunkDir, "0"), []byte("hello"), 0600))
	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: chunkRoot,
		},
	}
	err := svc.UploadFileChunkStream(7, &dto.UploadChunkReq{
		TaskID: 41, ChunkIndex: 0, ChunkHash: chunkHash,
	}, bytes.NewBufferString("hello"), 5)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadFileChunkStream_RepairsMissingProgressWhenReusingAnIdenticalChunk(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const chunkHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_size", "chunk_size", "total_chunks", "uploaded_chunks", "user_id", "status",
		}).AddRow(41, 5, 5, 1, "[]", 7, model.UploadStatusUploading))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_chunks`")).
		WithArgs(uint(41), 0, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "chunk_index", "chunk_hash", "size"}).
			AddRow(3, 41, 0, chunkHash, 5))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	chunkRoot := t.TempDir()
	chunkDir := filepath.Join(chunkRoot, "41")
	require.NoError(t, os.MkdirAll(chunkDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chunkDir, "0"), []byte("hello"), 0600))
	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: chunkRoot,
		},
	}
	err := svc.UploadFileChunkStream(7, &dto.UploadChunkReq{
		TaskID: 41, ChunkIndex: 0, ChunkHash: chunkHash,
	}, bytes.NewBufferString("hello"), 5)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadFileChunkStream_RejectsConflictingChunkContent(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const existingHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	const incomingHash = "486ea46224d1bb4fb680f34f7c9ad96a8f24ec88be73ea8e5a6c65260e9cb8a7"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_size", "chunk_size", "total_chunks", "uploaded_chunks", "user_id", "status",
		}).AddRow(41, 5, 5, 1, "[0]", 7, model.UploadStatusUploading))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_chunks`")).
		WithArgs(uint(41), 0, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "chunk_index", "chunk_hash", "size"}).
			AddRow(3, 41, 0, existingHash, 5))
	mock.ExpectRollback()

	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: t.TempDir(),
		},
	}
	err := svc.UploadFileChunkStream(7, &dto.UploadChunkReq{
		TaskID: 41, ChunkIndex: 0, ChunkHash: incomingHash,
	}, bytes.NewBufferString("world"), 5)

	assert.ErrorContains(t, err, "chunk content conflicts")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadFileChunkStream_PersistsTheChunkAndItsUniqueMetadata(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const chunkHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_size", "chunk_size", "total_chunks", "uploaded_chunks", "user_id", "status",
		}).AddRow(41, 5, 5, 1, "[]", 7, model.UploadStatusUploading))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_chunks`")).
		WithArgs(uint(41), 0, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `upload_chunks`")).
		WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	chunkRoot := t.TempDir()
	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: chunkRoot,
		},
	}
	err := svc.UploadFileChunkStream(7, &dto.UploadChunkReq{
		TaskID: 41, ChunkIndex: 0, ChunkHash: chunkHash,
	}, bytes.NewBufferString("hello"), 5)

	require.NoError(t, err)
	stored, err := os.ReadFile(filepath.Join(chunkRoot, "41", "0"))
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), stored)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMergeUploadedChunks_ReturnsMergingWhenAnotherRequestOwnsTheLease(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "file_id"}).
			AddRow(41, 7, model.UploadStatusMerging, nil))

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	result, err := svc.MergeUploadedChunks(7, 41)

	require.NoError(t, err)
	assert.Equal(t, model.UploadStatusMerging, result.Status)
	assert.Nil(t, result.FileID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMergeUploadedChunks_ReturnsTheSameFileForACompletedTask(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "file_id"}).
			AddRow(41, 7, model.UploadStatusCompleted, 88))

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}
	result, err := svc.MergeUploadedChunks(7, 41)

	require.NoError(t, err)
	assert.Equal(t, model.UploadStatusCompleted, result.Status)
	require.NotNil(t, result.FileID)
	assert.Equal(t, uint(88), *result.FileID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMergeUploadedChunks_LeaseOwnerCreatesOneFileAndCompletesTheTask(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const fileHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	fixedNow := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	chunkRoot := t.TempDir()
	fileRoot := t.TempDir()
	chunkDir := filepath.Join(chunkRoot, "41")
	require.NoError(t, os.MkdirAll(chunkDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chunkDir, "0"), []byte("hello"), 0600))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_name", "file_hash", "file_size", "chunk_size", "total_chunks",
			"uploaded_chunks", "file_type", "folder_id", "user_id", "status", "merge_lease_id",
		}).AddRow(
			41, "hello.txt", fileHash, 5, 5, 1, "[0]", "text/plain", 0, 7,
			model.UploadStatusMerging, "fixed-merge-lease",
		))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(size\\), 0\\)").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `upload_tasks`")).
		WithArgs(uint(41), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "file_name", "file_hash", "file_size", "chunk_size", "total_chunks",
			"uploaded_chunks", "file_type", "folder_id", "user_id", "status", "merge_lease_id",
		}).AddRow(
			41, "hello.txt", fileHash, 5, 5, 1, "[0]", "text/plain", 0, 7,
			model.UploadStatusMerging, "fixed-merge-lease",
		))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `file_models`")).
		WillReturnResult(sqlmock.NewResult(88, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: chunkRoot,
			FileStoragePath:  fileRoot,
		},
		newMergeLeaseID: func() (string, error) { return "fixed-merge-lease", nil },
		now:             func() time.Time { return fixedNow },
	}
	result, err := svc.MergeUploadedChunks(7, 41)

	require.NoError(t, err)
	assert.Equal(t, model.UploadStatusCompleted, result.Status)
	require.NotNil(t, result.FileID)
	assert.Equal(t, uint(88), *result.FileID)
	merged, err := os.ReadFile(filepath.Join(fileRoot, fileHash[0:2], fileHash[2:4], fileHash))
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), merged)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test allowedMIMETypes map
func TestAllowedMIMETypes(t *testing.T) {
	// Verify the map is properly initialized
	assert.True(t, allowedMIMETypes["image/"])
	assert.True(t, allowedMIMETypes["video/"])
	assert.True(t, allowedMIMETypes["application/pdf"])
	assert.True(t, allowedMIMETypes["application/zip"])
	assert.True(t, allowedMIMETypes["text/"])
}

// Test sanitizeFileName with null bytes
func TestSanitizeFileName_NullBytes(t *testing.T) {
	name := "file\x00name.txt"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.NotContains(t, result, "\x00")
	assert.Equal(t, "filename.txt", result)
}

// Test sanitizeFileName with control characters
func TestSanitizeFileName_ControlChars(t *testing.T) {
	name := "file\x01\x02\x03name.txt"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "filename.txt", result)
}

// Test sanitizeFileName with spaces
func TestSanitizeFileName_WithSpaces(t *testing.T) {
	name := "file name with spaces.txt"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "file name with spaces.txt", result)
}

// Test sanitizeFileName with unicode
func TestSanitizeFileName_Unicode(t *testing.T) {
	names := []string{
		"文件.txt",
		"文件_日本語.doc",
		"файл.pdf",
		"😀emoji.txt",
	}

	for _, name := range names {
		result, err := sanitizeFileName(name)
		assert.NoError(t, err, "Name %s should be valid", name)
		assert.NotEmpty(t, result)
	}
}

// Test sanitizeFileName very long name
func TestSanitizeFileName_VeryLongName(t *testing.T) {
	// Create a very long name
	longName := ""
	for i := 0; i < 100; i++ {
		longName += "a"
	}
	longName += ".txt"

	result, err := sanitizeFileName(longName)
	assert.NoError(t, err)
	assert.Equal(t, longName, result)
}

// Test sanitizeFileName multiple extensions
func TestSanitizeFileName_MultipleExtensions(t *testing.T) {
	name := "file.tar.gz"
	result, err := sanitizeFileName(name)
	assert.NoError(t, err)
	assert.Equal(t, "file.tar.gz", result)
}

// Test sanitizeFileName case sensitivity
func TestSanitizeFileName_CaseSensitivity(t *testing.T) {
	// Reserved names should be case-insensitive
	_, err := sanitizeFileName("con")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFileName, err)

	_, err = sanitizeFileName("CON")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFileName, err)

	_, err = sanitizeFileName("Con")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFileName, err)
}

func TestBuildFileAbsolutePath_UsesCanonicalHashPath(t *testing.T) {
	tempDir := t.TempDir()
	fileHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	storageDir := filepath.Join(tempDir, fileHash[0:2], fileHash[2:4])
	assert.NoError(t, os.MkdirAll(storageDir, 0755))

	canonicalPath := filepath.Join(storageDir, fileHash)
	assert.NoError(t, os.WriteFile(canonicalPath, []byte("content"), 0644))

	svc := &fileService{FileServiceOptions: FileServiceOptions{FileStoragePath: tempDir}}
	path, err := svc.BuildFileAbsolutePath(&model.FileModel{
		FileHash: fileHash,
		Name:     "renamed.pdf",
	})

	assert.NoError(t, err)
	assert.Equal(t, canonicalPath, path)
}

func TestBuildFileAbsolutePath_FallsBackToLegacyExtensionPath(t *testing.T) {
	tempDir := t.TempDir()
	fileHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	storageDir := filepath.Join(tempDir, fileHash[0:2], fileHash[2:4])
	assert.NoError(t, os.MkdirAll(storageDir, 0755))

	legacyPath := filepath.Join(storageDir, fileHash+".txt")
	assert.NoError(t, os.WriteFile(legacyPath, []byte("content"), 0644))

	svc := &fileService{FileServiceOptions: FileServiceOptions{FileStoragePath: tempDir}}
	path, err := svc.BuildFileAbsolutePath(&model.FileModel{
		FileHash: fileHash,
		Name:     "renamed.pdf",
	})

	assert.NoError(t, err)
	assert.Equal(t, legacyPath, path)
}

func setupServiceMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		DriverName:                "mysql",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	return db, mock, func() {
		_ = sqlDB.Close()
	}
}

func TestDownloadByPickUpCode_ReservesDownloadBeforeStreaming(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	tempDir := t.TempDir()
	fileHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	storageDir := filepath.Join(tempDir, fileHash[0:2], fileHash[2:4])
	assert.NoError(t, os.MkdirAll(storageDir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(storageDir, fileHash), []byte("secret"), 0644))

	expireTime := time.Now().Add(time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pick_up_code_models`")).
		WithArgs("ABC123", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "file_id", "folder_id", "download", "max_download", "expire_time", "code", "status", "user_id"}).
			AddRow(9, model.PickUpTargetTypeFile, uint(7), nil, 0, 1, expireTime, "ABC123", model.PickUpCodeStatusActive, 42))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(7), uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "file_hash", "type", "size", "folder_id", "user_id"}).
			AddRow(7, "secret.txt", fileHash, "document", 6, 0, 42))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pick_up_code_models`")).
		WithArgs(uint(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "file_id", "folder_id", "download", "max_download", "expire_time", "code", "status", "user_id"}).
			AddRow(9, model.PickUpTargetTypeFile, uint(7), nil, 1, 1, expireTime, "ABC123", model.PickUpCodeStatusActive, 42))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `pick_up_code_models`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &fileService{
		FileRepository: repository.NewFileRepository(db),
		FileServiceOptions: FileServiceOptions{
			FileStoragePath: tempDir,
		},
	}
	var writer bytes.Buffer
	err := svc.DownloadByPickUpCode("ABC123", &writer, nil)

	assert.ErrorIs(t, err, ErrPickupCodeExpired)
	assert.Empty(t, writer.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPickUpCodeListByUserID_ReportsEffectiveStatus(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	svc := &fileService{FileRepository: repository.NewFileRepository(db)}

	now := time.Now()
	expiredAt := now.Add(-time.Hour)
	activeAt := now.Add(24 * time.Hour)

	// 列表查询前先批量回写失效状态（GORM 写操作默认包裹事务）
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `pick_up_code_models` SET `status`=?")).
		WithArgs(model.PickUpCodeStatusExpire, uint(7), model.PickUpCodeStatusActive, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	// SELECT 模拟回写前的旧数据，验证 VO 组装时仍按有效状态兜底计算
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pick_up_code_models` WHERE user_id = ? LIMIT ? OFFSET ?")).
		WithArgs(uint(7), 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "file_id", "folder_id", "download", "max_download",
			"expire_time", "created_at", "code", "status", "user_id",
		}).
			AddRow(1, model.PickUpTargetTypeFile, nil, nil, 1, 10, expiredAt, now, "AAAAAA", model.PickUpCodeStatusActive, 7).
			AddRow(2, model.PickUpTargetTypeFile, nil, nil, 1, 10, activeAt, now, "BBBBBB", model.PickUpCodeStatusActive, 7).
			AddRow(3, model.PickUpTargetTypeFile, nil, nil, 10, 10, activeAt, now, "CCCCCC", model.PickUpCodeStatusActive, 7))

	list, err := svc.GetPickUpCodeListByUserID(7, 1, 10)

	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, model.PickUpCodeStatusExpire, list[0].Status) // 已过期
	assert.Equal(t, model.PickUpCodeStatusActive, list[1].Status) // 正常
	assert.Equal(t, model.PickUpCodeStatusExpire, list[2].Status) // 下载次数打满
	assert.NoError(t, mock.ExpectationsWereMet())
}
