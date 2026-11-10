package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

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
	CreateManual(context.Context, *model.KnowledgeItem) error
	Update(context.Context, uint, uint, string, string, string, uint) (*model.KnowledgeItem, error)
	SetFavorite(context.Context, uint, uint, bool) error
	SetArchived(context.Context, uint, uint, bool) (*time.Time, error)
	ListVersions(context.Context, uint, uint) ([]model.KnowledgeVersion, error)
	GetVersion(context.Context, uint, uint, uint) (*model.KnowledgeVersion, error)
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
	// Fetcher 网页导入的抓取器；为 nil 时网页导入不可用，不影响其余功能。
	Fetcher WebFetcher
	// Logs 为可选依赖：为 nil 时跳过操作日志记录，不影响主流程。
	Logs OperationLogWriter
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

// KnowledgeListOptions 汇总知识列表的附加筛选与排序参数。
type KnowledgeListOptions struct {
	SourceType string
	TagID      uint
	Favorite   *bool
	Archived   bool
	Sort       string
}

// knowledgeSourceTypes 是列表中允许筛选的来源类型；新增提取格式时需同步登记。
var knowledgeSourceTypes = map[string]struct{}{
	"manual":   {},
	"web":      {},
	"markdown": {},
	"text":     {},
	"pdf":      {},
	"docx":     {},
	"csv":      {},
	"xlsx":     {},
}

func (s *KnowledgeService) ListFiltered(ctx context.Context, user uint, page, size int, q string, status model.KnowledgeStatus, vector model.VectorState, opts KnowledgeListOptions) ([]model.KnowledgeItem, int64, error) {
	switch vector {
	case "", model.VectorPending, model.VectorRunning, model.VectorReady, model.VectorPartial:
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 || utf8.RuneCountInString(q) > 200 {
		return nil, 0, ErrKnowledgeInvalid
	}
	switch status {
	case "", model.KnowledgePending, model.KnowledgeRunning, model.KnowledgeReady, model.KnowledgeFailed:
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	if opts.SourceType != "" {
		if _, ok := knowledgeSourceTypes[opts.SourceType]; !ok {
			return nil, 0, ErrKnowledgeInvalid
		}
	}
	switch opts.Sort {
	case "", "created", "updated", "title":
	default:
		return nil, 0, ErrKnowledgeInvalid
	}
	if store, ok := s.store.(interface {
		ListVector(context.Context, uint, repository.KnowledgeListFilter) ([]model.KnowledgeItem, int64, error)
	}); ok {
		return store.ListVector(ctx, user, repository.KnowledgeListFilter{
			Page:        page,
			Size:        size,
			Query:       strings.TrimSpace(q),
			Status:      status,
			VectorState: vector,
			SourceType:  opts.SourceType,
			TagID:       opts.TagID,
			Favorite:    opts.Favorite,
			Archived:    opts.Archived,
			Sort:        opts.Sort,
		})
	}
	// Retain the original store contract for import-only consumers and test doubles.
	if opts == (KnowledgeListOptions{}) && vector == "" {
		return s.List(ctx, user, page, size, q, status)
	}
	return nil, 0, ErrKnowledgeInvalid
}

func NewKnowledgeService(store KnowledgeStore, storagePath string) *KnowledgeService {
	return &KnowledgeService{store: store, storagePath: storagePath}
}

// recordOperation 记录知识域操作日志；Logs 为 nil 时跳过，绝不影响主流程。
func (s *KnowledgeService) recordOperation(userID uint, action model.OperationLogAction, targetID uint, targetName, folderName string, detail map[string]any) {
	if s.Logs == nil {
		return
	}
	writeOperationLog(s.Logs, userID, action, model.OperationLogTargetTypeKnowledge, targetID, targetName, folderName, detail)
}

func knowledgeError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrKnowledgeNotFound
	}
	return err
}

// validateKnowledgeTitle 校验知识标题：1–200 字符、合法 UTF-8、不含 NUL。
func validateKnowledgeTitle(title string) error {
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 200 || strings.ContainsRune(title, 0) {
		return fmt.Errorf("%w: 知识标题须为 1–200 字符", ErrKnowledgeInvalid)
	}
	return nil
}

// ManualContentMaxRunes 手动创建与编辑正文上限（字符数）。
const ManualContentMaxRunes = 2_000_000

