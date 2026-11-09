package service

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maxXlsxBytes = 20 * 1024 * 1024

// maxXlsxColumns 是 Excel 单表列上限（XFD 列，共 16384 列）。
const maxXlsxColumns = 16384

// xlsxExtractor 每个工作表转 "## 表名" + GFM 表格（首行作表头）；空表跳过；
// 公式取其缓存值，日期/货币等显示格式不做还原。
type xlsxExtractor struct{}

func (xlsxExtractor) SourceType() string { return "xlsx" }
func (xlsxExtractor) Label() string      { return "Excel" }
func (xlsxExtractor) MaxBytes() int64    { return maxXlsxBytes }

func (xlsxExtractor) MatchesExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".xlsx")
}

func (e xlsxExtractor) Extract(file *os.File) (string, error) {
	zr, fail := openOOXML(file)
	if fail != nil {
		return "", fail
	}
	budget := int64(maxOOXMLUncompressedBytes)
	wb, fail := readOOXMLPart(zr, "xl/workbook.xml", &budget)
	if fail != nil {
		return "", fail
	}
	if wb == nil {
		return "", failMarkdown("parse_failed", "XLSX 缺少工作簿部件，文件可能已损坏")
	}
	rels, fail := readOOXMLPart(zr, "xl/_rels/workbook.xml.rels", &budget)
	if fail != nil {
		return "", fail
	}
	sst, fail := readOOXMLPart(zr, "xl/sharedStrings.xml", &budget)
	if fail != nil {
		return "", fail
	}
	shared, fail := xlsxSharedStrings(sst)
	if fail != nil {
		return "", fail
	}
	targets, fail := xlsxSheetTargets(rels)
	if fail != nil {
		return "", fail
	}
	sheets, fail := xlsxSheetList(wb)
	if fail != nil {
		return "", fail
	}
	out := &cappedBuilder{limit: maxMarkdownExpandedBytes}
	emitted, cells := 0, 0
	for _, sh := range sheets {
		target := targets[sh.rid]
		if target == "" {
			continue
		}
		data, fail := readOOXMLPart(zr, target, &budget)
		if fail != nil {
			return "", fail
		}
		if data == nil {
			continue
		}
		rows, fail := xlsxSheetToMarkdown(data, shared, &cells)
		if fail != nil {
			return "", fail
		}
		if len(rows) == 0 {
			continue
		}
		out.WriteString("## " + strings.NewReplacer("\n", " ", "\r", " ").Replace(sh.name) + "\n\n")
		if fail := writeMarkdownTable(out, rows, maxTableCells); fail != nil {
			return "", fail
		}
		out.WriteString("\n")
		emitted++
	}
	if out.Overflow() {
		return "", failMarkdown("file_too_large", "XLSX 展开后的表格内容过大，无法加入知识库")
	}
	if emitted == 0 {
		return "", failMarkdown("empty_content", "XLSX 未提取到有效表格内容")
	}
	return out.String(), nil
}

type xlsxSheet struct{ name, rid string }

// xlsxSheetList 按顺序返回工作簿声明的工作表；XML 截断时报错而非返回部分结果。
func xlsxSheetList(b []byte) ([]xlsxSheet, error) {
	var sheets []xlsxSheet
	d := newXMLDecoder(b)
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				return sheets, nil
			}
			return nil, failTruncatedXML()
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		name, rid := xmlAttr(se, "name"), xmlAttr(se, "id")
		if name != "" && rid != "" {
			sheets = append(sheets, xlsxSheet{name: name, rid: rid})
		}
	}
}

// xlsxSheetTargets 从 workbook.xml.rels 映射 rid→包内 sheet 部件路径（忽略外链）。
func xlsxSheetTargets(b []byte) (map[string]string, error) {
	targets := map[string]string{}
	if len(b) == 0 {
		return targets, nil
	}
	d := newXMLDecoder(b)
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				return targets, nil
			}
			return nil, failTruncatedXML()
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		id, target, mode := xmlAttr(se, "Id"), xmlAttr(se, "Target"), xmlAttr(se, "TargetMode")
		if id == "" || target == "" || strings.EqualFold(mode, "External") {
			continue
		}
		target = strings.TrimPrefix(target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = "xl/" + target
		}
		targets[id] = target
	}
}

