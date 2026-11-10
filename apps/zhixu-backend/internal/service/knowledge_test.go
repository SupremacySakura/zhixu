package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func uintPtr(v uint) *uint { return &v }

type knowledgeFake struct {
	KnowledgeStore
	file                   *model.FileModel
	item                   *model.KnowledgeItem
	task                   *model.KnowledgeImportTask
	fileErr                error
	created, finished      bool
	manualCreated          bool
	code, message, content string
}

func (f *knowledgeFake) File(_ context.Context, user, id uint) (*model.FileModel, error) {
	if f.fileErr != nil {
		return nil, f.fileErr
	}
	if f.file == nil || f.file.UserID != user || f.file.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.file, nil
}
func (f *knowledgeFake) FolderName(context.Context, uint, uint) (string, error) {
	return "学习资料", nil
}
func (f *knowledgeFake) ByFile(_ context.Context, user, file uint) (*model.KnowledgeItem, error) {
	if f.item == nil || f.item.UserID != user || f.item.SourceFileID == nil || *f.item.SourceFileID != file {
		return nil, gorm.ErrRecordNotFound
	}
	return f.item, nil
}
func (f *knowledgeFake) Get(_ context.Context, user, id uint) (*model.KnowledgeItem, error) {
	if f.item == nil || f.item.UserID != user || f.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.item, nil
}
func (f *knowledgeFake) Create(_ context.Context, item *model.KnowledgeItem) (*model.KnowledgeItem, bool, error) {
	f.created = true
	item.ID = 10
	f.item = item
	return item, true, nil
}
func (f *knowledgeFake) CreateManual(_ context.Context, item *model.KnowledgeItem) error {
	f.manualCreated = true
	item.ID = 11
	f.item = item
	return nil
}
func (f *knowledgeFake) Update(_ context.Context, user, id uint, title, content, sourceHash string, baseVersion uint) (*model.KnowledgeItem, error) {
	if f.item == nil || f.item.UserID != user || f.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	if f.item.Status != model.KnowledgeReady || f.item.Version != baseVersion {
		return nil, repository.ErrKnowledgeConflict
	}
	if f.item.Title == title && f.item.Content == content {
		return f.item, nil
	}
	f.item.Title, f.item.Content, f.item.SourceHash = title, content, sourceHash
	f.item.Version = baseVersion + 1
	return f.item, nil
}
func (f *knowledgeFake) Claim(context.Context, string, time.Time) (*model.KnowledgeImportTask, error) {
	if f.task == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.task, nil
}
func (f *knowledgeFake) Finish(_ context.Context, _ *model.KnowledgeImportTask, content, code, message string) error {
	f.finished, f.content, f.code, f.message = true, content, code, message
	return nil
}
func (f *knowledgeFake) Retry(context.Context, uint, uint) error {
	f.item.Status = model.KnowledgePending
	return nil
}
func (f *knowledgeFake) Delete(_ context.Context, user, id uint) error {
	if f.item == nil || f.item.UserID != user || f.item.ID != id {
		return nil
	}
	f.item = nil
	return nil
}

// operationLogSpy 捕获写入的操作日志，供知识域日志断言。
type operationLogSpy struct {
	entries []model.OperationLogModel
	pruned  int
}

func (s *operationLogSpy) CreateOperationLog(entry *model.OperationLogModel) error {
	s.entries = append(s.entries, *entry)
	return nil
}
func (s *operationLogSpy) PruneOperationLogs(uint, int) (int64, error) {
	s.pruned++
	return 0, nil
}

