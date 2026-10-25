package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

const MaxMarkdownBytes = 20 * 1024 * 1024

var (
	ErrKnowledgeInvalid  = errors.New("invalid knowledge request")
	ErrKnowledgeNotFound = errors.New("knowledge or source not found")
)

type KnowledgeStore interface {
	File(context.Context, uint, uint) (*model.FileModel, error)
	FolderName(context.Context, uint, uint) (string, error)
	ByFile(context.Context, uint, uint) (*model.KnowledgeItem, error)
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
	Create(context.Context, *model.KnowledgeItem) (*model.KnowledgeItem, bool, error)
	List(context.Context, uint, int, int, string, model.KnowledgeStatus) ([]model.KnowledgeItem, int64, error)
	Retry(context.Context, uint, uint) error
	Delete(context.Context, uint, uint) error
	Claim(context.Context, string, time.Time) (*model.KnowledgeImportTask, error)
	Finish(context.Context, *model.KnowledgeImportTask, string, string, string) error
}

type KnowledgeService struct {
	store       KnowledgeStore
	storagePath string
	Vectors     VectorizeStore
}

func (s *KnowledgeService) Chunks(ctx context.Context, user, id uint) ([]model.KnowledgeChunk, error) {
	if _, err := s.Get(ctx, user, id); err != nil {
		return nil, err
	}
	if s.Vectors == nil {
		return []model.KnowledgeChunk{}, nil
	}
	return s.Vectors.Chunks(ctx, user, id)
}

