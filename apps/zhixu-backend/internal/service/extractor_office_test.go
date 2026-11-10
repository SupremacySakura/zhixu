package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

// buildZipFixture 用内存 zip 构造 OOXML 夹具。
func buildZipFixture(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range parts {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// writeZipSource 将 zip 夹具写入按 hash 分片的存储路径。
func writeZipSource(t *testing.T, root, hash string, parts map[string]string) {
	t.Helper()
	writeSourceFile(t, root, hash, buildZipFixture(t, parts))
}

func docxFakeTask(hash string) *knowledgeFake {
	return &knowledgeFake{
		file: &model.FileModel{ID: 1, UserID: 2, Name: "report.docx", FileHash: hash},
		item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "docx"},
		task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
	}
}

func xlsxFakeTask(hash string) *knowledgeFake {
	return &knowledgeFake{
		file: &model.FileModel{ID: 1, UserID: 2, Name: "data.xlsx", FileHash: hash},
		item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "xlsx"},
		task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
	}
}

func docxFixture() map[string]string {
	return map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<w:body>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>年度报告</w:t></w:r></w:p>
<w:p><w:r><w:t>正文第一段。</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="3"/></w:numPr></w:pPr><w:r><w:t>第一项</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="3"/></w:numPr></w:pPr><w:r><w:t>子项</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="4"/></w:numPr></w:pPr><w:r><w:t>有序项</w:t></w:r></w:p>
<w:p><w:hyperlink r:id="rId9"><w:r><w:t>示例链接</w:t></w:r></w:hyperlink></w:p>
<w:tbl>
<w:tr><w:tc><w:p><w:r><w:t>姓名</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>年龄</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>张三</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>30</w:t></w:r></w:p></w:tc></w:tr>
</w:tbl>
<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>二级标题</w:t></w:r></w:p>
</w:body></w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/></w:style>
</w:styles>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>
<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>
<w:num w:numId="3"><w:abstractNumId w:val="0"/></w:num>
<w:num w:numId="4"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`,
	}
}

func TestDocxExtractorLifecycle(t *testing.T) {
	t.Run("converts document to markdown", func(t *testing.T) {
		root := t.TempDir()
		hash := "aabbccddeeff"
		writeZipSource(t, root, hash, docxFixture())
		f := docxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		for _, want := range []string{
			"# 年度报告",
			"正文第一段。",
			"- 第一项",
			"  - 子项",
			"1. 有序项",
			"[示例链接](https://example.com)",
			"| 姓名 | 年龄 |",
			"| --- | --- |",
			"| 张三 | 30 |",
			"## 二级标题",
		} {
			require.Contains(t, f.content, want)
		}
	})

	t.Run("add sets docx source type", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "季度总结.DOCX", FileHash: "aabbccdd", Size: 1}}
		item, created, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		require.Equal(t, "docx", item.SourceType)
		require.Equal(t, "季度总结", item.Title)
	})

	t.Run("missing document part reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "bbccddeeff00"
		writeZipSource(t, root, hash, map[string]string{"word/styles.xml": "<w:styles/>"})
		f := docxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("non-zip file reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "ccddeeff0011"
		writeSourceFile(t, root, hash, []byte("not a zip file"))
		f := docxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("truncated document xml reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "ccddeeff0012"
		writeZipSource(t, root, hash, map[string]string{
			"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>截断正文`,
		})
		f := docxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
		require.NotContains(t, f.content, "截断正文")
	})

	t.Run("zip bomb guarded by decompressed budget", func(t *testing.T) {
		root := t.TempDir()
		hash := "ddeeff001122"
		huge := strings.Repeat("a", maxOOXMLPartBytes+1024)
		writeZipSource(t, root, hash, map[string]string{"word/document.xml": huge})
		f := docxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "file_too_large", f.code)
	})

	t.Run("oversize docx rejected at add", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "big.docx", FileHash: "aabbccdd", Size: maxDocxBytes + 1}}
		_, _, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.ErrorIs(t, err, ErrKnowledgeInvalid)
	})
}

