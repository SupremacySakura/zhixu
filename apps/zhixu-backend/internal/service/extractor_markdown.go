package service

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxMarkdownBytes = 20 * 1024 * 1024

// markdownExtractor 处理 .md/.markdown：UTF-8 校验、去 BOM、换行归一。
type markdownExtractor struct{}

func (markdownExtractor) SourceType() string { return "markdown" }
func (markdownExtractor) Label() string      { return "Markdown" }
func (markdownExtractor) MaxBytes() int64    { return MaxMarkdownBytes }

func (markdownExtractor) MatchesExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

func (e markdownExtractor) Extract(file *os.File) (string, error) {
	data, err := io.ReadAll(io.LimitReader(file, e.MaxBytes()+1))
	if err != nil {
		return "", failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	if int64(len(data)) > e.MaxBytes() {
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