func TestKnowledgeOperationLogs(t *testing.T) {
	newFixture := func() (*knowledgeFake, *operationLogSpy, *KnowledgeService) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "note.md", FileHash: "aabbccdd", Size: 1}}
		spy := &operationLogSpy{}
		s := NewKnowledgeService(f, "")
		s.Logs = spy
		return f, spy, s
	}

	t.Run("add logs once", func(t *testing.T) {
		_, spy, s := newFixture()
		item, created, err := s.Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		require.Len(t, spy.entries, 1)
		entry := spy.entries[0]
		require.Equal(t, model.OperationLogActionKnowledgeAdd, entry.Action)
		require.Equal(t, model.OperationLogTargetTypeKnowledge, entry.TargetType)
		require.Equal(t, item.ID, entry.TargetID)
		require.Equal(t, item.Title, entry.TargetName)
		require.Equal(t, "学习资料", entry.FolderName)
		require.JSONEq(t, `{"source_file":"note.md"}`, entry.Detail)
		require.Equal(t, 1, spy.pruned)

		// 幂等重复请求不重复记日志。
		_, created, err = s.Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.False(t, created)
		require.Len(t, spy.entries, 1)
	})

	t.Run("delete logs snapshot", func(t *testing.T) {
		f, spy, s := newFixture()
		f.item = &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceFolderName: "学习资料", Title: "旧知识", Status: model.KnowledgeReady}
		require.NoError(t, s.Delete(context.Background(), 2, 10))
		require.Len(t, spy.entries, 1)
		entry := spy.entries[0]
		require.Equal(t, model.OperationLogActionKnowledgeDelete, entry.Action)
		require.Equal(t, uint(10), entry.TargetID)
		require.Equal(t, "旧知识", entry.TargetName)
		require.Equal(t, "学习资料", entry.FolderName)

		// 不存在的 id 现在返回 404，且不再产生日志。
		require.ErrorIs(t, s.Delete(context.Background(), 2, 10), ErrKnowledgeNotFound)
		require.Len(t, spy.entries, 1)
	})

	t.Run("retry logs after success", func(t *testing.T) {
		f, spy, s := newFixture()
		f.item = &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: "aabbccdd", SourceFolderName: "学习资料", Title: "失败知识", Status: model.KnowledgeFailed}
		_, err := s.Retry(context.Background(), 2, 10)
		require.NoError(t, err)
		require.Len(t, spy.entries, 1)
		entry := spy.entries[0]
		require.Equal(t, model.OperationLogActionKnowledgeRetry, entry.Action)
		require.Equal(t, "失败知识", entry.TargetName)

		// 冲突（非失败状态）不记日志。
		_, err = s.Retry(context.Background(), 2, 10)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
		require.Len(t, spy.entries, 1)
	})

	t.Run("nil logs stays silent", func(t *testing.T) {
		f, _, _ := newFixture()
		s := NewKnowledgeService(f, "")
		_, created, err := s.Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		f.item.Status = model.KnowledgeFailed
		_, err = s.Retry(context.Background(), 2, 10)
		require.NoError(t, err)
		require.NoError(t, s.Delete(context.Background(), 2, 10))
	})
}

func writeKnowledgeSource(t *testing.T, root, hash, suffix, content string) {
	t.Helper()
	dir := filepath.Join(root, hash[:2], hash[2:4])
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, hash+suffix), []byte(content), 0600))
}

func TestKnowledgeAdd(t *testing.T) {
	for _, tc := range []struct {
		name, filename, title string
		size                  uint64
		invalid               bool
	}{
		{"default title", "笔记.MD", "", 1, false},
		{"custom title", "note.markdown", "  标题  ", 1, false},
		{"unsupported", "note.exe", "", 1, true},
		{"too large", "note.md", "", MaxMarkdownBytes + 1, true},
		{"long title", "note.md", strings.Repeat("知", 201), 1, true},
		{"empty filename title", ".md", "", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: tc.filename, FileHash: "aabbccdd", Size: tc.size}}
			item, created, err := NewKnowledgeService(f, "").Add(context.Background(), 2, 1, tc.title)
			if tc.invalid {
				require.ErrorIs(t, err, ErrKnowledgeInvalid)
				require.False(t, f.created)
				return
			}
			require.NoError(t, err)
			require.True(t, created)
			require.Equal(t, model.KnowledgePending, item.Status)
			require.Equal(t, "markdown", item.SourceType)
			require.Equal(t, "学习资料", item.SourceFolderName)
			require.NotEmpty(t, item.Title)
		})
	}
}

func TestKnowledgeAddIsolationAndIdempotency(t *testing.T) {
	f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "note.md"}, item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), Title: "原标题", Status: model.KnowledgeFailed}}
	s := NewKnowledgeService(f, "")
	_, _, err := s.Add(context.Background(), 3, 1, "")
	require.ErrorIs(t, err, ErrKnowledgeNotFound)
	item, created, err := s.Add(context.Background(), 2, 1, "新标题")
	require.NoError(t, err)
	require.False(t, created)
	require.False(t, f.created)
	require.Equal(t, "原标题", item.Title)
	require.Equal(t, model.KnowledgeFailed, item.Status)
}

