package service

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
		"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"zhixu-backend/internal/dto"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/utils"
	"zhixu-backend/internal/vo"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPickupCodeExpired     = errors.New("pickup code expired")
	ErrPickupTargetNotFound  = errors.New("pickup target not found")
	ErrPickupEmptyFolder     = errors.New("empty folder")
	ErrPublicShareNotFound   = errors.New("public share not found")
	ErrInvalidFileName       = errors.New("invalid file name")
	ErrStorageQuotaExceeded  = errors.New("storage quota exceeded")
	ErrChunkSizeMismatch     = errors.New("chunk size mismatch")
	ErrChunkHashMismatch     = errors.New("chunk hash mismatch")
	ErrChunkConflict         = errors.New("chunk content conflicts with an existing upload")
	ErrUploadStateConflict   = errors.New("upload task is not accepting chunks")
	ErrUploadIncomplete      = errors.New("not all chunks uploaded")
	ErrInvalidMIMEType       = errors.New("invalid mime type")
	ErrUploadRequestConflict = errors.New("upload request conflicts with an existing idempotency key")
	// 新增错误类型，用于HTTP状态码映射
	ErrFileNotFound            = errors.New("file not found")
	ErrFolderNotFound          = errors.New("folder not found")
	ErrPermissionDenied        = errors.New("permission denied")
	errUploadTaskAlreadyExists = errors.New("upload task already exists")
)

var allowedMIMETypes = map[string]bool{
	"image/":          true,
	"video/":          true,
	"application/pdf": true,
	"application/zip": true,
	"text/":           true,
	// Office Open XML：加入知识库的 docx/xlsx 需要先能上传
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
}

var blockedMIMETypes = map[string]bool{
	"text/javascript":          true,
	"application/javascript":   true,
	"application/x-javascript": true,
	"application/ecmascript":   true,
	"text/ecmascript":          true,
}

func (s *fileService) IsAllowedMIMEType(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	if blockedMIMETypes[mimeType] {
		return false
	}
	if allowedMIMETypes[mimeType] {
		return true
	}
	for prefix := range allowedMIMETypes {
		if strings.HasSuffix(prefix, "/") && strings.HasPrefix(mimeType, prefix) {
			return true
		}
	}
	return false
}

func sanitizeFileName(name string) (string, error) {
	normalizedName, err := utils.NormalizeUploadFileName(name)
	if err != nil {
		return "", ErrInvalidFileName
	}
	return normalizedName, nil
}

func sanitizeStorageFileExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return ""
	}
	var cleaned strings.Builder
	for _, r := range ext {
		if r >= 32 && r != 127 {
			cleaned.WriteRune(r)
		}
	}
	ext = cleaned.String()
	if ext == "." {
		return ""
	}
	dangerousChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		if strings.Contains(ext, char) {
			return ""
		}
	}
	return ext
}

func buildUploadIdentity(task *model.UploadTask) (idempotencyKey, requestHash, normalizedName string, err error) {
	idempotencyKey, requestHash, normalizedName, err = utils.BuildUploadIdentity(task)
	if err != nil {
		return "", "", "", ErrInvalidFileName
	}
	return idempotencyKey, requestHash, normalizedName, nil
}

func containsChunk(chunks model.IntSlice, target int) bool {
	for _, chunk := range chunks {
		if chunk == target {
			return true
		}
	}
	return false
}

func withoutChunk(chunks model.IntSlice, target int) model.IntSlice {
	result := make(model.IntSlice, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk != target {
			result = append(result, chunk)
		}
	}
	return result
}

func validateZipEntryPath(entryPath string) error {
	if entryPath == "" {
		return errors.New("invalid zip entry path: empty path")
	}
	if strings.Contains(entryPath, "..") {
		return errors.New("invalid zip entry path: contains path traversal")
	}
	cleanPath := path.Clean(entryPath)
	if cleanPath == "." {
		return errors.New("invalid zip entry path: empty path")
	}
	if strings.HasPrefix(cleanPath, "/") {
		return errors.New("invalid zip entry path: absolute path not allowed")
	}
	for _, segment := range strings.Split(cleanPath, "/") {
		if segment == ".." {
			return errors.New("invalid zip entry path: contains path traversal")
		}
	}
	return nil
}

const (
	defaultStorageLimitBytes uint64 = 1024 * 1024 * 1024
	operationLogKeepCount    int    = 500
)

type FileServiceOptions struct {
	ChunkStoragePath string
	FileStoragePath  string
}

// FileService 定义文件服务的对外接口（用于 DI/替换实现）
type FileService interface {
	InitUploadFile(req *model.UploadTask) (task *model.UploadTask, err error)
	UploadFileChunkStream(userID uint, chunk *dto.UploadChunkReq, reader io.Reader, chunkSize int64) error
	IsAllowedMIMEType(mimeType string) bool
	MergeUploadedChunks(userID uint, taskID uint) (*vo.MergeUploadedChunksResp, error)
	GetDashboardOverview(userID uint, storageLimit uint64) (*dto.DashboardOverviewResp, error)
	GetListByFolderIDAndUserID(folderID uint, userID uint, page, pageSize int) ([]dto.FileListItem, error)
	GetListCountByFolderIDAndUserID(folderID uint, userID uint) (int64, error)
	GetOperationLogs(userID uint, page, pageSize int) (*vo.OperationLogListResp, error)
	MakeDirectory(folderID uint, name string, userID uint) (uint, error)
	RenameByIDs(userID uint, fileID, folderID uint, name string) error
	MoveByIDs(userID uint, fileID, folderID, targetFolderID uint) error
	DeleteByIDs(userID uint, fileID, folderID uint, deleteKnowledge ...bool) error
	CreatePickUpCode(userID uint, code *model.PickUpCodeModel) (uint, error)
	GetPickUpCodeListByUserID(userID uint, page int, pageSize int) ([]vo.PickUpCodeListItem, error)
	GetPickUpCodeListCountByUserID(userID uint) (int64, error)
	DeletePickUpCodeByID(userID uint, codeID uint) error
	CreatePublicShareLink(fileID uint, userID uint) (string, error)
	GetPublicShareLink(fileID uint, userID uint) (string, error)
	DeletePublicShareLink(fileID uint, userID uint) error
	OpenPublicShare(token string, writer io.Writer, setMeta func(fileName, contentType string)) error
	PreviewFileByID(fileID uint, userID uint, writer io.Writer, setMeta func(fileName, contentType string)) error
	DownloadByIDs(userID uint, fileID, folderID uint, writer io.Writer, setMeta func(fileName, contentType string)) error
	DownloadByPickUpCode(code string, writer io.Writer, setMeta func(fileName, contentType string)) error
}

