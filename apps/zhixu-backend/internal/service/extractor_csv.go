package service

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxCSVBytes = 20 * 1024 * 1024

// csvExtractor 复用文本编码探测（UTF-8/UTF-16/GBK），读取为记录后输出 GFM 表格，
// 首行作表头；兼容参差行与不规整引号。
type csvExtractor struct{}

func (csvExtractor) SourceType() string { return "csv" }
func (csvExtractor) Label() string      { return "CSV" }
func (csvExtractor) MaxBytes() int64    { return maxCSVBytes }

func (csvExtractor) MatchesExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".csv")
}

func (e csvExtractor) Extract(file *os.File) (string, error) {
	data, err := io.ReadAll(io.LimitReader(file, maxCSVBytes+1))
	if err != nil {
		return "", failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	if len(data) > maxCSVBytes {
		return "", failMarkdown("file_too_large", "CSV 文件不能超过 20 MB")
	}
	text, err := decodeText(data)
	if err != nil {
		return "", err
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return "", failMarkdown("parse_failed", "CSV 解析失败，请检查分隔符与引号格式")
	}
	rows := make([][]string, 0, len(records))
	for _, rec := range records {
		nonEmpty := false
		for _, field := range rec {
			if strings.TrimSpace(field) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			rows = append(rows, rec)
		}
	}
	if len(rows) == 0 {
		return "", failMarkdown("empty_content", "CSV 内容为空，未读取到有效数据")
	}
	// CSV 输入本身 ≤ 20 MiB，行数受限；但参差行按最大宽度补齐仍会放大输出，
	// 同样用限写构建器兜底，写满即停。
	out := &cappedBuilder{limit: maxMarkdownExpandedBytes}
	if fail := writeMarkdownTable(out, rows, 0); fail != nil {
		return "", fail
	}
	if out.Overflow() {
		return "", failMarkdown("file_too_large", "CSV 展开后的表格内容过大，无法加入知识库")
	}
	return out.String(), nil
}