// validateKnowledgeContent 校验知识正文：1–2M 字符、合法 UTF-8、不含 NUL。
func validateKnowledgeContent(content string) error {
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return fmt.Errorf("%w: 知识正文须为合法 UTF-8 文本", ErrKnowledgeInvalid)
	}
	if n := utf8.RuneCountInString(content); n == 0 || n > ManualContentMaxRunes {
		return fmt.Errorf("%w: 知识正文须为 1–%d 字符", ErrKnowledgeInvalid, ManualContentMaxRunes)
	}
	return nil
}

// buildInlineKnowledge 组装正文已定稿的知识条目：手动创建与网页导入共用。
func buildInlineKnowledge(userID uint, title, content, sourceType, sourceURL string) (*model.KnowledgeItem, error) {
	title = strings.TrimSpace(title)
	if err := validateKnowledgeTitle(title); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if err := validateKnowledgeContent(content); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(content))
	now := time.Now()
	return &model.KnowledgeItem{
		UserID: userID, SourceHash: hex.EncodeToString(sum[:]), SourceType: sourceType, SourceURL: sourceURL,
		Title: title, Content: content, Status: model.KnowledgeReady, Version: 1, ImportedAt: &now,
	}, nil
}

// CreateManual 手动创建知识：正文即用户输入，直接就绪并排入向量化队列。
func (s *KnowledgeService) CreateManual(ctx context.Context, userID uint, title, content string) (*model.KnowledgeItem, error) {
	item, err := buildInlineKnowledge(userID, title, content, model.KnowledgeSourceTypeManual, "")
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateManual(ctx, item); err != nil {
		return nil, knowledgeError(err)
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeCreate, item.ID, item.Title, "", nil)
	return item, nil
}

// ImportWeb 从网页导入知识：抓取正文 → 落库（source_type=web）→ 复用分块向量化管线。
func (s *KnowledgeService) ImportWeb(ctx context.Context, userID uint, rawURL string) (*model.KnowledgeItem, error) {
	if s.Fetcher == nil {
		return nil, fmt.Errorf("%w: 网页导入未配置", ErrWebFetchFailed)
	}
	document, err := s.Fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(document.Title)
	if title == "" {
		title = webTitleFallback(document.URL)
	}
	item, err := buildInlineKnowledge(
		userID,
		truncateRunes(title, 200),
		truncateRunes(document.Content, ManualContentMaxRunes),
		model.KnowledgeSourceTypeWeb,
		document.URL,
	)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateManual(ctx, item); err != nil {
		return nil, knowledgeError(err)
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeWebImport, item.ID, item.Title, "", nil)
	return item, nil
}

// truncateRunes 按字符数截断，避免网页标题或正文超限导致导入失败。
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

// webTitleFallback 网页没有标题时用域名兜底。
func webTitleFallback(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "网页导入"
	}
	return parsed.Host
}

// Update 编辑知识正文：乐观锁校验 base_version，服务层只做校验与日志，快照与重建由存储层单事务完成。
func (s *KnowledgeService) Update(ctx context.Context, userID, id uint, title, content string, baseVersion uint) (*model.KnowledgeItem, error) {
	if baseVersion == 0 {
		return nil, fmt.Errorf("%w: 缺少 base_version", ErrKnowledgeInvalid)
	}
	title = strings.TrimSpace(title)
	if err := validateKnowledgeTitle(title); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if err := validateKnowledgeContent(content); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(content))
	item, err := s.store.Update(ctx, userID, id, title, content, hex.EncodeToString(sum[:]), baseVersion)
	if err != nil {
		return nil, knowledgeError(err)
	}
	// 标题与正文均未变化时版本号不变，不记编辑日志。
	if item.Version > baseVersion {
		s.recordOperation(userID, model.OperationLogActionKnowledgeUpdate, item.ID, item.Title, item.SourceFolderName, map[string]any{"version": item.Version})
	}
	return item, nil
}

// SetFavorite 收藏或取消收藏；幂等操作，仅状态变化时记操作日志。
func (s *KnowledgeService) SetFavorite(ctx context.Context, userID, id uint, favorite bool) error {
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if item.IsFavorite == favorite {
		return nil
	}
	if err := s.store.SetFavorite(ctx, userID, id, favorite); err != nil {
		return knowledgeError(err)
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeFavorite, item.ID, item.Title, item.SourceFolderName, map[string]any{"favorite": favorite})
	return nil
}