// xlsxSharedStrings 提取共享字符串表（富文本 run 直接拼接）。
func xlsxSharedStrings(b []byte) ([]string, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var out []string
	d := newXMLDecoder(b)
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				return out, nil
			}
			return nil, failTruncatedXML()
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "si" {
			continue
		}
		var sb strings.Builder
		depth := 1
		for depth > 0 {
			tok2, err2 := d.Token()
			if err2 != nil {
				return nil, failTruncatedXML()
			}
			switch e2 := tok2.(type) {
			case xml.StartElement:
				depth++
				if e2.Name.Local == "t" {
					sb.WriteString(readCharData(d))
					depth-- // readCharData 已消费 t 的 EndElement
				}
			case xml.EndElement:
				depth--
			}
		}
		out = append(out, sb.String())
	}
}

// xlsxColIndex 将 A1 引用的列字母转 0 基序号；无列字母返回 -1，
// 超出 Excel 列上限返回 -2。列号必须先做上限校验，否则形如 ZZZZZZZZ1
// 的引用会让单元格补齐循环分配出约 2×10¹¹ 个空字符串。
func xlsxColIndex(ref string) int {
	col := 0
	seen := false
	for _, r := range ref {
		var digit int
		switch {
		case r >= 'A' && r <= 'Z':
			digit = int(r-'A') + 1
		case r >= 'a' && r <= 'z':
			digit = int(r-'a') + 1
		default:
			if !seen || col == 0 {
				return -1
			}
			return col - 1
		}
		col = col*26 + digit
		if col > maxXlsxColumns {
			return -2
		}
		seen = true
	}
	if !seen || col == 0 {
		return -1
	}
	return col - 1
}

func xlsxResolveCell(cellType, v, inline string, shared []string) string {
	switch cellType {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && i >= 0 && i < len(shared) {
			return shared[i]
		}
		return ""
	case "inlineStr":
		return inline
	case "b":
		switch v {
		case "1":
			return "TRUE"
		case "0":
			return "FALSE"
		}
		return v
	default: // n / str / e / 空：原样取值
		return v
	}
}

// xlsxSheetToMarkdown 将 sheetData 流转为非空行；cells 累计全文件单元格数，
// 超限即失败，避免共享字符串被大量引用时把内存放大。
func xlsxSheetToMarkdown(data []byte, shared []string, cells *int) ([][]string, error) {
	d := newXMLDecoder(data)
	var rows [][]string
	var row []string
	var val, inline strings.Builder
	cellType, cellRef := "", ""
	inC := false
	lastCol := -1
	flushRow := func() {
		nonEmpty := false
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			rows = append(rows, row)
		}
		row = nil
		lastCol = -1
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				break
			}
			return nil, failTruncatedXML()
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "c":
				inC = true
				cellType, cellRef = xmlAttr(e, "t"), xmlAttr(e, "r")
				val.Reset()
				inline.Reset()
			case "v":
				if inC {
					val.WriteString(readCharData(d))
				}
			case "is":
				if inC {
					inline.WriteString(readInlineString(d))
				}
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "c":
				if inC {
					col := xlsxColIndex(cellRef)
					switch {
					case col == -2:
						return nil, failMarkdown("parse_failed", "XLSX 单元格列引用超出 Excel 列上限，文件可能已损坏")
					case col < 0 || col <= lastCol:
						col = lastCol + 1
					}
					// 稀疏列会按列号补出空单元格，补齐部分同样计入上限：
					// 否则大量“只出现在最右列”的行能在渲染前耗尽内存。
					pad := col - len(row)
					if pad < 0 {
						pad = 0
					}
					*cells = *cells + pad + 1
					if *cells > maxMarkdownCells {
						return nil, failMarkdown("file_too_large", "XLSX 单元格数量过多，请拆分后导入")
					}
					for len(row) < col {
						row = append(row, "")
					}
					row = append(row, xlsxResolveCell(cellType, val.String(), inline.String(), shared))
					lastCol = col
					inC = false
				}
			case "row":
				flushRow()
			}
		}
	}
	flushRow()
	return rows, nil
}

// readInlineString 消费 is 元素，拼接其中所有 t 文本。
func readInlineString(d *xml.Decoder) string {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			depth++
			if e.Name.Local == "t" {
				sb.WriteString(readCharData(d))
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return sb.String()
}