// FileService 为对外暴露的接口，保持与原有实现一致

type fileService struct {
	FileRepository         *repository.FileRepository
	OperationLogRepository *repository.OperationLogRepository
	FileServiceOptions
	newMergeLeaseID func() (string, error)
	now             func() time.Time
}

type PickUpDownloadTarget struct {
	CodeID       uint
	UserID       uint
	Type         model.PickUpTargetType
	FilePath     string
	FolderID     uint
	DownloadName string
}

func (s *fileService) ensureStorageQuota(userID uint, additionalSize uint64) error {
	if additionalSize == 0 {
		return nil
	}
	storageUsed, err := s.FileRepository.GetStorageUsedByUserID(userID)
	if err != nil {
		return err
	}
	if storageUsed+additionalSize > defaultStorageLimitBytes {
		return ErrStorageQuotaExceeded
	}
	return nil
}

func (s *fileService) buildStorageDir(fileHash string) (string, error) {
	if len(fileHash) < 4 {
		return "", errors.New("invalid file hash")
	}
	return filepath.Join(s.FileStoragePath, fileHash[0:2], fileHash[2:4]), nil
}

func (s *fileService) buildCanonicalStoragePath(fileHash string) (string, error) {
	dir, err := s.buildStorageDir(fileHash)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileHash), nil
}

func NewFileService(fileRepository *repository.FileRepository, operationLogRepository *repository.OperationLogRepository, options FileServiceOptions) FileService {
	return &fileService{
		FileRepository:         fileRepository,
		OperationLogRepository: operationLogRepository,
		FileServiceOptions:     options,
	}
}

func (s *fileService) nextMergeLeaseID() (string, error) {
	if s.newMergeLeaseID != nil {
		return s.newMergeLeaseID()
	}
	return generateRandomHex(16)
}

func (s *fileService) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

// folderName 尽力解析目录名；根目录或查询失败时返回空串，仅用于日志快照。
func (s *fileService) folderName(userID uint, folderID uint) string {
	if folderID == 0 {
		return ""
	}
	folder, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID)
	if err != nil {
		return ""
	}
	return folder.Name
}

// recordOperation 以快照形式落一条操作日志（folderID 现查目录名），具体见 writeOperationLog。
func (s *fileService) recordOperation(userID uint, action model.OperationLogAction, targetType model.OperationLogTargetType, targetID uint, targetName string, folderID uint, detail map[string]any) {
	if s.OperationLogRepository == nil {
		return
	}
	writeOperationLog(s.OperationLogRepository, userID, action, targetType, targetID, targetName, s.folderName(userID, folderID), detail)
}

// Completed tasks only deduplicate while their logical file still exists.
// Retire the key with a CAS so concurrent reuploads still share one new task.
func (s *fileService) reusableUploadTask(task *model.UploadTask, requestHash string) (*model.UploadTask, error) {
	if task.Status == model.UploadStatusCompleted {
		var fileErr error = gorm.ErrRecordNotFound
		if task.FileID != nil {
			_, fileErr = s.FileRepository.GetFileByFileIDAndUserID(*task.FileID, task.UserID)
		}
		if fileErr != nil && !errors.Is(fileErr, gorm.ErrRecordNotFound) {
			return nil, fileErr
		}
		if errors.Is(fileErr, gorm.ErrRecordNotFound) {
			key := sha256.Sum256([]byte(fmt.Sprintf("retired-upload:%d:%s", task.ID, task.IdempotencyKey)))
			err := s.FileRepository.DB.Model(&model.UploadTask{}).
				Where("id = ? AND user_id = ? AND idempotency_key = ? AND status = ?",
					task.ID, task.UserID, task.IdempotencyKey, model.UploadStatusCompleted).
				Update("idempotency_key", hex.EncodeToString(key[:])).Error
			return nil, err
		}
	}
	if task.RequestHash != requestHash {
		return nil, ErrUploadRequestConflict
	}
	return task, nil
}

func (s *fileService) InitUploadFile(req *model.UploadTask) (task *model.UploadTask, err error) {
	idempotencyKey, requestHash, normalizedName, err := buildUploadIdentity(req)
	if err != nil {
		return nil, err
	}
	req.IdempotencyKey = idempotencyKey
	req.RequestHash = requestHash
	req.FileName = normalizedName
	req.FileHash = strings.ToLower(req.FileHash)

	existingTask, err := s.FileRepository.GetUploadTaskByIdempotencyKey(idempotencyKey)
	if err == nil {
		reusable, reuseErr := s.reusableUploadTask(existingTask, requestHash)
		if reuseErr != nil || reusable != nil {
			return reusable, reuseErr
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if req.FolderID > 0 {
		if _, err := s.FileRepository.GetFolderByFolderIDAndUserID(req.FolderID, req.UserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrFolderNotFound
			}
			return nil, err
		}
	}

	var instantFile *model.FileModel
	targetFile, err := s.FileRepository.GetFileByHashAndUserIDAndFolderID(req.FileHash, req.UserID, req.FolderID)
	switch {
	case err == nil:
		req.Status = model.UploadStatusCompleted
		req.FileID = &targetFile.ID
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	default:
		_, blobErr := s.FileRepository.GetFileByHash(req.FileHash)
		if blobErr == nil {
			if err := s.ensureStorageQuota(req.UserID, req.FileSize); err != nil {
				return nil, err
			}
			req.Status = model.UploadStatusCompleted
			instantFile = &model.FileModel{
				UserID: req.UserID, FolderID: req.FolderID, Name: req.FileName,
				Size: req.FileSize, Type: req.FileType, FileHash: req.FileHash,
			}
		} else if errors.Is(blobErr, gorm.ErrRecordNotFound) {
			req.Status = model.UploadStatusUploading
		} else {
			return nil, blobErr
		}
	}

	created := false
	err = s.FileRepository.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(req)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errUploadTaskAlreadyExists
		}
		created = true
		if instantFile == nil {
			return nil
		}
		if err := tx.Create(instantFile).Error; err != nil {
			return err
		}
		req.FileID = &instantFile.ID
		return tx.Model(req).Update("file_id", instantFile.ID).Error
	})
	if errors.Is(err, errUploadTaskAlreadyExists) {
		existingTask, getErr := s.FileRepository.GetUploadTaskByIdempotencyKey(idempotencyKey)
		if getErr != nil {
			return nil, getErr
		}
		reusable, reuseErr := s.reusableUploadTask(existingTask, requestHash)
		if reuseErr != nil || reusable != nil {
			return reusable, reuseErr
		}
		return s.InitUploadFile(req)
	}
	if err != nil {
		return nil, err
	}
	if created && req.Status == model.UploadStatusUploading {
		chunkDir := filepath.Join(s.ChunkStoragePath, strconv.FormatUint(uint64(req.ID), 10))
		if err := os.MkdirAll(chunkDir, 0755); err != nil {
			return nil, err
		}
	}
	if instantFile != nil {
		s.recordOperation(req.UserID, model.OperationLogActionUploadInstant, model.OperationLogTargetTypeFile, instantFile.ID, instantFile.Name, instantFile.FolderID, map[string]any{"size": instantFile.Size})
	}
	return req, nil
}

