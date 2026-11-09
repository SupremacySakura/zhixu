package service

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const maxTextBytes = 20 * 1024 * 1024

// textExtractor 处理 .txt：自动识别 UTF-8 / UTF-16（BOM）/ GBK，统一归一为 UTF-8 + LF 换行。
type textExtractor struct{}

func (textExtractor) SourceType() string { return "text" }
func (textExtractor) Label() string      { return "文本" }
func (textExtractor) MaxBytes() int64    { return maxTextBytes }

func (textExtractor) MatchesExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".txt")
}

func (e textExtractor) Extract(file *os.File) (string, error) {
	data, err := io.ReadAll(io.LimitReader(file, e.MaxBytes()+1))
	if err != nil {
		return "", failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	if int64(len(data)) > e.MaxBytes() {
		return "", failMarkdown("file_too_large", "文本文件不能超过 20 MB")
	}
	text, err := decodeText(data)
	if err != nil {
		return "", err
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if strings.TrimSpace(text) == "" {
		return "", failMarkdown("empty_content", "未读取到有效正文，原文件为空")
	}
	return text, nil
}

// decodeText 按 BOM → UTF-8 → GBK 的顺序识别编码；
// 解码产生替换符（U+FFFD）或 NUL 视为无法识别，拒绝入库而不是留存乱码。
func decodeText(data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeWith(data, unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM))
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeWith(data, unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM))
	}
	if utf8.Valid(data) && !bytes.ContainsRune(data, 0) {
		return string(data), nil
	}
	return decodeWith(data, simplifiedchinese.GBK)
}

func decodeWith(data []byte, enc encoding.Encoding) (string, error) {
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), enc.NewDecoder()))
	if err != nil {
		return "", failMarkdown("invalid_encoding", "无法识别文件编码，仅支持 UTF-8 / UTF-16 / GBK 文本")
	}
	text := string(decoded)
	if strings.ContainsRune(text, utf8.RuneError) || strings.ContainsRune(text, 0) {
		return "", failMarkdown("invalid_encoding", "无法识别文件编码，仅支持 UTF-8 / UTF-16 / GBK 文本")
	}
	return text, nil
}