func TestReadMarkdown(t *testing.T) {
	for _, tc := range []struct{ name, content, suffix, expected, code string }{
		{"normal", "# 笔记\n正文\n", "", "# 笔记\n正文\n", ""},
		{"BOM and CRLF", "\uFEFF# 标题\r\n内容\r", "", "# 标题\n内容\n", ""},
		{"legacy", "正文", ".md", "正文", ""},
		{"empty", " \r\n\t", "", "", "empty_content"},
		{"invalid UTF8", "\xff", "", "", "invalid_encoding"},
		{"binary", "abc\x00", "", "", "invalid_encoding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeKnowledgeSource(t, root, "aabbccdd", tc.suffix, tc.content)
			actual, err := readMarkdown(root, "aabbccdd")
			if tc.code != "" {
				var failure *markdownFailure
				require.ErrorAs(t, err, &failure)
				require.Equal(t, tc.code, failure.code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestReadMarkdownSecurityAndLimits(t *testing.T) {
	root := t.TempDir()
	for _, hash := range []string{"", "../..", "aabb/../cc", "aa*bb", "aab"} {
		_, err := readMarkdown(root, hash)
		require.Error(t, err)
	}
	_, err := readMarkdown(root, "aabbccdd")
	require.Error(t, err)
	writeKnowledgeSource(t, root, "aabbccdd", "", strings.Repeat("a", MaxMarkdownBytes+1))
	_, err = readMarkdown(root, "aabbccdd")
	var failure *markdownFailure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, "file_too_large", failure.code)
	external := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(external, []byte("outside"), 0600))
	require.NoError(t, os.Remove(filepath.Join(root, "aa", "bb", "aabbccdd")))
	require.NoError(t, os.Symlink(external, filepath.Join(root, "aa", "bb", "aabbccdd")))
	_, err = readMarkdown(root, "aabbccdd")
	require.Error(t, err)
}

func TestKnowledgeProcess(t *testing.T) {
	for _, tc := range []struct {
		name, content, hash, code string
		deleted                   bool
	}{
		{"success", "# 文本\n内容", "aabbccdd", "", false},
		{"empty", "", "aabbccdd", "empty_content", false},
		{"changed", "内容", "aabb0000", "source_changed", false},
		{"deleted", "内容", "aabbccdd", "source_deleted", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeKnowledgeSource(t, root, "aabbccdd", "", tc.content)
			f := &knowledgeFake{
				file: &model.FileModel{ID: 1, UserID: 2, FileHash: tc.hash},
				item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: "aabbccdd"},
				task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
			}
			if tc.deleted {
				f.file = nil
			}
			processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
			require.NoError(t, err)
			require.True(t, processed)
			require.True(t, f.finished)
			require.Equal(t, tc.code, f.code)
			if tc.code == "" {
				require.Equal(t, tc.content, f.content)
			}
		})
	}
}

func TestKnowledgeRetryAndValidation(t *testing.T) {
	f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "note.md", FileHash: "aabbccdd"}, item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: "aabbccdd", Status: model.KnowledgeReady}}
	s := NewKnowledgeService(f, "")
	_, err := s.Retry(context.Background(), 2, 10)
	require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
	f.item.Status = model.KnowledgeFailed
	_, err = s.Retry(context.Background(), 2, 10)
	require.NoError(t, err)
	require.Equal(t, model.KnowledgePending, f.item.Status)
	for _, status := range []model.KnowledgeStatus{"invalid", "new", "unsupported"} {
		_, _, err := s.List(context.Background(), 2, 1, 20, "", status)
		require.ErrorIs(t, err, ErrKnowledgeInvalid)
	}
	_, _, err = s.List(context.Background(), 2, 0, 20, "", "")
	require.ErrorIs(t, err, ErrKnowledgeInvalid)
	processed, err := s.ProcessNext(context.Background())
	require.NoError(t, err)
	require.False(t, processed)
}