func (s *fileService) UploadFileChunkStream(userID uint, req *dto.UploadChunkReq, reader io.Reader, chunkSize int64) error {
	return s.FileRepository.DB.Transaction(func(tx *gorm.DB) error {
		var task model.UploadTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", req.TaskID, userID).
			First(&task).Error; err != nil {
			return err
		}

		if req.ChunkIndex < 0 || req.ChunkIndex >= task.TotalChunks {
			return ErrChunkSizeMismatch
		}

		expectedChunkSize := int64(task.ChunkSize)
		if req.ChunkIndex == task.TotalChunks-1 {
			expectedChunkSize = int64(task.FileSize) - int64(req.ChunkIndex)*int64(task.ChunkSize)
		}

		if chunkSize != expectedChunkSize {
			return ErrChunkSizeMismatch
		}
		req.ChunkHash = strings.ToLower(strings.TrimSpace(req.ChunkHash))
		chunkDir := filepath.Join(s.ChunkStoragePath, strconv.FormatUint(uint64(task.ID), 10))
		chunkPath := filepath.Join(chunkDir, strconv.Itoa(req.ChunkIndex))

		var existingChunk model.UploadChunk
		err := tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).
			First(&existingChunk).Error
		if err == nil {
			if existingChunk.ChunkHash != req.ChunkHash || existingChunk.Size != chunkSize {
				return ErrChunkConflict
			}
			if task.Status == model.UploadStatusCompleted {
				return nil
			}
			info, statErr := os.Stat(chunkPath)
			if statErr == nil && info.Size() == chunkSize {
				valid, verifyErr := utils.VerifyFileSHA256(chunkPath, req.ChunkHash)
				if verifyErr != nil {
					return verifyErr
				}
				if valid {
					if containsChunk(task.UploadedChunks, req.ChunkIndex) {
						return nil
					}
					if task.Status != model.UploadStatusUploading {
						return ErrUploadStateConflict
					}
					task.UploadedChunks = append(task.UploadedChunks, req.ChunkIndex)
					sort.Ints(task.UploadedChunks)
					return tx.Model(&task).Update("uploaded_chunks", task.UploadedChunks).Error
				}
			}
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return statErr
			}
			if task.Status != model.UploadStatusUploading {
				return ErrUploadStateConflict
			}
			if err := tx.Delete(&existingChunk).Error; err != nil {
				return err
			}
			task.UploadedChunks = withoutChunk(task.UploadedChunks, req.ChunkIndex)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if task.Status != model.UploadStatusUploading {
			return ErrUploadStateConflict
		}

		if err := os.MkdirAll(chunkDir, 0755); err != nil {
			return err
		}

		tmpToken, err := generateRandomHex(8)
		if err != nil {
			return err
		}
		tmpPath := chunkPath + ".tmp-" + tmpToken
		defer os.Remove(tmpPath)

		dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		written, err := io.Copy(dst, reader)
		if err != nil {
			dst.Close()
			return err
		}
		dst.Close()

		if written != chunkSize {
			os.Remove(tmpPath)
			return ErrChunkSizeMismatch
		}

		ok, err := utils.VerifyFileSHA256(tmpPath, req.ChunkHash)
		if err != nil {
			return err
		}
		if !ok {
			return ErrChunkHashMismatch
		}
		if err := os.Rename(tmpPath, chunkPath); err != nil {
			return err
		}

		if !containsChunk(task.UploadedChunks, req.ChunkIndex) {
			task.UploadedChunks = append(task.UploadedChunks, req.ChunkIndex)
			sort.Ints(task.UploadedChunks)
		}

		chunkRecord := &model.UploadChunk{
			TaskID: req.TaskID, ChunkIndex: req.ChunkIndex,
			ChunkHash: req.ChunkHash, Size: chunkSize,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(chunkRecord)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var winner model.UploadChunk
			if err := tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).First(&winner).Error; err != nil {
				return err
			}
			if winner.ChunkHash != req.ChunkHash || winner.Size != chunkSize {
				return ErrChunkConflict
			}
		}

		return tx.Model(&task).Update("uploaded_chunks", task.UploadedChunks).Error
	})
}

