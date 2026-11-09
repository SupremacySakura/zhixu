package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	pdf "github.com/ledongthuc/pdf"
)

const maxPDFBytes = 20 * 1024 * 1024

// pdfExtractor 逐页提取文本层，每页加 "## 第 N 页" 标题便于分块与定位；
// 扫描件（无文本层）与加密文件明确报错，不静默产出空知识。
type pdfExtractor struct{}

func (pdfExtractor) SourceType() string { return "pdf" }
func (pdfExtractor) Label() string      { return "PDF" }
func (pdfExtractor) MaxBytes() int64    { return maxPDFBytes }

func (pdfExtractor) MatchesExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pdf")
}

func (e pdfExtractor) Extract(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	reader, err := pdf.NewReader(file, info.Size())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "encrypt") {
			return "", failMarkdown("encrypted_file", "PDF 已加密，暂不支持加密文件")
		}
		return "", failMarkdown("parse_failed", "PDF 解析失败，文件可能已损坏")
	}
	var b strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, pageErr := page.GetPlainText(nil)
		if pageErr != nil {
			return "", failMarkdown("parse_failed", fmt.Sprintf("PDF 第 %d 页解析失败，文件可能已损坏", i))
		}
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "## 第 %d 页\n\n%s\n\n", i, text)
	}
	content := b.String()
	if !utf8.ValidString(content) {
		// 个别 PDF 字体编码不规范会产生非法 UTF-8，剥离而不是整篇拒绝。
		content = strings.ToValidUTF8(content, "")
	}
	if strings.TrimSpace(content) == "" {
		return "", failMarkdown("scanned_pdf_no_text", "未提取到文本：该 PDF 可能是扫描件（图片型），暂不支持 OCR")
	}
	return content, nil
}
