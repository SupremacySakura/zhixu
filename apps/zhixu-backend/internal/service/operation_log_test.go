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
	"gorm.io/gorm"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
)

// MergeUploadedChunks 在 CAS 赢家真正创建文件后写入 upload 日志
func TestMergeUploadedChunks_RecordsUploadLogForCreatedFile(t *testing.T) {
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
	// 文件创建成功后写入 upload 操作日志（FolderID=0 时 folderName 不查库）
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "upload", "file", uint(88), "hello.txt", "", `{"size":5}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
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
	assert.NoError(t, mock.ExpectationsWereMet())
}

// InitUploadFile 命中秒传（blob 已存在）创建新文件记录后写入 upload_instant 日志
func TestInitUploadFile_RecordsInstantUploadLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const fileHash = "abcdef0123456789"
	req := &model.UploadTask{
		UserID: 7, FolderID: 0, FileName: "report.pdf", FileHash: fileHash,
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
	// blob 已存在 → 秒传：为当前用户创建一条新的文件记录
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(req.FileHash, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "folder_id", "name", "size", "type", "file_hash",
		}).AddRow(9, 7, 0, "report.pdf", 1024, "application/pdf", fileHash))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(size\\), 0\\)").
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `file_models`")).
		WillReturnResult(sqlmock.NewResult(88, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `upload_tasks`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 秒传创建文件记录后写入 upload_instant 操作日志
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "upload_instant", "file", uint(88), "report.pdf", "", `{"size":1024}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
		FileServiceOptions: FileServiceOptions{
			ChunkStoragePath: t.TempDir(),
		},
	}
	task, err := svc.InitUploadFile(req)

	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, model.UploadStatusCompleted, task.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// RenameByIDs 重命名文件时记录 old_name，重命名后日志仍可读