func TestXlsxExtractorLifecycle(t *testing.T) {
	xlsxFixture := func() map[string]string {
		return map[string]string{
			"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>
<sheet name="销售" sheetId="1" r:id="rId1"/>
<sheet name="汇总" sheetId="2" r:id="rId2"/>
</sheets></workbook>`,
			"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
</Relationships>`,
			"xl/sharedStrings.xml": `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
<si><t>产品</t></si><si><t>数量</t></si><si><t>苹果</t></si></sst>`,
			"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>42</v></c></row>
<row r="3"><c r="B3" t="inlineStr"><is><t>备注列</t></is></c></row>
</sheetData></worksheet>`,
			"xl/worksheets/sheet2.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData></sheetData></worksheet>`,
		}
	}

	t.Run("converts sheets to markdown tables", func(t *testing.T) {
		root := t.TempDir()
		hash := "aabbccddeeff"
		writeZipSource(t, root, hash, xlsxFixture())
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "销售表.xlsx", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "xlsx"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		for _, want := range []string{
			"## 销售",
			"| 产品 | 数量 |",
			"| 苹果 | 42 |",
			"|  | 备注列 |",
		} {
			require.Contains(t, f.content, want)
		}
		require.NotContains(t, f.content, "## 汇总")
	})

	t.Run("truncated sheet xml reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "ddeeff001133"
		parts := xlsxFixture()
		parts["xl/worksheets/sheet1.xml"] = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>42</v>`
		writeZipSource(t, root, hash, parts)
		f := xlsxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("rejects column reference beyond excel limit", func(t *testing.T) {
		root := t.TempDir()
		hash := "ddeeff001144"
		parts := xlsxFixture()
		parts["xl/worksheets/sheet1.xml"] = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="ZZZZZZZZ1"><v>1</v></c></row></sheetData></worksheet>`
		writeZipSource(t, root, hash, parts)
		f := xlsxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("limits markdown expanded from shared strings", func(t *testing.T) {
		root := t.TempDir()
		hash := "ddeeff001155"
		long := strings.Repeat("x", 4096)
		var sheet strings.Builder
		sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
		for r := 0; r < 800; r++ {
			sheet.WriteString(`<row>`)
			for c := 0; c < 10; c++ {
				sheet.WriteString(`<c t="s"><v>0</v></c>`)
			}
			sheet.WriteString(`</row>`)
		}
		sheet.WriteString(`</sheetData></worksheet>`)
		writeZipSource(t, root, hash, map[string]string{
			"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="大表" sheetId="1" r:id="rId1"/></sheets></workbook>`,
			"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
			"xl/sharedStrings.xml":       `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>` + long + `</t></si></sst>`,
			"xl/worksheets/sheet1.xml":   sheet.String(),
		})
		f := xlsxFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "file_too_large", f.code)
	})

	t.Run("missing workbook reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "bbccddeeff00"
		writeZipSource(t, root, hash, map[string]string{"xl/sharedStrings.xml": "<sst/>"})
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "broken.xlsx", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "xlsx"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("empty workbook reports empty_content", func(t *testing.T) {
		root := t.TempDir()
		hash := "ccddeeff0011"
		writeZipSource(t, root, hash, map[string]string{
			"xl/workbook.xml":          `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets></sheets></workbook>`,
			"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData></sheetData></worksheet>`,
		})
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "empty.xlsx", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "xlsx"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "empty_content", f.code)
	})
}