func (s *fileService) MergeUploadedChunks(userID uint, taskID uint) (*vo.MergeUploadedChunksResp, error) {
	leaseID, err := s.nextMergeLeaseID()
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	leaseExpiresAt := now.Add(10 * time.Minute)
	acquire := s.FileRepository.DB.Model(&model.UploadTask{}).
		Where("id = ? AND user_id = ? AND (status = ? OR (status = ? AND (merge_lease_expires_at IS NULL OR merge_lease_expires_at < ?)))",
			taskID, userID, model.UploadStatusUploading, model.UploadStatusMerging, now).
		Updates(map[string]interface{}{
			"status":                 model.UploadStatusMerging,
			"merge_lease_id":         leaseID,
			"merge_lease_expires_at": leaseExpiresAt,
		})
	if acquire.Error != nil {
		return nil, acquire.Error
	}
	if acquire.RowsAffected == 0 {
		task, err := s.FileRepository.GetUploadTaskByIDAndUserID(taskID, userID)
		if err != nil {
			return nil, err
		}
		switch task.Status {
		case model.UploadStatusMerging:
			return &vo.MergeUploadedChunksResp{Status: model.UploadStatusMerging}, nil
		case model.UploadStatusCompleted:
			return &vo.MergeUploadedChunksResp{Status: model.UploadStatusCompleted, FileID: task.FileID}, nil
		default:
			return nil, ErrUploadStateConflict
		}
	}

	mergeCompleted := false
	defer func() {
		if mergeCompleted {
			return
		}
		_ = s.FileRepository.DB.Model(&model.UploadTask{}).
			Where("id = ? AND user_id = ? AND merge_lease_id = ?", taskID, userID, leaseID).
			Updates(map[string]interface{}{
				"status":                 model.UploadStatusUploading,
				"merge_lease_id":         "",
				"merge_lease_expires_at": nil,
			}).Error
	}()

	task, err := s.FileRepository.GetUploadTaskByIDAndUserID(taskID, userID)
	if err != nil {
		return nil, err
	}
	if task.MergeLeaseID != leaseID || task.Status != model.UploadStatusMerging {
		return nil, ErrUploadStateConflict
	}
	if !utils.HasAllChunks(task.UploadedChunks, task.TotalChunks) {
		return nil, ErrUploadIncomplete
	}
	if len(task.FileHash) < 4 {
		return nil, errors.New("invalid file hash")
	}

	var totalChunkSize int64
	chunkDir := filepath.Join(s.ChunkStoragePath, strconv.FormatUint(uint64(task.ID), 10))
	for i := 0; i < task.TotalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, strconv.Itoa(i))
		info, err := os.Stat(chunkPath)
		if err != nil {
			return nil, errors.New("chunk file not found")
		}
		totalChunkSize += info.Size()
	}
	if uint64(totalChunkSize) != task.FileSize {
		return nil, ErrChunkSizeMismatch
	}
	if err := s.ensureStorageQuota(userID, task.FileSize); err != nil {
		return nil, err
	}

	dirPath, err := s.buildStorageDir(task.FileHash)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, err
	}
	filePath, err := s.buildCanonicalStoragePath(task.FileHash)
	if err != nil {
		return nil, err
	}
	tmpPath := filePath + ".merge-" + leaseID
	defer os.Remove(tmpPath)
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	for i := 0; i < task.TotalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, strconv.Itoa(i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		_, copyErr := io.Copy(file, chunkFile)
		closeErr := chunkFile.Close()
		if copyErr != nil {
			_ = file.Close()
			return nil, copyErr
		}
		if closeErr != nil {
			_ = file.Close()
			return nil, closeErr
		}
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	ok, err := utils.VerifyFileSHA256(tmpPath, task.FileHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("merged file hash mismatch")
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return nil, err
	}

	var fileID uint
	fileCreated := false
	err = s.FileRepository.DB.Transaction(func(tx *gorm.DB) error {
		var lockedTask model.UploadTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", taskID, userID).First(&lockedTask).Error; err != nil {
			return err
		}
		if lockedTask.Status == model.UploadStatusCompleted && lockedTask.FileID != nil {
			fileID = *lockedTask.FileID
			return nil
		}
		if lockedTask.Status != model.UploadStatusMerging || lockedTask.MergeLeaseID != leaseID {
			return ErrUploadStateConflict
		}
		fileModel := &model.FileModel{
			UserID: lockedTask.UserID, FolderID: lockedTask.FolderID, Name: lockedTask.FileName,
			Size: lockedTask.FileSize, Type: lockedTask.FileType, FileHash: lockedTask.FileHash,
		}
		if err := tx.Create(fileModel).Error; err != nil {
			return err
		}
		fileID = fileModel.ID
		fileCreated = true
		return tx.Model(&lockedTask).Updates(map[string]interface{}{
			"status":                 model.UploadStatusCompleted,
			"file_id":                fileID,
			"merge_lease_id":         "",
			"merge_lease_expires_at": nil,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	mergeCompleted = true
	if fileCreated {
		s.recordOperation(userID, model.OperationLogActionUpload, model.OperationLogTargetTypeFile, fileID, task.FileName, task.FolderID, map[string]any{"size": task.FileSize})
	}
	return &vo.MergeUploadedChunksResp{Status: model.UploadStatusCompleted, FileID: &fileID}, nil
}

func (s *fileService) GetDashboardOverview(userID uint, storageLimit uint64) (*dto.DashboardOverviewResp, error) {
	if storageLimit == 0 {
		storageLimit = defaultStorageLimitBytes
	}

	storageUsed, err := s.FileRepository.GetStorageUsedByUserID(userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.FileRepository.GetFileStatsByUserID(userID)
	if err != nil {
		return nil, err
	}
	statMap := make(map[string]dto.DashboardFileStatItem, len(stats))
	for _, item := range stats {
		statMap[item.Type] = item
	}
	orderedTypes := []string{"image", "video", "audio", "document", "other"}
	fileStats := make([]dto.DashboardFileStatItem, 0, len(orderedTypes))
	for _, fileType := range orderedTypes {
		stat, ok := statMap[fileType]
		if !ok {
			stat = dto.DashboardFileStatItem{
				Type:  fileType,
				Count: 0,
				Size:  0,
			}
		}
		fileStats = append(fileStats, stat)
	}

	usedPercent := int(storageUsed * 100 / storageLimit)
	if usedPercent > 100 {
		usedPercent = 100
	}
	storageLeft := uint64(0)
	if storageLimit > storageUsed {
		storageLeft = storageLimit - storageUsed
	}

	return &dto.DashboardOverviewResp{
		StorageUsed:        storageUsed,
		StorageTotal:       storageLimit,
		StorageLeft:        storageLeft,
		StorageUsedPercent: usedPercent,
		FileStats:          fileStats,
	}, nil
}

func (s *fileService) GetListByFolderIDAndUserID(folderID uint, userID uint, page, pageSize int) ([]dto.FileListItem, error) {
	list, err := s.FileRepository.GetListByFolderIDAndUserID(folderID, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *fileService) GetListCountByFolderIDAndUserID(folderID uint, userID uint) (int64, error) {
	count, err := s.FileRepository.GetListCountByFolderIDAndUserID(folderID, userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetOperationLogs 查询当前用户的操作日志（按时间倒序分页，附带总数）
func (s *fileService) GetOperationLogs(userID uint, page, pageSize int) (*vo.OperationLogListResp, error) {
	logs, err := s.OperationLogRepository.GetOperationLogsByUserID(userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]vo.OperationLogListItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, vo.OperationLogListItem{
			ID:         l.ID,
			Action:     l.Action,
			TargetType: l.TargetType,
			TargetID:   l.TargetID,
			TargetName: l.TargetName,
			FolderName: l.FolderName,
			Detail:     l.Detail,
			CreatedAt:  l.CreatedAt,
		})
	}
	total, err := s.OperationLogRepository.CountOperationLogsByUserID(userID)
	if err != nil {
		return nil, err
	}
	return &vo.OperationLogListResp{List: items, Total: total}, nil
}

func (s *fileService) MakeDirectory(folderID uint, name string, userID uint) (uint, error) {
	cleanedName, err := utils.SanitizeFileName(name)
	if err != nil {
		return 0, err
	}
	if folderID > 0 {
		if _, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrFolderNotFound
			}
			return 0, err
		}
	}
	id, err := s.FileRepository.MakeDirectory(folderID, cleanedName, userID)
	if err != nil {
		return 0, err
	}
	s.recordOperation(userID, model.OperationLogActionMkdir, model.OperationLogTargetTypeFolder, id, cleanedName, folderID, nil)
	return id, nil
}

func (s *fileService) RenameByIDs(userID uint, fileID, folderID uint, name string) error {
	cleanedName, err := utils.SanitizeFileName(name)
	if err != nil {
		return fmt.Errorf("文件名无效: %w", err)
	}
	if fileID > 0 && folderID > 0 {
		return errors.New("invalid rename target")
	}
	if fileID == 0 && folderID == 0 {
		return errors.New("missing rename target")
	}
	if fileID > 0 {
		oldFile, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("重命名文件失败: %w", err)
		}
		if err := s.FileRepository.RenameFileByIDAndUserID(fileID, userID, cleanedName); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("重命名文件失败: %w", err)
		}
		s.recordOperation(userID, model.OperationLogActionRename, model.OperationLogTargetTypeFile, fileID, cleanedName, oldFile.FolderID, map[string]any{"old_name": oldFile.Name})
		return nil
	}
	oldFolder, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFolderNotFound
		}
		return fmt.Errorf("重命名文件夹失败: %w", err)
	}
	if err := s.FileRepository.RenameFolderByIDAndUserID(folderID, userID, cleanedName); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFolderNotFound
		}
		return fmt.Errorf("重命名文件夹失败: %w", err)
	}
	s.recordOperation(userID, model.OperationLogActionRename, model.OperationLogTargetTypeFolder, folderID, cleanedName, oldFolder.ParentID, map[string]any{"old_name": oldFolder.Name})
	return nil
}