func TestRenameByIDs_RecordsOldNameForFile(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "folder_id", "name", "size", "type", "file_hash",
		}).AddRow(5, 7, 0, "old-name.txt", 100, "text/plain", "hash-old"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "rename", "file", uint(5), "new-name.txt", "", `{"old_name":"old-name.txt"}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.RenameByIDs(7, 5, 0, "new-name.txt")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeleteByIDs 软删除文件后，日志仍使用事前捕获的名称/目录快照
func TestDeleteByIDs_RecordsSnapshotAfterSoftDelete(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "folder_id", "name", "size", "type", "file_hash",
		}).AddRow(5, 7, 3, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	// GORM 软删除：DELETE 生成 UPDATE SET deleted_at，且单条 Delete 也有隐式事务
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WithArgs(sqlmock.AnyArg(), uint(5), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// recordOperation 解析 folderName（folder_id=3 → "工作"）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `folder_models`")).
		WithArgs(uint(3), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "parent_id", "name"}).
			AddRow(3, 7, 0, "工作"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "delete", "file", uint(5), "doc.pdf", "工作", `{"size":2048}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeleteByIDs(7, 5, 0)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 日志写入失败不能影响删除主流程
func TestDeleteByIDs_IgnoresLogWriteFailure(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "folder_id", "name", "size", "type", "file_hash",
		}).AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WithArgs(sqlmock.AnyArg(), uint(5), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 日志写入失败：FolderID=0 不查 folder_models，落库失败后 GORM 回滚隐式事务
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WillReturnError(errors.New("log table unavailable"))
	mock.ExpectRollback()

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeleteByIDs(7, 5, 0)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeleteByIDs 级联删除知识时，文件删除日志的 detail 携带 knowledge_removed 计数
func TestDeleteByIDs_RecordsKnowledgeCascadeCount(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "folder_id", "name", "size", "type", "file_hash",
		}).AddRow(5, 7, 0, "doc.md", 2048, "text/markdown", "hash-doc"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WithArgs(sqlmock.AnyArg(), uint(5), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// 知识级联删除链：任务 → 向量任务锁 → 知识行锁（1 条）→ tombstone → 清理
	mock.ExpectExec("DELETE FROM `knowledge_import_tasks` WHERE user_id = .*knowledge_id IN").
		WithArgs(uint(7), uint(7), uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `vectorize_tasks` .*FOR UPDATE").
		WithArgs(uint(7), uint(7), uint(5)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
		WithArgs(uint(7), uint(5)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(9, 7))
	mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
	for _, table := range []string{"vectorize_tasks", "knowledge_chunks", "knowledge_tags"} {
		mock.ExpectExec("DELETE FROM `"+table+"` WHERE user_id = .*knowledge_id IN").
			WithArgs(uint(7), uint(7), uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("DELETE FROM `knowledge_relations` WHERE user_id = .*").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `knowledge_items` WHERE user_id = .*source_file_id").
		WithArgs(uint(7), uint(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// FolderID=0 时 folderName 不查库，直接落日志（detail 含级联计数）
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "delete", "file", uint(5), "doc.md", "", `{"knowledge_removed":1,"size":2048}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeleteByIDs(7, 5, 0, true)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeleteByIDs 文件夹级联删除知识时，文件夹删除日志携带 knowledge_removed 计数
func TestDeleteFolderByIDs_RecordsKnowledgeCascadeCount(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `folder_models`").
		WithArgs(uint(3), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "parent_id", "name"}).AddRow(3, 7, 0, "资料"))
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
	// 两条知识被级联删除
	mock.ExpectQuery("SELECT .* FROM `knowledge_items` .*FOR UPDATE").
		WithArgs(uint(7), uint(7), uint(3), uint(4)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(9, 7).AddRow(10, 7))
	mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `vector_cleanup_tasks`").WillReturnResult(sqlmock.NewResult(2, 1))
	for _, table := range []string{"vectorize_tasks", "knowledge_chunks", "knowledge_tags"} {
		mock.ExpectExec("DELETE FROM `"+table+"` .*source_file_id IN .*folder_id IN").
			WithArgs(uint(7), uint(7), uint(7), uint(3), uint(4)).WillReturnResult(sqlmock.NewResult(0, 2))
	}
	mock.ExpectExec("DELETE FROM `knowledge_relations` WHERE user_id = .*").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM `knowledge_items` .*source_file_id IN .*folder_id IN").
		WithArgs(uint(7), uint(7), uint(3), uint(4)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `folder_models`").WithArgs(sqlmock.AnyArg(), uint(3), uint(4), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	// 根目录文件夹（parent_id=0）不查 folderName；detail 只含级联计数
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "delete", "folder", uint(3), "资料", "", `{"knowledge_removed":2}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeleteByIDs(7, 0, 3, true)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 在磁盘上按 hash 布局落一个真实文件，供 preview/download 流式读取
func writeStoredFile(t *testing.T, fileRoot, fileHash string) {
	t.Helper()
	dir := filepath.Join(fileRoot, fileHash[0:2], fileHash[2:4])
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileHash), []byte("data"), 0600))
}

var fileColumns = []string{"id", "user_id", "folder_id", "name", "size", "type", "file_hash"}

// PreviewFileByID 流式读取成功后写入 preview 日志
func TestPreviewFileByID_RecordsPreviewLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const fileHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fileRoot := t.TempDir()
	writeStoredFile(t, fileRoot, fileHash)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", fileHash))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "preview", "file", uint(5), "doc.pdf", "", "{}", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
		FileServiceOptions:     FileServiceOptions{FileStoragePath: fileRoot},
	}
	var buf bytes.Buffer
	err := svc.PreviewFileByID(5, 7, &buf, nil)

	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), buf.Bytes())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DownloadByIDs 下载单个文件成功后写入 download 日志
func TestDownloadByIDs_RecordsDownloadLogForFile(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const fileHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fileRoot := t.TempDir()
	writeStoredFile(t, fileRoot, fileHash)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", fileHash))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "download", "file", uint(5), "doc.pdf", "", "{}", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
		FileServiceOptions:     FileServiceOptions{FileStoragePath: fileRoot},
	}
	var buf bytes.Buffer
	err := svc.DownloadByIDs(7, 5, 0, &buf, nil)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DownloadByIDs 打包下载文件夹成功后写入 download 日志（detail.packaged）
func TestDownloadByIDs_RecordsPackagedLogForFolder(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	const fileHash = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fileRoot := t.TempDir()
	writeStoredFile(t, fileRoot, fileHash)
	folderColumns := []string{"id", "user_id", "parent_id", "name"}

	// DownloadByIDs 前置校验 + StreamFolderAsZip 内部再次读取根目录
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `folder_models`")).
		WithArgs(uint(3), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(folderColumns).AddRow(3, 7, 0, "资料"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `folder_models`")).
		WithArgs(uint(3), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(folderColumns).AddRow(3, 7, 0, "资料"))
	// GetChildrenByFolderIDAndUserID：Find 无 LIMIT，只有两个参数
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `folder_models`")).
		WithArgs(uint(3), uint(7)).
		WillReturnRows(sqlmock.NewRows(folderColumns))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(3), uint(7)).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(9, 7, 3, "doc.pdf", 4, "application/pdf", fileHash))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "download", "folder", uint(3), "资料", "", `{"packaged":true}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
		FileServiceOptions:     FileServiceOptions{FileStoragePath: fileRoot},
	}
	var buf bytes.Buffer
	err := svc.DownloadByIDs(7, 0, 3, &buf, nil)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// CreatePublicShareLink 首次创建链接后写入 share_link_create 日志
func TestCreatePublicShareLink_RecordsCreateLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `public_share_link_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `public_share_link_models`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "share_link_create", "file", uint(5), "doc.pdf", "", "{}", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	token, err := svc.CreatePublicShareLink(5, 7)

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// CreatePublicShareLink 复用已有链接时不产生新的分享行为，不写日志
func TestCreatePublicShareLink_ReusesExistingLinkWithoutLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `public_share_link_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "token", "file_id", "user_id"}).
			AddRow(1, "existing-token", 5, 7))

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	token, err := svc.CreatePublicShareLink(5, 7)

	assert.NoError(t, err)
	assert.Equal(t, "existing-token", token)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeletePublicShareLink 删除链接后写入 share_link_delete 日志
func TestDeletePublicShareLink_RecordsDeleteLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	// 硬删除：DELETE FROM，且单条 Delete 有隐式事务
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `public_share_link_models`")).
		WithArgs(uint(5), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "share_link_delete", "file", uint(5), "doc.pdf", "", "{}", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeletePublicShareLink(5, 7)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// CreatePickUpCode 创建文件取件码后写入 pickup_code_create 日志（detail.code）
func TestCreatePickUpCode_RecordsCreateLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `pick_up_code_models`")).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "pickup_code_create", "file", uint(5), "doc.pdf", "", `{"code":"ABC123"}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	fileID := uint(5)
	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	id, err := svc.CreatePickUpCode(7, &model.PickUpCodeModel{
		UserID: 7, Type: model.PickUpTargetTypeFile, FileID: &fileID,
		MaxDownload: 3, ExpireTime: time.Now().Add(time.Hour),
		Code: "ABC123", Status: model.PickUpCodeStatusActive,
	})

	assert.NoError(t, err)
	assert.Equal(t, uint(11), id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DeletePickUpCodeByID 先查取件码，硬删除后写入 pickup_code_delete 日志（detail.code）
func TestDeletePickUpCodeByID_RecordsDeleteLog(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pick_up_code_models`")).
		WithArgs(uint(11), uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "type", "file_id", "folder_id", "code", "status"}).
			AddRow(11, 7, model.PickUpTargetTypeFile, 5, nil, "ABC123", model.PickUpCodeStatusActive))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `pick_up_code_models`")).
		WithArgs(uint(11), uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 删除后尽力解析目标文件名称，用于日志快照
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "doc.pdf", 2048, "application/pdf", "hash-doc"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "pickup_code_delete", "file", uint(5), "doc.pdf", "", `{"code":"ABC123"}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 写日志成功后触发保留上限清理（未超量：cutoff 查询无记录，不执行删除）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(gorm.ErrRecordNotFound)

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.DeletePickUpCodeByID(7, 11)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// GetOperationLogs 返回映射后的列表项与总数（detail 原样透传，由前端解析）
func TestGetOperationLogs_ReturnsMappedListAndTotal(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	logColumns := []string{"id", "user_id", "action", "target_type", "target_id", "target_name", "folder_name", "detail", "created_at"}
	createdAt := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `operation_log_models`")).
		WithArgs(uint(7), 20).
		WillReturnRows(sqlmock.NewRows(logColumns).
			AddRow(10, 7, model.OperationLogActionUpload, model.OperationLogTargetTypeFile, uint(88), "hello.txt", "", `{"size":5}`, createdAt).
			AddRow(9, 7, model.OperationLogActionMkdir, model.OperationLogTargetTypeFolder, uint(3), "资料", "", "{}", createdAt))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `operation_log_models`")).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	resp, err := svc.GetOperationLogs(7, 1, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	assert.Len(t, resp.List, 2)
	assert.Equal(t, model.OperationLogActionUpload, resp.List[0].Action)
	assert.Equal(t, "hello.txt", resp.List[0].TargetName)
	assert.Equal(t, `{"size":5}`, resp.List[0].Detail)
	assert.Equal(t, "资料", resp.List[1].TargetName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// GetOperationLogs 列表查询失败时返回错误
func TestGetOperationLogs_ReturnsErrorOnQueryFailure(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `operation_log_models`")).
		WithArgs(uint(7), 20).
		WillReturnError(errors.New("db unavailable"))

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	resp, err := svc.GetOperationLogs(7, 1, 20)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 写日志成功后触发保留上限清理：超过 500 条时删除更旧的记录
func TestRenameByIDs_PrunesLogsBeyondKeep(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "old-name.txt", 100, "text/plain", "hash-old"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "rename", "file", uint(5), "new-name.txt", "", `{"old_name":"old-name.txt"}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 超过保留上限：第 501 新的日志 id=60，删除 id < 60 的全部记录
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(60))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `operation_log_models`")).
		WithArgs(uint(7), uint(60)).
		WillReturnResult(sqlmock.NewResult(0, 40))
	mock.ExpectCommit()

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.RenameByIDs(7, 5, 0, "new-name.txt")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 清理失败不影响主流程（与日志写入失败同策略：仅记错误）
func TestRenameByIDs_IgnoresLogPruneFailure(t *testing.T) {
	db, mock, cleanup := setupServiceMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `file_models`")).
		WithArgs(uint(5), uint(7), 1).
		WillReturnRows(sqlmock.NewRows(fileColumns).
			AddRow(5, 7, 0, "old-name.txt", 100, "text/plain", "hash-old"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `file_models`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `operation_log_models`")).
		WithArgs(uint(7), "rename", "file", uint(5), "new-name.txt", "", `{"old_name":"old-name.txt"}`, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// 清理查询失败：仅记错误，业务结果不受影响
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `operation_log_models`")).
		WithArgs(uint(7), 1, 500).
		WillReturnError(errors.New("prune unavailable"))

	svc := &fileService{
		FileRepository:         repository.NewFileRepository(db),
		OperationLogRepository: repository.NewOperationLogRepository(db),
	}
	err := svc.RenameByIDs(7, 5, 0, "new-name.txt")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
