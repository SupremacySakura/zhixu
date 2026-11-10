package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

func TestDecodeText(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    []byte
		expect  string
		invalid bool
	}{
		{"utf8", []byte("你好，世界\n第二行"), "你好，世界\n第二行", false},
		{"utf8 bom", append([]byte{0xEF, 0xBB, 0xBF}, []byte("正文")...), "正文", false},
		// "，中文" 的 GBK 编码；0xA3 是非法 UTF-8 起始字节，保证落入 GBK 分支。
		// 注意 "中文" 二字的 GBK 编码（D6D0 CEC4）恰好也是合法 UTF-8，这类歧义按 UTF-8 优先处理。
		{"gbk", []byte{0xA3, 0xAC, 0xD6, 0xD0, 0xCE, 0xC4}, "，中文", false},
		// "中文" 的 UTF-16LE/BE（带 BOM）。
		{"utf16le bom", []byte{0xFF, 0xFE, 0x2D, 0x4E, 0x87, 0x65}, "中文", false},
		{"utf16be bom", []byte{0xFE, 0xFF, 0x4E, 0x2D, 0x65, 0x87}, "中文", false},
		// GBK 中 0xFF 无映射 → 替换符 → 拒绝。
		{"unrecognized", []byte{0xFF}, "", true},
		{"binary nul", []byte{'a', 'b', 0x00}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, err := decodeText(tc.data)
			if tc.invalid {
				var failure *markdownFailure
				require.ErrorAs(t, err, &failure)
				require.Equal(t, "invalid_encoding", failure.code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, text)
		})
	}
}

func TestTextExtractorLifecycle(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "aa", "bb")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aabbccdd"), []byte("第一行\r\n第二行\r第三行"), 0600))

	t.Run("add sets text source type", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "笔记.TXT", FileHash: "aabbccdd", Size: 1}}
		item, created, err := NewKnowledgeService(f, root).Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		require.Equal(t, "text", item.SourceType)
		require.Equal(t, "笔记", item.Title)
	})

	t.Run("process normalizes newlines", func(t *testing.T) {
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "note.txt", FileHash: "aabbccdd"},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: "aabbccdd", SourceType: "text"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		require.Equal(t, "第一行\n第二行\n第三行", f.content)
	})

	t.Run("oversize text rejected at add", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "big.txt", FileHash: "aabbccdd", Size: maxTextBytes + 1}}
		_, _, err := NewKnowledgeService(f, root).Add(context.Background(), 2, 1, "")
		require.ErrorIs(t, err, ErrKnowledgeInvalid)
	})

	t.Run("empty text reports empty_content", func(t *testing.T) {
		empty := t.TempDir()
		dir := filepath.Join(empty, "aa", "bb")
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "aabbccdd"), []byte(strings.Repeat(" \n", 3)), 0600))
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "empty.txt", FileHash: "aabbccdd"},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: "aabbccdd", SourceType: "text"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, empty).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "empty_content", f.code)
	})
}