func TestKnowledgeRetryVectorize(t *testing.T) {
	newFixture := func() (*knowledgeFake, *vectorStoreFake, *operationLogSpy, *KnowledgeService) {
		f := &knowledgeFake{item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFolderName: "学习资料", Title: "笔记", Status: model.KnowledgeReady, VectorState: model.VectorPartial}}
		vectors := &vectorStoreFake{}
		spy := &operationLogSpy{}
		s := NewKnowledgeService(f, "")
		s.Vectors = vectors
		s.Logs = spy
		return f, vectors, spy, s
	}

	t.Run("requeues partial and logs", func(t *testing.T) {
		_, vectors, spy, s := newFixture()
		updated, err := s.RetryVectorize(context.Background(), 2, 10)
		require.NoError(t, err)
		require.Equal(t, uint(10), updated.ID)
		require.True(t, vectors.requeueCalled)
		require.Equal(t, uint(2), vectors.requeueUser)
		require.Equal(t, uint(10), vectors.requeueKnowledge)
		require.Len(t, spy.entries, 1)
		require.Equal(t, model.OperationLogActionKnowledgeVectorRetry, spy.entries[0].Action)
		require.Equal(t, model.OperationLogTargetTypeKnowledge, spy.entries[0].TargetType)
	})

	t.Run("rejects non partial vector states", func(t *testing.T) {
		for _, state := range []model.VectorState{"", model.VectorPending, model.VectorRunning, model.VectorReady} {
			f, vectors, _, s := newFixture()
			f.item.VectorState = state
			_, err := s.RetryVectorize(context.Background(), 2, 10)
			require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
			require.False(t, vectors.requeueCalled)
		}
	})

	t.Run("rejects when content not ready", func(t *testing.T) {
		f, vectors, _, s := newFixture()
		f.item.Status = model.KnowledgeFailed
		_, err := s.RetryVectorize(context.Background(), 2, 10)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
		require.False(t, vectors.requeueCalled)
	})

	t.Run("not found or other user", func(t *testing.T) {
		_, _, _, s := newFixture()
		_, err := s.RetryVectorize(context.Background(), 2, 99)
		require.ErrorIs(t, err, ErrKnowledgeNotFound)
		_, err = s.RetryVectorize(context.Background(), 3, 10)
		require.ErrorIs(t, err, ErrKnowledgeNotFound)
	})

	t.Run("requires vector store", func(t *testing.T) {
		f := &knowledgeFake{item: &model.KnowledgeItem{ID: 10, UserID: 2, Status: model.KnowledgeReady, VectorState: model.VectorPartial}}
		s := NewKnowledgeService(f, "")
		_, err := s.RetryVectorize(context.Background(), 2, 10)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
	})

	t.Run("propagates store conflict", func(t *testing.T) {
		_, vectors, spy, s := newFixture()
		vectors.requeueErr = repository.ErrKnowledgeConflict
		_, err := s.RetryVectorize(context.Background(), 2, 10)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
		require.Empty(t, spy.entries)
	})
}

func TestKnowledgeCreateManual(t *testing.T) {
	t.Run("ready hashed and logged", func(t *testing.T) {
		f := &knowledgeFake{}
		spy := &operationLogSpy{}
		s := NewKnowledgeService(f, "")
		s.Logs = spy
		item, err := s.CreateManual(context.Background(), 2, "  手写笔记  ", "  # 正文\n内容  ")
		require.NoError(t, err)
		require.True(t, f.manualCreated)
		require.Equal(t, uint(11), item.ID)
		require.Equal(t, "手写笔记", item.Title)
		require.Equal(t, "# 正文\n内容", item.Content)
		require.Equal(t, model.KnowledgeReady, item.Status)
		require.Equal(t, model.KnowledgeSourceTypeManual, item.SourceType)
		require.Nil(t, item.SourceFileID)
		sum := sha256.Sum256([]byte("# 正文\n内容"))
		require.Equal(t, hex.EncodeToString(sum[:]), item.SourceHash)
		require.NotNil(t, item.ImportedAt)
		require.Len(t, spy.entries, 1)
		require.Equal(t, model.OperationLogActionKnowledgeCreate, spy.entries[0].Action)
		require.Equal(t, model.OperationLogTargetTypeKnowledge, spy.entries[0].TargetType)
		require.Equal(t, uint(11), spy.entries[0].TargetID)
		require.Equal(t, "手写笔记", spy.entries[0].TargetName)
		require.Equal(t, 1, spy.pruned)
	})

	t.Run("validation rejects invalid input before persisting", func(t *testing.T) {
		for _, tc := range []struct{ name, title, content string }{
			{"empty title", "", "正文"},
			{"blank title", "   ", "正文"},
			{"long title", strings.Repeat("知", 201), "正文"},
			{"NUL title", "标\x00题", "正文"},
			{"empty content", "标题", ""},
			{"blank content", "标题", "  \n\t "},
			{"NUL content", "标题", "abc\x00def"},
			{"invalid UTF-8 content", "标题", "\xff\xfe"},
			{"oversized content", "标题", strings.Repeat("文", ManualContentMaxRunes+1)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := &knowledgeFake{}
				_, err := NewKnowledgeService(f, "").CreateManual(context.Background(), 2, tc.title, tc.content)
				require.ErrorIs(t, err, ErrKnowledgeInvalid)
				require.False(t, f.manualCreated)
			})
		}
	})

	t.Run("manual item has no import retry", func(t *testing.T) {
		f := &knowledgeFake{}
		s := NewKnowledgeService(f, "")
		item, err := s.CreateManual(context.Background(), 2, "手写笔记", "正文")
		require.NoError(t, err)
		f.item.Status = model.KnowledgeFailed
		_, err = s.Retry(context.Background(), 2, item.ID)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
	})
}