func (s *KnowledgeService) ListFiltered(ctx context.Context, user uint, page, size int, q string, status model.KnowledgeStatus, vector model.VectorState) ([]model.KnowledgeItem, int64, error) {
	switch vector {
	case "", model.VectorPending, model.VectorRunning, model.VectorReady, model.VectorPartial:
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	// Retain the original store contract for import-only consumers and test doubles.
	if vector == "" {
		return s.List(ctx, user, page, size, q, status)
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 || utf8.RuneCountInString(q) > 200 {
		return nil, 0, ErrKnowledgeInvalid
	}
	switch status {
	case "", model.KnowledgePending, model.KnowledgeRunning, model.KnowledgeReady, model.KnowledgeFailed:
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	store, ok := s.store.(interface {
		ListVector(context.Context, uint, int, int, string, model.KnowledgeStatus, model.VectorState) ([]model.KnowledgeItem, int64, error)
	})
	if !ok {
		return nil, 0, ErrKnowledgeInvalid
	}
	return store.ListVector(ctx, user, page, size, strings.TrimSpace(q), status, vector)
}

func NewKnowledgeService(store KnowledgeStore, storagePath string) *KnowledgeService {
	return &KnowledgeService{store: store, storagePath: storagePath}
}

func knowledgeError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrKnowledgeNotFound
	}
	return err
}

func validMarkdown(file *model.FileModel) error {
	ext := strings.ToLower(filepath.Ext(file.Name))
	if ext != ".md" && ext != ".markdown" {
		return fmt.Errorf("%w: 仅支持 .md / .markdown 文件", ErrKnowledgeInvalid)
	}
	if file.Size > MaxMarkdownBytes {
		return fmt.Errorf("%w: Markdown 文件不能超过 20 MB", ErrKnowledgeInvalid)
	}
	return nil
}

func (s *KnowledgeService) Add(ctx context.Context, userID, fileID uint, title string) (*model.KnowledgeItem, bool, error) {
	file, err := s.store.File(ctx, userID, fileID)
	if err != nil {
		return nil, false, knowledgeError(err)
	}
	// Repeated requests keep the original title and task, including failed tasks.
	existing, err := s.store.ByFile(ctx, userID, fileID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if err := validMarkdown(file); err != nil {
		return nil, false, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(strings.TrimSuffix(file.Name, filepath.Ext(file.Name)))
	}
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 200 || strings.ContainsRune(title, 0) {
		return nil, false, fmt.Errorf("%w: 知识标题须为 1–200 字符", ErrKnowledgeInvalid)
	}
	folder, err := s.store.FolderName(ctx, userID, file.FolderID)
	if err != nil {
		return nil, false, knowledgeError(err)
	}
	item, created, err := s.store.Create(ctx, &model.KnowledgeItem{
		UserID: userID, SourceFileID: file.ID, SourceFileName: file.Name, SourceFolderID: file.FolderID,
		SourceFolderName: folder, SourceHash: file.FileHash, SourceType: "markdown", Title: title, Status: model.KnowledgePending,
	})
	return item, created, knowledgeError(err)
}

func (s *KnowledgeService) Get(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, err := s.store.Get(ctx, userID, id)
	return item, knowledgeError(err)
}

func (s *KnowledgeService) Delete(ctx context.Context, userID, id uint) error {
	return knowledgeError(s.store.Delete(ctx, userID, id))
}

func (s *KnowledgeService) ByFile(ctx context.Context, userID, fileID uint) (*model.KnowledgeItem, error) {
	if _, err := s.store.File(ctx, userID, fileID); err != nil {
		return nil, knowledgeError(err)
	}
	item, err := s.store.ByFile(ctx, userID, fileID)
	return item, knowledgeError(err)
}

func (s *KnowledgeService) List(ctx context.Context, userID uint, page, size int, query string, status model.KnowledgeStatus) ([]model.KnowledgeItem, int64, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 100 || utf8.RuneCountInString(query) > 200 {
		return nil, 0, ErrKnowledgeInvalid
	}
	switch status {
	case "", model.KnowledgePending, model.KnowledgeRunning, model.KnowledgeReady, model.KnowledgeFailed:
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	return s.store.List(ctx, userID, page, size, strings.TrimSpace(query), status)
}

func (s *KnowledgeService) Retry(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if item.Status != model.KnowledgeFailed {
		return nil, repository.ErrKnowledgeConflict
	}
	file, err := s.store.File(ctx, userID, item.SourceFileID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if err := validMarkdown(file); err != nil {
		return nil, err
	}
	if file.FileHash != item.SourceHash {
		return nil, repository.ErrKnowledgeConflict
	}
	if err := s.store.Retry(ctx, userID, id); err != nil {
		return nil, knowledgeError(err)
	}
	return s.Get(ctx, userID, id)
}

type markdownFailure struct{ code, message string }

func (e *markdownFailure) Error() string      { return e.message }
func failMarkdown(code, message string) error { return &markdownFailure{code, message} }

func readMarkdown(storagePath, hash string) (string, error) {
	// Hashes form paths and glob prefixes; accept hexadecimal characters only.
	if len(hash) < 4 || len(hash) > 128 {
		return "", failMarkdown("invalid_source", "原文件存储标识无效")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", failMarkdown("invalid_source", "原文件存储标识无效")
	}
	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return "", failMarkdown("source_unavailable", "无法读取原文件，请稍后重试")
	}
	defer root.Close()
	dir := filepath.Join(hash[:2], hash[2:4])
	path := filepath.Join(dir, hash)
	file, err := root.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		// Older uploads used hash.extension. Root.Open also rejects escaping symlinks.
		directory, openErr := root.Open(dir)
		if openErr == nil {
			entries, readErr := directory.ReadDir(-1)
			directory.Close()
			if readErr == nil {
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasPrefix(entry.Name(), hash+".") {
						continue
					}
					file, err = root.Open(filepath.Join(dir, entry.Name()))
					if err == nil {
						break
					}
				}
			}
		}
	}
	if err != nil {
		return "", failMarkdown("source_unavailable", "无法读取原文件，请确认文件仍存在后重试")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", failMarkdown("source_unavailable", "原文件不是可读取的普通文件")
	}
	if info.Size() > MaxMarkdownBytes {
		return "", failMarkdown("file_too_large", "Markdown 文件不能超过 20 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxMarkdownBytes+1))
	if err != nil {
		return "", failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	if len(data) > MaxMarkdownBytes {
		return "", failMarkdown("file_too_large", "Markdown 文件不能超过 20 MB")
	}
	text := strings.TrimPrefix(string(data), "\uFEFF")
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return "", failMarkdown("invalid_encoding", "仅支持 UTF-8 编码的 Markdown 文本")
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if strings.TrimSpace(text) == "" {
		return "", failMarkdown("empty_content", "未读取到有效正文，原文件为空")
	}
	return text, nil
}

func (s *KnowledgeService) ProcessNext(ctx context.Context) (bool, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return false, err
	}
	task, err := s.store.Claim(ctx, hex.EncodeToString(bytes), time.Now())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	item, err := s.store.Get(ctx, task.UserID, task.KnowledgeID)
	if err != nil {
		return true, err
	}
	file, err := s.store.File(ctx, task.UserID, item.SourceFileID)
	var content string
	var code, message string
	if errors.Is(err, gorm.ErrRecordNotFound) {
		code, message = "source_deleted", "原文件已删除，无法继续加入知识库"
	} else if err != nil {
		// Transient database failures leave the lease to expire instead of marking content invalid.
		return true, err
	} else if file.FileHash != item.SourceHash {
		code, message = "source_changed", "原文件内容已变化，无法处理原加入任务"
	} else {
		content, err = readMarkdown(s.storagePath, item.SourceHash)
		if err != nil {
			var failure *markdownFailure
			if !errors.As(err, &failure) {
				return true, err
			}
			code, message = failure.code, failure.message
		}
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	return true, s.store.Finish(ctx, task, content, code, message)
}

func (s *KnowledgeService) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := s.ProcessNext(ctx)
		if err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("知识导入任务处理失败")
		}
		if processed && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
