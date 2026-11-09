package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zhixu-backend/internal/model"
)

// Extractor 将一种来源格式提取为规范化 Markdown 正文。
// 所有格式在入库前统一收敛为 Markdown 文本，下游分块/向量化/检索对格式无感知。
// 新增格式：实现本接口并注册到 extractors 即可，无需改动任务状态机。
type Extractor interface {
	// SourceType 写入 knowledge_items.source_type，处理任务据此找回提取器。
	SourceType() string
	// Label 展示名，用于大小限制等提示文案。
	Label() string
	// MatchesExt 按文件名扩展名判断是否由本提取器处理（大小写不敏感）。
	MatchesExt(name string) bool
	// MaxBytes 该格式允许的源文件大小上限，加入时（元数据）与读取时（实际内容）双重校验。
	MaxBytes() int64
	// Extract 从已打开的源文件读取并返回规范化 Markdown 正文。
	// 业务失败返回 *markdownFailure（错误码直接落库）；其余错误视为瞬时故障，交由租约过期重试。
	Extract(file *os.File) (string, error)
}

// extractors 按注册顺序匹配扩展名，首个匹配生效。
var extractors = []Extractor{markdownExtractor{}, textExtractor{}, pdfExtractor{}, docxExtractor{}, csvExtractor{}, xlsxExtractor{}}

func extractorForExt(name string) Extractor {
	for _, e := range extractors {
		if e.MatchesExt(name) {
			return e
		}
	}
	return nil
}

// extractorForSourceType 按入库时记录的 source_type 找回提取器；
// 空值按 markdown 处理，兼容早期数据与历史任务。
func extractorForSourceType(sourceType string) Extractor {
	if sourceType == "" {
		sourceType = "markdown"
	}
	for _, e := range extractors {
		if e.SourceType() == sourceType {
			return e
		}
	}
	return nil
}

// validateSource 校验文件可知识化并返回对应提取器。
func validateSource(file *model.FileModel) (Extractor, error) {
	e := extractorForExt(file.Name)
	if e == nil {
		return nil, fmt.Errorf("%w: 暂不支持将该格式加入知识库", ErrKnowledgeInvalid)
	}
	if file.Size > uint64(e.MaxBytes()) {
		return nil, fmt.Errorf("%w: %s文件不能超过 %d MB", ErrKnowledgeInvalid, e.Label(), e.MaxBytes()/1024/1024)
	}
	return e, nil
}

// markdownFailure 是所有提取器共用的业务失败类型：code 落库为 error_code，message 直接展示。
type markdownFailure struct{ code, message string }

func (e *markdownFailure) Error() string      { return e.message }
func failMarkdown(code, message string) error { return &markdownFailure{code, message} }

// openSourceFile 按哈希分片路径打开源文件，兼容早期 hash.extension 布局。
// 只接受十六进制哈希；os.Root 阻止路径逃逸与符号链接穿越。
func openSourceFile(storagePath, hash string) (*os.File, error) {
	if len(hash) < 4 || len(hash) > 128 {
		return nil, failMarkdown("invalid_source", "原文件存储标识无效")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return nil, failMarkdown("invalid_source", "原文件存储标识无效")
	}
	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return nil, failMarkdown("source_unavailable", "无法读取原文件，请稍后重试")
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
		return nil, failMarkdown("source_unavailable", "无法读取原文件，请确认文件仍存在后重试")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, failMarkdown("source_unavailable", "原文件不是可读取的普通文件")
	}
	return file, nil
}

// callExtractor 执行提取并把 panic 收敛为 parse_failed，避免格式解析拖垮导入 worker。
func callExtractor(e Extractor, file *os.File) (content string, err error) {
	defer func() {
		if r := recover(); r != nil {
			content, err = "", failMarkdown("parse_failed", "文件内容解析失败，请确认文件未损坏后重试")
		}
	}()
	return e.Extract(file)
}

// extractSource 打开源文件并分派给 source_type 对应的提取器，做统一的大小预检。
func extractSource(storagePath, hash, sourceType string) (string, error) {
	e := extractorForSourceType(sourceType)
	if e == nil {
		return "", failMarkdown("unsupported_format", "暂不支持的知识来源格式")
	}
	file, err := openSourceFile(storagePath, hash)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if info, statErr := file.Stat(); statErr == nil && info.Size() > e.MaxBytes() {
		return "", failMarkdown("file_too_large", fmt.Sprintf("%s文件不能超过 %d MB", e.Label(), e.MaxBytes()/1024/1024))
	}
	return callExtractor(e, file)
}

// readMarkdown 为既有调用方与测试保留的入口：按 markdown 提取源文件正文。
func readMarkdown(storagePath, hash string) (string, error) {
	return extractSource(storagePath, hash, "markdown")
}
