package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"strings"
)

// OOXML(docx/xlsx) 本质是 zip 包。解析时限制解压总量与单部件大小，防解压炸弹；
// 直接使用文件大小上限挡住的攻击面之外，这里再加一道解压后的限额。
const (
	maxOOXMLUncompressedBytes = 50 * 1024 * 1024
	maxOOXMLPartBytes         = 30 * 1024 * 1024
	// maxMarkdownExpandedBytes 限制表格类格式展开为 Markdown 后的正文大小：
	// 解压限额只能拦住 zip 内的字节数，拦不住重复引用共享字符串等“渲染后放大”。
	maxMarkdownExpandedBytes = 20 * 1024 * 1024
	// maxMarkdownCells 限制单个表格文件展开后的单元格总数，控制解析期内存占用。
	maxMarkdownCells = 1_000_000
	// maxTableCells 限制表格按最大列宽补齐后的单元格总数（渲染期放大）：
	// 解析期计数只看真实单元格，而“大量窄行 + 一个超宽行”会在渲染时补齐出巨量空单元格。
	maxTableCells = 4_000_000
)

// openOOXML 以 zip 方式打开 OOXML 包。
func openOOXML(file *os.File) (*zip.Reader, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, failMarkdown("source_unavailable", "读取原文件失败，请稍后重试")
	}
	zr, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, failMarkdown("parse_failed", "文件解析失败，可能不是有效的 Office 文件或已损坏")
	}
	return zr, nil
}

// readOOXMLPart 读取包内指定部件；部件缺失返回 (nil, nil)。
// budget 为全包解压字节总预算，随读取递减；单部件另受 maxOOXMLPartBytes 限制。
func readOOXMLPart(zr *zip.Reader, name string, budget *int64) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > uint64(*budget) {
			return nil, failMarkdown("file_too_large", "Office 文件内容过大，无法处理")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, failMarkdown("parse_failed", "Office 文件内容损坏，无法读取")
		}
		defer rc.Close()
		limit := *budget
		if limit > maxOOXMLPartBytes {
			limit = maxOOXMLPartBytes
		}
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		if err != nil {
			return nil, failMarkdown("parse_failed", "Office 文件内容损坏，无法读取")
		}
		if int64(len(data)) > limit {
			return nil, failMarkdown("file_too_large", "Office 文件内容过大，无法处理")
		}
		*budget -= int64(len(data))
		return data, nil
	}
	return nil, nil
}

// newXMLDecoder 构造宽松的 XML 解码器。Go 的 encoding/xml 不解析 DTD、不展开
// 自定义实体、不访问外部资源，天然规避 XXE 与实体炸弹；此处仅放宽字符集声明。
func newXMLDecoder(b []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return d
}

// xmlAttr 按本地名取属性（忽略命名空间前缀差异）。
func xmlAttr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// readCharData 读取当前元素的字符内容，直到其 EndElement。
func readCharData(d *xml.Decoder) string {
	var sb strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.CharData:
			sb.WriteString(string(e))
		case xml.EndElement:
			return sb.String()
		}
	}
	return sb.String()
}

// escapeTableCell 转义 Markdown 表格单元格内容。
func escapeTableCell(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").
		Replace(strings.TrimSpace(s))
}

// xmlEnd 判断 XML 解码错误是否为正常结束；其他错误说明内容被截断或损坏。
func xmlEnd(err error) bool {
	return errors.Is(err, io.EOF)
}

// failTruncatedXML 把 XML 解析中的非 EOF 错误统一映射为 parse_failed，
// 避免把截断文档的“已解析部分”当作完整内容入库。
func failTruncatedXML() error {
	return failMarkdown("parse_failed", "文件内容不完整或被截断，无法解析")
}

// markdownWriter 是表格渲染的输出目标（*strings.Builder 与 cappedBuilder 均满足）。
type markdownWriter interface {
	WriteString(string) (int, error)
}

// cappedBuilder 是带总量上限的 Markdown 构建器：超限后停止写入并标记溢出。
type cappedBuilder struct {
	sb    strings.Builder
	limit int
	over  bool
}

func (b *cappedBuilder) WriteString(s string) (int, error) {
	if b.over {
		return 0, nil
	}
	if b.sb.Len()+len(s) > b.limit {
		b.over = true
		return 0, nil
	}
	return b.sb.WriteString(s)
}

func (b *cappedBuilder) String() string { return b.sb.String() }

func (b *cappedBuilder) Overflow() bool { return b.over }

// Reset 在复用同一构建器（如逐段落缓冲）时清空内容与溢出标记。
func (b *cappedBuilder) Reset() {
	b.sb.Reset()
	b.over = false
}

// overflowed 判断写入目标是否已到上限（只有 cappedBuilder 具备该能力）。
func overflowed(w markdownWriter) bool {
	o, ok := w.(interface{ Overflow() bool })
	return ok && o.Overflow()
}

// writeMarkdownTable 输出 GFM 表格：首行作表头，其余为数据行；全空行已在调用方过滤。
// 各行列数不齐时按最大列数补齐：markdown-it 只按表头列数渲染，超出部分会被丢弃。
// 补齐后的单元格总数在渲染期才能确定（width×行数），maxCells>0 时先限额再写，
// 避免“窄行 + 一个超宽行”的构造长时间占住导入 worker；写满输出上限后立即停止遍历。
func writeMarkdownTable(out markdownWriter, rows [][]string, maxCells int64) error {
	if len(rows) == 0 {
		return nil
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if maxCells > 0 && int64(width)*int64(len(rows)) > maxCells {
		return failMarkdown("file_too_large", "表格按最大列宽补齐后的内容过大，无法加入知识库")
	}
	writeRow := func(cells []string) {
		out.WriteString("|")
		for i := 0; i < width; i++ {
			if overflowed(out) {
				return
			}
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			out.WriteString(" " + escapeTableCell(cell) + " |")
		}
		out.WriteString("\n")
	}
	writeRow(rows[0])
	out.WriteString("|")
	for i := 0; i < width; i++ {
		if overflowed(out) {
			return nil
		}
		out.WriteString(" --- |")
	}
	out.WriteString("\n")
	for _, row := range rows[1:] {
		if overflowed(out) {
			return nil
		}
		writeRow(row)
	}
	return nil
}