func (s *fileService) MoveByIDs(userID uint, fileID, folderID, targetFolderID uint) error {
	if fileID > 0 && folderID > 0 {
		return errors.New("invalid move target")
	}
	if fileID == 0 && folderID == 0 {
		return errors.New("missing move target")
	}

	if targetFolderID > 0 {
		if _, err := s.FileRepository.GetFolderByFolderIDAndUserID(targetFolderID, userID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFolderNotFound
			}
			return fmt.Errorf("移动失败: %w", err)
		}
	}

	if fileID > 0 {
		movedFile, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("移动失败: %w", err)
		}
		if err := s.FileRepository.MoveFileByIDAndUserID(fileID, userID, targetFolderID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("移动文件失败: %w", err)
		}
		s.recordOperation(userID, model.OperationLogActionMove, model.OperationLogTargetTypeFile, fileID, movedFile.Name, targetFolderID, map[string]any{
			"from": s.folderName(userID, movedFile.FolderID),
			"to":   s.folderName(userID, targetFolderID),
		})
		return nil
	}

	sourceFolder, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFolderNotFound
		}
		return fmt.Errorf("移动文件夹失败: %w", err)
	}
	if sourceFolder.ID == targetFolderID {
		return errors.New("cannot move folder into itself")
	}

	// 防止把文件夹移动到自己的子孙目录中，避免形成环。
	current := targetFolderID
	for current != 0 {
		if current == sourceFolder.ID {
			return errors.New("cannot move folder into child folder")
		}
		parent, err := s.FileRepository.GetFolderByFolderIDAndUserID(current, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFolderNotFound
			}
			return fmt.Errorf("移动文件夹失败: %w", err)
		}
		current = parent.ParentID
	}

	err = s.FileRepository.MoveFolderByIDAndUserID(folderID, userID, targetFolderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFolderNotFound
		}
		return fmt.Errorf("移动文件夹失败: %w", err)
	}
	s.recordOperation(userID, model.OperationLogActionMove, model.OperationLogTargetTypeFolder, folderID, sourceFolder.Name, targetFolderID, map[string]any{
		"from": s.folderName(userID, sourceFolder.ParentID),
		"to":   s.folderName(userID, targetFolderID),
	})
	return nil
}