// Archive 归档知识：归档后不出现在默认列表与问答检索中；幂等。
func (s *KnowledgeService) Archive(ctx context.Context, userID, id uint) (*time.Time, error) {
	return s.setArchived(ctx, userID, id, true)
}

// Unarchive 恢复归档知识，恢复后重新参与列表与检索。
func (s *KnowledgeService) Unarchive(ctx context.Context, userID, id uint) error {
	_, err := s.setArchived(ctx, userID, id, false)
	return err
}

func (s *KnowledgeService) setArchived(ctx context.Context, userID, id uint, archived bool) (*time.Time, error) {
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if archived == (item.ArchivedAt != nil) {
		return item.ArchivedAt, nil
	}
	at, err := s.store.SetArchived(ctx, userID, id, archived)
	if err != nil {
		return nil, knowledgeError(err)
	}
	action := model.OperationLogActionKnowledgeArchive
	if !archived {
		action = model.OperationLogActionKnowledgeUnarchive
	}
	s.recordOperation(userID, action, item.ID, item.Title, item.SourceFolderName, nil)
	return at, nil
}

// KnowledgeVersionSummary 版本列表项：当前版本由 knowledge_items 实时合成，便于前端标出「当前」。
type KnowledgeVersionSummary struct {
	VersionNumber uint      `json:"version_number"`
	Title         string    `json:"title"`
	ChangeSource  string    `json:"change_source"`
	CreatedAt     time.Time `json:"created_at"`
	Current       bool      `json:"current"`
}

// KnowledgeVersionDetail 单个版本的完整内容。
type KnowledgeVersionDetail struct {
	KnowledgeVersionSummary
	Content string `json:"content"`
}

// Versions 返回版本历史：当前版本置顶，其余为不可变快照；列表不含正文。
func (s *KnowledgeService) Versions(ctx context.Context, userID, id uint) ([]KnowledgeVersionSummary, error) {
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	versions, err := s.store.ListVersions(ctx, userID, id)
	if err != nil {
		return nil, knowledgeError(err)
	}
	summaries := make([]KnowledgeVersionSummary, 0, len(versions)+1)
	summaries = append(summaries, KnowledgeVersionSummary{
		VersionNumber: item.Version, Title: item.Title,
		ChangeSource: string(model.KnowledgeChangeSourceManual), CreatedAt: item.UpdatedAt, Current: true,
	})
	for _, version := range versions {
		summaries = append(summaries, KnowledgeVersionSummary{
			VersionNumber: version.VersionNumber, Title: version.Title,
			ChangeSource: string(version.ChangeSource), CreatedAt: version.CreatedAt,
		})
	}
	return summaries, nil
}

// VersionDetail 返回指定版本正文；版本号等于当前版本时返回条目实时内容。
func (s *KnowledgeService) VersionDetail(ctx context.Context, userID, id, versionNumber uint) (*KnowledgeVersionDetail, error) {
	if versionNumber == 0 {
		return nil, fmt.Errorf("%w: 版本号无效", ErrKnowledgeInvalid)
	}
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if versionNumber > item.Version {
		return nil, ErrKnowledgeNotFound
	}
	if versionNumber == item.Version {
		return &KnowledgeVersionDetail{
			KnowledgeVersionSummary: KnowledgeVersionSummary{
				VersionNumber: item.Version, Title: item.Title,
				ChangeSource: string(model.KnowledgeChangeSourceManual), CreatedAt: item.UpdatedAt, Current: true,
			},
			Content: item.Content,
		}, nil
	}
	version, err := s.store.GetVersion(ctx, userID, id, versionNumber)
	if err != nil {
		return nil, knowledgeError(err)
	}
	return &KnowledgeVersionDetail{
		KnowledgeVersionSummary: KnowledgeVersionSummary{
			VersionNumber: version.VersionNumber, Title: version.Title,
			ChangeSource: string(version.ChangeSource), CreatedAt: version.CreatedAt,
		},
		Content: version.Content,
	}, nil
}