func TestCSVExtractorLifecycle(t *testing.T) {
	csvFakeTask := func(hash string) *knowledgeFake {
		return &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "数据.csv", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: uintPtr(1), SourceHash: hash, SourceType: "csv"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
	}

	t.Run("converts csv to markdown table", func(t *testing.T) {
		root := t.TempDir()
		hash := "aabbccddeeff"
		writeSourceFile(t, root, hash, []byte("姓名,年龄\n张三,30\n李四,25\n"))
		f := csvFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		for _, want := range []string{"| 姓名 | 年龄 |", "| --- | --- |", "| 张三 | 30 |", "| 李四 | 25 |"} {
			require.Contains(t, f.content, want)
		}
	})

	t.Run("decodes gbk csv", func(t *testing.T) {
		root := t.TempDir()
		hash := "bbccddeeff00"
		var buf []byte
		buf = append(buf, []byte("\xc3\xfb\xb3\xc6\xa3\xac\xca\xfd\xc1\xbf\n")...) // GBK: 名称，数量\n
		buf = append(buf, []byte("\xc6\xbb\xb9\xfb,3\n")...)                       // GBK: 苹果,3
		writeSourceFile(t, root, hash, buf)
		f := csvFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		require.Contains(t, f.content, "名称，数量")
		require.Contains(t, f.content, "苹果")
	})

	t.Run("escapes pipes and tolerates ragged rows", func(t *testing.T) {
		root := t.TempDir()
		hash := "ccddeeff0011"
		writeSourceFile(t, root, hash, []byte("a|b,c\n1,2,3\n"))
		f := csvFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		require.Contains(t, f.content, `| a\|b | c |`)
		require.Contains(t, f.content, "| 1 | 2 | 3 |")
		// 参差行按最大列数补齐，否则 markdown-it 会丢弃超出表头的单元格。
		require.Contains(t, f.content, "| --- | --- | --- |")
	})

	t.Run("blank file reports empty_content", func(t *testing.T) {
		root := t.TempDir()
		hash := "ddeeff001122"
		writeSourceFile(t, root, hash, []byte("  \n\n"))
		f := csvFakeTask(hash)
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "empty_content", f.code)
	})

	t.Run("add sets csv source type", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "报表.CSV", FileHash: "aabbccdd", Size: 1}}
		item, created, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		require.Equal(t, "csv", item.SourceType)
		require.Equal(t, "报表", item.Title)
	})

	t.Run("oversize csv rejected at add", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "big.csv", FileHash: "aabbccdd", Size: maxCSVBytes + 1}}
		_, _, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.ErrorIs(t, err, ErrKnowledgeInvalid)
	})
}

func TestXlsxColIndex(t *testing.T) {
	cases := []struct {
		ref  string
		want int
	}{
		{ref: "A1", want: 0},
		{ref: "B2", want: 1},
		{ref: "Z1", want: 25},
		{ref: "AA1", want: 26},
		{ref: "XFD1", want: 16383},
		{ref: "XFE1", want: -2},      // 超出 Excel 列上限
		{ref: "ZZZZZZZZ1", want: -2}, // 拒绝超大列号，避免补齐循环分配巨量空单元格
		{ref: "", want: -1},          // 无列字母：位置未知
		{ref: "1", want: -1},         // 仅行号
	}
	for _, c := range cases {
		require.Equal(t, c.want, xlsxColIndex(c.ref), "ref=%q", c.ref)
	}
}

// TestXlsxSparseColumnsBounded 回归：只有最右列（XFD）有值的稀疏行会按列号补出
// 16383 个空单元格，补齐部分必须计入单元格上限，否则渲染前就会耗尽内存。
func TestXlsxSparseColumnsBounded(t *testing.T) {
	root := t.TempDir()
	hash := "eeff00112233"
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r := 1; r <= 200; r++ {
		fmt.Fprintf(&sheet, `<row r="%d"><c r="XFD%d"><v>1</v></c></row>`, r, r)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	writeZipSource(t, root, hash, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="宽表" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   sheet.String(),
	})
	f := xlsxFakeTask(hash)
	processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, "file_too_large", f.code)
	require.Contains(t, f.message, "单元格数量过多")
}