func (s *fileService) DeleteByIDs(userID uint, fileID, folderID uint, deleteKnowledge ...bool) error {
	removeKnowledge := len(deleteKnowledge) > 0 && deleteKnowledge[0]
	if fileID > 0 && folderID > 0 {
		return errors.New("invalid delete target")
	}
	if fileID == 0 && folderID == 0 {
		return errors.New("missing delete target")
	}

	if fileID > 0 {
		targetFile, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("删除文件失败: %w", err)
		}
		knowledgeRemoved := 0
		deleteFile := func() error {
			if !removeKnowledge {
				return s.FileRepository.DeleteFileByIDAndUserID(fileID, userID)
			}
			return s.FileRepository.DB.Transaction(func(tx *gorm.DB) error {
				if err := (&repository.FileRepository{DB: tx}).DeleteFileByIDAndUserID(fileID, userID); err != nil {
					return err
				}
				query := tx.Model(&model.KnowledgeItem{}).Where("user_id = ? AND source_file_id = ?", userID, fileID)
				removed, err := repository.DeleteKnowledgeRows(tx, userID, query)
				knowledgeRemoved = removed
				return err
			})
		}
		if err := deleteFile(); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("删除文件失败: %w", err)
		}
		// 实体已删除，日志只依赖事前捕获的名称/目录快照
		detail := map[string]any{"size": targetFile.Size}
		if knowledgeRemoved > 0 {
			detail["knowledge_removed"] = knowledgeRemoved
		}
		s.recordOperation(userID, model.OperationLogActionDelete, model.OperationLogTargetTypeFile, fileID, targetFile.Name, targetFile.FolderID, detail)
		return nil
	}

	var deletedRoot *model.FolderModel
	knowledgeRemoved := 0
	err := s.FileRepository.DB.Transaction(func(tx *gorm.DB) error {
		txRepo := &repository.FileRepository{DB: tx}

		rootFolder, err := txRepo.GetFolderByFolderIDAndUserID(folderID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFolderNotFound
			}
			return fmt.Errorf("删除文件夹失败: %w", err)
		}
		deletedRoot = rootFolder
		folderIDs := []uint{rootFolder.ID}
		queue := []uint{rootFolder.ID}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			children, err := txRepo.GetDirectChildFoldersByParentAndUserID(current, userID)
			if err != nil {
				return fmt.Errorf("删除文件夹失败: %w", err)
			}
			for _, child := range children {
				folderIDs = append(folderIDs, child.ID)
				queue = append(queue, child.ID)
			}
		}

		if err := txRepo.DeleteFilesByFolderIDsAndUserID(folderIDs, userID); err != nil {
			return fmt.Errorf("删除文件失败: %w", err)
		}
		if removeKnowledge {
			// Match current file membership, including the files just soft-deleted.
			files := tx.Unscoped().Model(&model.FileModel{}).Select("id").Where("user_id = ? AND folder_id IN ?", userID, folderIDs)
			query := tx.Model(&model.KnowledgeItem{}).Where("user_id = ? AND source_file_id IN (?)", userID, files)
			removed, err := repository.DeleteKnowledgeRows(tx, userID, query)
			if err != nil {
				return err
			}
			knowledgeRemoved = removed
		}
		return txRepo.DeleteFoldersByIDsAndUserID(folderIDs, userID)
	})
	if err != nil {
		return err
	}
	if deletedRoot != nil {
		var detail map[string]any
		if knowledgeRemoved > 0 {
			detail = map[string]any{"knowledge_removed": knowledgeRemoved}
		}
		s.recordOperation(userID, model.OperationLogActionDelete, model.OperationLogTargetTypeFolder, deletedRoot.ID, deletedRoot.Name, deletedRoot.ParentID, detail)
	}
	return nil
}

func (s *fileService) CreatePickUpCode(userID uint, code *model.PickUpCodeModel) (uint, error) {
	if code.UserID != userID {
		return 0, ErrPermissionDenied
	}
	if code.MaxDownload == 0 || time.Now().After(code.ExpireTime) {
		return 0, ErrPickupCodeExpired
	}
	var targetFile *model.FileModel
	var targetFolder *model.FolderModel
	switch code.Type {
	case model.PickUpTargetTypeFile:
		if code.FileID == nil || code.FolderID != nil {
			return 0, ErrPickupTargetNotFound
		}
		file, err := s.FileRepository.GetFileByFileIDAndUserID(*code.FileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrPickupTargetNotFound
			}
			return 0, err
		}
		targetFile = file
	case model.PickUpTargetTypeFolder:
		if code.FolderID == nil || code.FileID != nil {
			return 0, ErrPickupTargetNotFound
		}
		folder, err := s.FileRepository.GetFolderByFolderIDAndUserID(*code.FolderID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, ErrPickupTargetNotFound
			}
			return 0, err
		}
		targetFolder = folder
	default:
		return 0, ErrPickupTargetNotFound
	}
	id, err := s.FileRepository.CreatePickUpCode(code)
	if err != nil {
		return 0, err
	}
	switch {
	case targetFile != nil:
		s.recordOperation(userID, model.OperationLogActionPickupCodeCreate, model.OperationLogTargetTypeFile, *code.FileID, targetFile.Name, targetFile.FolderID, map[string]any{"code": code.Code})
	case targetFolder != nil:
		s.recordOperation(userID, model.OperationLogActionPickupCodeCreate, model.OperationLogTargetTypeFolder, *code.FolderID, targetFolder.Name, targetFolder.ParentID, map[string]any{"code": code.Code})
	}
	return id, nil
}

func (s *fileService) GetPickUpCodeListByUserID(userID uint, page, pageSize int) ([]vo.PickUpCodeListItem, error) {
	// 先批量回写已失效取件码的状态，让数据库与判定口径（过期时间 / 下载上限）保持一致。
	// 回写是尽力而为：失败不阻断列表，组装 VO 时仍会按有效状态兜底计算。
	if _, err := s.FileRepository.ExpireOutdatedPickUpCodes(userID, s.currentTime()); err != nil {
		log.Warn().Err(err).Msg("回写过期取件码状态失败")
	}

	// 先获取分页的 pickup 码列表
	list, err := s.FileRepository.GetPickUpCodeListByUserIDAndPage(userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 批量聚合名称：先收集需要的 FileID / FolderID
	var fileIDs []uint
	var folderIDs []uint
	for _, item := range list {
		if item.FileID != nil {
			fileIDs = append(fileIDs, *item.FileID)
		}
		if item.FolderID != nil {
			folderIDs = append(folderIDs, *item.FolderID)
		}
	}

	// 通过批量查询获得名称映射，避免 N+1 查询
	fileMap := make(map[uint]string)
	if len(fileIDs) > 0 {
		files, err := s.FileRepository.GetFilesByIDs(fileIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			fileMap[f.ID] = f.Name
		}
	}
	folderMap := make(map[uint]string)
	if len(folderIDs) > 0 {
		folders, err := s.FileRepository.GetFoldersByIDs(folderIDs)
		if err != nil {
			return nil, err
		}
		for _, fd := range folders {
			folderMap[fd.ID] = fd.Name
		}
	}

	now := s.currentTime()
	var voList []vo.PickUpCodeListItem
	for _, item := range list {
		var name string
		if item.FileID != nil {
			if n, ok := fileMap[*item.FileID]; ok {
				name = n
			}
		} else if item.FolderID != nil {
			if n, ok := folderMap[*item.FolderID]; ok {
				name = n
			}
		}
		// 兜底计算有效状态：到达过期时间或下载次数打满即视为 Expired，
		// 与 ResolveActivePickUpCode / IncrementDownloadAndMaybeExpire 的判定口径一致。
		status := item.Status
		if status == model.PickUpCodeStatusActive &&
			(now.After(item.ExpireTime) || item.Download >= item.MaxDownload) {
			status = model.PickUpCodeStatusExpire
		}
		voList = append(voList, vo.PickUpCodeListItem{
			ID:          item.ID,
			Code:        item.Code,
			FileID:      item.FileID,
			FolderID:    item.FolderID,
			Name:        name,
			Type:        item.Type,
			Download:    int(item.Download),
			MaxDownload: int(item.MaxDownload),
			ExpireTime:  item.ExpireTime,
			CreatedAt:   item.CreatedAt,
			Status:      status,
		})
	}
	return voList, nil
}

func (s *fileService) GetPickUpCodeListCountByUserID(userID uint) (int64, error) {
	count, err := s.FileRepository.GetPickUpCodeListCountByUserID(userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *fileService) DeletePickUpCodeByID(userID uint, codeID uint) error {
	code, err := s.FileRepository.GetPickUpCodeByIDAndUserID(codeID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPickupTargetNotFound
		}
		return fmt.Errorf("删除取件码失败: %w", err)
	}
	err = s.FileRepository.DeletePickUpCodeByIDAndUserID(codeID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPickupTargetNotFound
		}
		return fmt.Errorf("删除取件码失败: %w", err)
	}
	// 目标可能已被删除，名称尽力解析；取件码本身已在 detail 中留存
	targetType := model.OperationLogTargetTypeFolder
	var targetID uint
	var targetName string
	var folderID uint
	if code.FileID != nil {
		targetType = model.OperationLogTargetTypeFile
		targetID = *code.FileID
		if target, err := s.FileRepository.GetFileByFileIDAndUserID(*code.FileID, userID); err == nil {
			targetName = target.Name
			folderID = target.FolderID
		}
	} else if code.FolderID != nil {
		targetID = *code.FolderID
		if target, err := s.FileRepository.GetFolderByFolderIDAndUserID(*code.FolderID, userID); err == nil {
			targetName = target.Name
			folderID = target.ParentID
		}
	}
	s.recordOperation(userID, model.OperationLogActionPickupCodeDelete, targetType, targetID, targetName, folderID, map[string]any{"code": code.Code})
	return nil
}