// Restore 恢复历史版本：以目标版本内容生成新版本（当前正文先写入快照），历史记录不被篡改。
func (s *KnowledgeService) Restore(ctx context.Context, userID, id, versionNumber uint) (*model.KnowledgeItem, error) {
	if versionNumber == 0 {
		return nil, fmt.Errorf("%w: 版本号无效", ErrKnowledgeInvalid)
	}
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if item.Status != model.KnowledgeReady {
		return nil, repository.ErrKnowledgeConflict
	}
	if versionNumber > item.Version {
		return nil, ErrKnowledgeNotFound
	}
	// 恢复当前版本无需任何变更。
	if versionNumber == item.Version {
		return item, nil
	}
	target, err := s.store.GetVersion(ctx, userID, id, versionNumber)
	if err != nil {
		return nil, knowledgeError(err)
	}
	sum := sha256.Sum256([]byte(target.Content))
	baseVersion := item.Version
	updated, err := s.store.Update(ctx, userID, id, target.Title, target.Content, hex.EncodeToString(sum[:]), baseVersion)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if updated.Version > baseVersion {
		s.recordOperation(userID, model.OperationLogActionKnowledgeRestore, updated.ID, updated.Title, updated.SourceFolderName,
			map[string]any{"version": updated.Version, "restored_from": versionNumber})
	}
	return updated, nil
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
	extractor, err := validateSource(file)
	if err != nil {
		return nil, false, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(strings.TrimSuffix(file.Name, filepath.Ext(file.Name)))
	}
	if err := validateKnowledgeTitle(title); err != nil {
		return nil, false, err
	}
	folder, err := s.store.FolderName(ctx, userID, file.FolderID)
	if err != nil {
		return nil, false, knowledgeError(err)
	}
	item, created, err := s.store.Create(ctx, &model.KnowledgeItem{
		UserID: userID, SourceFileID: &file.ID, SourceFileName: file.Name, SourceFolderID: file.FolderID,
		SourceFolderName: folder, SourceHash: file.FileHash, SourceType: extractor.SourceType(), Title: title, Status: model.KnowledgePending, Version: 1,
	})
	if err != nil {
		return nil, false, knowledgeError(err)
	}
	// 幂等重复请求不重复记日志。
	if created {
		s.recordOperation(userID, model.OperationLogActionKnowledgeAdd, item.ID, item.Title, item.SourceFolderName, map[string]any{"source_file": item.SourceFileName})
	}
	return item, created, nil
}

func (s *KnowledgeService) Get(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, err := s.store.Get(ctx, userID, id)
	return item, knowledgeError(err)
}

func (s *KnowledgeService) Delete(ctx context.Context, userID, id uint) error {
	// 先取快照：日志需要标题与来源目录；顺带让不存在的 id 返回 404（此前静默成功）。
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := knowledgeError(s.store.Delete(ctx, userID, id)); err != nil {
		return err
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeDelete, id, item.Title, item.SourceFolderName, nil)
	return nil
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
	// 手动创建的知识没有来源文件，不存在可重试的导入任务。
	if item.SourceFileID == nil {
		return nil, repository.ErrKnowledgeConflict
	}
	file, err := s.store.File(ctx, userID, *item.SourceFileID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if _, err := validateSource(file); err != nil {
		return nil, err
	}
	if file.FileHash != item.SourceHash {
		return nil, repository.ErrKnowledgeConflict
	}
	if err := s.store.Retry(ctx, userID, id); err != nil {
		return nil, knowledgeError(err)
	}
	updated, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeRetry, id, updated.Title, updated.SourceFolderName, nil)
	return updated, nil
}

// RetryVectorize 重排队部分失败的向量化任务：失败分块回到 pending，已成功分块保留，
// 不重复调用已确认的付费 Embedding。仅 vector_state=partial 可重试。
func (s *KnowledgeService) RetryVectorize(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if item.Status != model.KnowledgeReady || item.VectorState != model.VectorPartial {
		return nil, repository.ErrKnowledgeConflict
	}
	if s.Vectors == nil {
		return nil, repository.ErrKnowledgeConflict
	}
	if err := s.Vectors.Requeue(ctx, userID, id); err != nil {
		return nil, knowledgeError(err)
	}
	updated, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	s.recordOperation(userID, model.OperationLogActionKnowledgeVectorRetry, id, updated.Title, updated.SourceFolderName, nil)
	return updated, nil
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
	// 导入任务只可能属于文件来源；空指针防御性按来源缺失处理。
	var file *model.FileModel
	if item.SourceFileID == nil {
		err = gorm.ErrRecordNotFound
	} else {
		file, err = s.store.File(ctx, task.UserID, *item.SourceFileID)
	}
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
		content, err = extractSource(s.storagePath, item.SourceHash, item.SourceType)
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