// TestDocxLinkAmplificationBounded 回归：同一超链接的每个文本节点都会重复输出 URL，
// 大小检查必须在正文构建期生效，而不是构建完成后。
func TestDocxLinkAmplificationBounded(t *testing.T) {
	url := "https://example.com/" + strings.Repeat("a", 1800)
	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` + url + `" TargetMode="External"/></Relationships>`
	anchor := `<w:r><w:t>x</w:t></w:r>`
	document := func(body string) string {
		return `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body + `</w:body></w:document>`
	}
	cases := []struct {
		name string
		body string
	}{
		{
			// 单段落内大量文本节点：段落缓冲自身就要设上限。
			name: "single paragraph",
			body: `<w:p><w:hyperlink r:id="rId9">` + strings.Repeat(anchor, 12000) + `</w:hyperlink></w:p>`,
		},
		{
			// 多段落各自引用同一长链接：正文总缓冲要设上限。
			name: "many paragraphs",
			body: strings.Repeat(`<w:p><w:hyperlink r:id="rId9">`+anchor+`</w:hyperlink></w:p>`, 12000),
		},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			hash := fmt.Sprintf("eeff1122334%d", i)
			writeZipSource(t, root, hash, map[string]string{
				"word/document.xml":            document(c.body),
				"word/_rels/document.xml.rels": rels,
			})
			f := docxFakeTask(hash)
			processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
			require.NoError(t, err)
			require.True(t, processed)
			require.Equal(t, "file_too_large", f.code)
			require.Contains(t, f.message, "正文过大")
		})
	}
}

// TestXlsxTablePaddingBounded 回归：解析期计数只看真实单元格，而渲染期会把每行补到
// 最大列宽。大量窄行 + 一个 XFD 超宽行会在写表时产生上亿次单元格遍历，必须提前拒绝。
func TestXlsxTablePaddingBounded(t *testing.T) {
	root := t.TempDir()
	hash := "eeff33445566"
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r := 1; r <= 10000; r++ {
		fmt.Fprintf(&sheet, `<row r="%d"><c r="A%d"><v>1</v></c></row>`, r, r)
	}
	fmt.Fprintf(&sheet, `<row r="10001"><c r="XFD10001"><v>1</v></c></row>`)
	sheet.WriteString(`</sheetData></worksheet>`)
	writeZipSource(t, root, hash, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="参差表" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   sheet.String(),
	})
	f := xlsxFakeTask(hash)
	processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, "file_too_large", f.code)
	require.Contains(t, f.message, "最大列宽")
}

// TestDocxTablePaddingBounded 回归：DOCX 表格不能先在无上限的临时构建器里按最大宽度
// 补齐渲染，单元格放大必须在表格阶段就被拒绝。
func TestDocxTablePaddingBounded(t *testing.T) {
	root := t.TempDir()
	hash := "eeff44556677"
	var table strings.Builder
	table.WriteString(`<w:tbl>`)
	for r := 0; r < 3000; r++ {
		table.WriteString(`<w:tr><w:tc><w:p><w:r><w:t>x</w:t></w:r></w:p></w:tc></w:tr>`)
	}
	table.WriteString(`<w:tr>`)
	for c := 0; c < 2000; c++ {
		table.WriteString(`<w:tc><w:p><w:r><w:t>y</w:t></w:r></w:p></w:tc>`)
	}
	table.WriteString(`</w:tr></w:tbl>`)
	writeZipSource(t, root, hash, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
			table.String() + `</w:body></w:document>`,
	})
	f := docxFakeTask(hash)
	processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, "file_too_large", f.code)
	require.Contains(t, f.message, "最大列宽")
}

// TestDocxOverlongLinkFallsBackToText 回归：超过 maxDocxLinkBytes 的外链目标丢弃，
// 保留可见文本，避免用超长 URL 放大输出。
func TestDocxOverlongLinkFallsBackToText(t *testing.T) {
	root := t.TempDir()
	hash := "eeff22334455"
	url := "https://example.com/" + strings.Repeat("a", maxDocxLinkBytes)
	writeZipSource(t, root, hash, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
			`<w:p><w:hyperlink r:id="rId9"><w:r><w:t>链接文本</w:t></w:r></w:hyperlink></w:p>` +
			`</w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="` + url + `" TargetMode="External"/></Relationships>`,
	})
	f := docxFakeTask(hash)
	processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, f.code)
	require.Contains(t, f.content, "链接文本")
	require.NotContains(t, f.content, "](http")
}