func (s *fileService) CreatePublicShareLink(fileID uint, userID uint) (string, error) {
	fileModel, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("创建分享链接失败: %w", err)
	}
	existing, err := s.FileRepository.GetPublicShareLinkByFileIDAndUserID(fileID, userID)
	if err == nil && existing != nil {
		// 已存在分享链接，直接复用，不产生新的分享行为
		return existing.Token, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	token, err := generatePublicShareToken()
	if err != nil {
		return "", err
	}
	link := &model.PublicShareLinkModel{
		Token:  token,
		FileID: fileID,
		UserID: userID,
	}
	if err := s.FileRepository.CreatePublicShareLink(link); err != nil {
		return "", err
	}
	s.recordOperation(userID, model.OperationLogActionShareLinkCreate, model.OperationLogTargetTypeFile, fileID, fileModel.Name, fileModel.FolderID, nil)
	return token, nil
}

func (s *fileService) GetPublicShareLink(fileID uint, userID uint) (string, error) {
	_, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("获取分享链接失败: %w", err)
	}
	link, err := s.FileRepository.GetPublicShareLinkByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrPublicShareNotFound
		}
		return "", err
	}
	return link.Token, nil
}

func (s *fileService) DeletePublicShareLink(fileID uint, userID uint) error {
	fileModel, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("删除分享链接失败: %w", err)
	}
	err = s.FileRepository.DeletePublicShareLinkByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPublicShareNotFound
		}
		return err
	}
	s.recordOperation(userID, model.OperationLogActionShareLinkDelete, model.OperationLogTargetTypeFile, fileID, fileModel.Name, fileModel.FolderID, nil)
	return nil
}

func (s *fileService) OpenPublicShare(token string, writer io.Writer, setMeta func(fileName, contentType string)) error {
	link, err := s.FileRepository.GetPublicShareLinkByToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPublicShareNotFound
		}
		return err
	}
	fileModel, err := s.FileRepository.GetFileByID(link.FileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPublicShareNotFound
		}
		return err
	}
	filePath, err := s.BuildFileAbsolutePath(fileModel)
	if err != nil {
		return err
	}
	contentType := mime.TypeByExtension(filepath.Ext(fileModel.Name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if setMeta != nil {
		setMeta(fileModel.Name, contentType)
	}
	return s.StreamSingleFile(filePath, writer)
}

func (s *fileService) PreviewFileByID(fileID uint, userID uint, writer io.Writer, setMeta func(fileName, contentType string)) error {
	fileModel, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("预览文件失败: %w", err)
	}
	filePath, err := s.BuildFileAbsolutePath(fileModel)
	if err != nil {
		return err
	}
	contentType := mime.TypeByExtension(filepath.Ext(fileModel.Name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if setMeta != nil {
		setMeta(fileModel.Name, contentType)
	}
	if err := s.StreamSingleFile(filePath, writer); err != nil {
		return err
	}
	s.recordOperation(userID, model.OperationLogActionPreview, model.OperationLogTargetTypeFile, fileID, fileModel.Name, fileModel.FolderID, nil)
	return nil
}

func (s *fileService) DownloadByIDs(userID uint, fileID, folderID uint, writer io.Writer, setMeta func(fileName, contentType string)) error {
	if fileID > 0 && folderID > 0 {
		return errors.New("invalid download target")
	}
	if fileID == 0 && folderID == 0 {
		return errors.New("missing download target")
	}

	if fileID > 0 {
		fileModel, err := s.FileRepository.GetFileByFileIDAndUserID(fileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("下载文件失败: %w", err)
		}
		filePath, err := s.BuildFileAbsolutePath(fileModel)
		if err != nil {
			return err
		}
		contentType := mime.TypeByExtension(filepath.Ext(fileModel.Name))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if setMeta != nil {
			setMeta(fileModel.Name, contentType)
		}
		if err := s.StreamSingleFile(filePath, writer); err != nil {
			return err
		}
		s.recordOperation(userID, model.OperationLogActionDownload, model.OperationLogTargetTypeFile, fileID, fileModel.Name, fileModel.FolderID, nil)
		return nil
	}

	folderModel, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFolderNotFound
		}
		return fmt.Errorf("下载文件夹失败: %w", err)
	}
	if setMeta != nil {
		setMeta(folderModel.Name+".zip", "application/zip")
	}
	if err := s.StreamFolderAsZip(userID, folderModel.ID, writer); err != nil {
		return err
	}
	s.recordOperation(userID, model.OperationLogActionDownload, model.OperationLogTargetTypeFolder, folderModel.ID, folderModel.Name, folderModel.ParentID, map[string]any{"packaged": true})
	return nil
}