func TestKnowledgeUpdate(t *testing.T) {
	newFixture := func() (*knowledgeFake, *operationLogSpy, *KnowledgeService) {
		f := &knowledgeFake{item: &model.KnowledgeItem{
			ID: 7, UserID: 2, Title: "旧标题", Content: "旧正文", Status: model.KnowledgeReady,
			Version: 3, SourceFolderName: "学习资料",
		}}
		spy := &operationLogSpy{}
		s := NewKnowledgeService(f, "")
		s.Logs = spy
		return f, spy, s
	}

	t.Run("bumps version and logs", func(t *testing.T) {
		f, spy, s := newFixture()
		item, err := s.Update(context.Background(), 2, 7, "  新标题 ", " # 新正文\n", 3)
		require.NoError(t, err)
		require.Equal(t, "新标题", item.Title)
		require.Equal(t, "# 新正文", item.Content)
		require.Equal(t, uint(4), item.Version)
		sum := sha256.Sum256([]byte("# 新正文"))
		require.Equal(t, hex.EncodeToString(sum[:]), item.SourceHash)
		require.Equal(t, "新标题", f.item.Title)
		require.Len(t, spy.entries, 1)
		entry := spy.entries[0]
		require.Equal(t, model.OperationLogActionKnowledgeUpdate, entry.Action)
		require.Equal(t, model.OperationLogTargetTypeKnowledge, entry.TargetType)
		require.Equal(t, uint(7), entry.TargetID)
		require.Equal(t, "新标题", entry.TargetName)
		require.Equal(t, "学习资料", entry.FolderName)
		require.JSONEq(t, `{"version":4}`, entry.Detail)
		require.Equal(t, 1, spy.pruned)
	})

	t.Run("unchanged content keeps version and logs nothing", func(t *testing.T) {
		_, spy, s := newFixture()
		item, err := s.Update(context.Background(), 2, 7, "旧标题", "旧正文", 3)
		require.NoError(t, err)
		require.Equal(t, uint(3), item.Version)
		require.Empty(t, spy.entries)
	})

	t.Run("stale base version conflicts", func(t *testing.T) {
		_, spy, s := newFixture()
		_, err := s.Update(context.Background(), 2, 7, "新标题", "新正文", 2)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
		require.Empty(t, spy.entries)
	})

	t.Run("unready item conflicts", func(t *testing.T) {
		f, _, s := newFixture()
		f.item.Status = model.KnowledgePending
		_, err := s.Update(context.Background(), 2, 7, "新标题", "新正文", 3)
		require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
	})

	t.Run("unknown item is not found", func(t *testing.T) {
		_, _, s := newFixture()
		_, err := s.Update(context.Background(), 2, 8, "新标题", "新正文", 3)
		require.ErrorIs(t, err, ErrKnowledgeNotFound)
	})

	for _, tc := range []struct {
		name        string
		title       string
		content     string
		baseVersion uint
	}{
		{"missing base version", "标题", "正文", 0},
		{"empty title", "", "正文", 3},
		{"long title", strings.Repeat("知", 201), "正文", 3},
		{"empty content", "标题", "", 3},
		{"invalid UTF-8 content", "标题", "\xff\xfe", 3},
		{"NUL content", "标题", "abc\x00def", 3},
		{"oversized content", "标题", strings.Repeat("文", ManualContentMaxRunes+1), 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, spy, s := newFixture()
			_, err := s.Update(context.Background(), 2, 7, tc.title, tc.content, tc.baseVersion)
			require.ErrorIs(t, err, ErrKnowledgeInvalid)
			require.Empty(t, spy.entries)
		})
	}
}