func generateRandomHex(byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func generatePublicShareToken() (string, error) {
	return generateRandomHex(24)
}

func (s *fileService) DownloadByPickUpCode(code string, writer io.Writer, setMeta func(fileName, contentType string)) error {
	target, err := s.ResolveActivePickUpCode(code)
	if err != nil {
		return err
	}
	if err := s.MarkPickUpDownloadSuccess(target.CodeID); err != nil {
		return err
	}

	contentType := "application/octet-stream"
	switch target.Type {
	case model.PickUpTargetTypeFile:
		contentType = mime.TypeByExtension(filepath.Ext(target.DownloadName))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if setMeta != nil {
			setMeta(target.DownloadName, contentType)
		}
		if err := s.StreamSingleFile(target.FilePath, writer); err != nil {
			return err
		}
	case model.PickUpTargetTypeFolder:
		contentType = "application/zip"
		if setMeta != nil {
			setMeta(target.DownloadName, contentType)
		}
		if err := s.StreamFolderAsZip(target.UserID, target.FolderID, writer); err != nil {
			return err
		}
	default:
		return errors.New("invalid pickup target type")
	}
	return nil
}

func (s *fileService) ResolveActivePickUpCode(code string) (*PickUpDownloadTarget, error) {
	pickupCode, err := s.FileRepository.GetPickUpCodeByCode(code)
	if err != nil {
		return nil, err
	}
	if pickupCode.Status != model.PickUpCodeStatusActive {
		return nil, ErrPickupCodeExpired
	}
	now := time.Now()
	if now.After(pickupCode.ExpireTime) || pickupCode.Download >= pickupCode.MaxDownload {
		return nil, ErrPickupCodeExpired
	}

	switch pickupCode.Type {
	case model.PickUpTargetTypeFile:
		if pickupCode.FileID == nil {
			return nil, ErrPickupTargetNotFound
		}
		fileModel, err := s.FileRepository.GetFileByFileIDAndUserID(*pickupCode.FileID, pickupCode.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrPickupTargetNotFound
			}
			return nil, err
		}
		filePath, err := s.BuildFileAbsolutePath(fileModel)
		if err != nil {
			return nil, err
		}
		return &PickUpDownloadTarget{
			CodeID:       pickupCode.ID,
			UserID:       pickupCode.UserID,
			Type:         pickupCode.Type,
			FilePath:     filePath,
			DownloadName: fileModel.Name,
		}, nil
	case model.PickUpTargetTypeFolder:
		if pickupCode.FolderID == nil {
			return nil, ErrPickupTargetNotFound
		}
		folderModel, err := s.FileRepository.GetFolderByFolderIDAndUserID(*pickupCode.FolderID, pickupCode.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrPickupTargetNotFound
			}
			return nil, err
		}
		return &PickUpDownloadTarget{
			CodeID:       pickupCode.ID,
			UserID:       pickupCode.UserID,
			Type:         pickupCode.Type,
			FolderID:     folderModel.ID,
			DownloadName: folderModel.Name + ".zip",
		}, nil
	default:
		return nil, errors.New("invalid pickup target type")
	}
}

func (s *fileService) BuildFileAbsolutePath(fileModel *model.FileModel) (string, error) {
	filePath, err := s.buildCanonicalStoragePath(fileModel.FileHash)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filePath); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		dirPath, dirErr := s.buildStorageDir(fileModel.FileHash)
		if dirErr != nil {
			return "", dirErr
		}
		legacyMatches, globErr := filepath.Glob(filepath.Join(dirPath, fileModel.FileHash+".*"))
		if globErr != nil {
			return "", globErr
		}
		for _, match := range legacyMatches {
			if info, statErr := os.Stat(match); statErr == nil && !info.IsDir() {
				return match, nil
			}
		}
		return "", err
	}
	return filePath, nil
}

func (s *fileService) StreamSingleFile(filePath string, writer io.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func (s *fileService) StreamFolderAsZip(userID uint, folderID uint, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	rootFolder, err := s.FileRepository.GetFolderByFolderIDAndUserID(folderID, userID)
	if err != nil {
		return err
	}
	rootFolderName, err := utils.SanitizeFileName(rootFolder.Name)
	if err != nil {
		return err
	}
	if err := utils.ValidateZipEntryPath(rootFolderName); err != nil {
		return err
	}
	if err := s.writeFolderToZip(zipWriter, userID, folderID, rootFolderName); err != nil {
		_ = zipWriter.Close()
		return err
	}
	return zipWriter.Close()
}

func (s *fileService) writeFolderToZip(zipWriter *zip.Writer, userID uint, folderID uint, zipPrefix string) error {
	folders, files, err := s.FileRepository.GetChildrenByFolderIDAndUserID(folderID, userID)
	if err != nil {
		return err
	}
	if len(folders) == 0 && len(files) == 0 {
		return ErrPickupEmptyFolder
	}

	for _, fileModel := range files {
		filePath, err := s.BuildFileAbsolutePath(&fileModel)
		if err != nil {
			return err
		}
		src, err := os.Open(filePath)
		if err != nil {
			return err
		}
		cleanedName, err := utils.SanitizeFileName(fileModel.Name)
		if err != nil {
			src.Close()
			return err
		}
		entryPath := path.Join(zipPrefix, cleanedName)
		if err := utils.ValidateZipEntryPath(entryPath); err != nil {
			src.Close()
			return err
		}
		info, statErr := src.Stat()
		if statErr != nil {
			src.Close()
			return statErr
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			src.Close()
			return err
		}
		header.Name = entryPath
		header.Method = zip.Deflate
		dst, err := zipWriter.CreateHeader(header)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}

	for _, folder := range folders {
		cleanedFolderName, err := utils.SanitizeFileName(folder.Name)
		if err != nil {
			return err
		}
		nextPrefix := path.Join(zipPrefix, cleanedFolderName)
		if err := validateZipEntryPath(nextPrefix); err != nil {
			return err
		}
		if err := s.writeFolderToZip(zipWriter, userID, folder.ID, nextPrefix); err != nil {
			if errors.Is(err, ErrPickupEmptyFolder) {
				_, _ = zipWriter.Create(nextPrefix + "/")
				continue
			}
			return err
		}
	}
	return nil
}

func (s *fileService) MarkPickUpDownloadSuccess(codeID uint) error {
	err := s.FileRepository.IncrementDownloadAndMaybeExpire(codeID, time.Now())
	if err != nil && strings.Contains(err.Error(), "pickup code expired") {
		return ErrPickupCodeExpired
	}
	return err
}
