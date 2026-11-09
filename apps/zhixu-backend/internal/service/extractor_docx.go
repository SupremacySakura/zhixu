package service

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxDocxBytes = 20 * 1024 * 1024

// maxDocxLinkBytes 限制外链目标长度：链接文本按节点重复输出 URL，
// 超长 URL 能在正文构建期放大内存，超过该长度的链接退化为纯文本。
const maxDocxLinkBytes = 2048

var (
	docxHeadingIDRe   = regexp.MustCompile(`(?i)^heading\s*([1-9])$`)
	docxHeadingNameRe = regexp.MustCompile(`(?i)^(?:heading|标题|標題)\s*([1-9])$`)
)

// docxExtractor 解析 word/document.xml：标题样式→# 层级、编号列表→-/1. 缩进列表、
// 外链→[文本](url)、表格→GFM 表格；图片、样式、页眉页脚等版式信息刻意丢弃。
type docxExtractor struct{}

func (docxExtractor) SourceType() string { return "docx" }
func (docxExtractor) Label() string      { return "Word" }
func (docxExtractor) MaxBytes() int64    { return maxDocxBytes }

func (docxExtractor) MatchesExt(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".docx")
}

func (e docxExtractor) Extract(file *os.File) (string, error) {
	zr, fail := openOOXML(file)
	if fail != nil {
		return "", fail
	}
	budget := int64(maxOOXMLUncompressedBytes)
	doc, fail := readOOXMLPart(zr, "word/document.xml", &budget)
	if fail != nil {
		return "", fail
	}
	if doc == nil {
		return "", failMarkdown("parse_failed", "DOCX 缺少正文部件，文件可能已损坏")
	}
	rels, fail := readOOXMLPart(zr, "word/_rels/document.xml.rels", &budget)
	if fail != nil {
		return "", fail
	}
	styles, fail := readOOXMLPart(zr, "word/styles.xml", &budget)
	if fail != nil {
		return "", fail
	}
	numbering, fail := readOOXMLPart(zr, "word/numbering.xml", &budget)
	if fail != nil {
		return "", fail
	}
	content, fail := docxToMarkdown(doc, docxHeadingLevels(styles), docxOrderedNums(numbering), docxExternalLinks(rels))
	if fail != nil {
		return "", fail
	}
	if strings.TrimSpace(content) == "" {
		return "", failMarkdown("empty_content", "DOCX 正文为空，未提取到有效文本")
	}
	return content, nil
}

// docxExternalLinks 从 document.xml.rels 提取外链 rid→url（仅 TargetMode=External）。
func docxExternalLinks(b []byte) map[string]string {
	links := map[string]string{}
	if len(b) == 0 {
		return links
	}
	d := newXMLDecoder(b)
	for {
		tok, err := d.Token()
		if err != nil {
			return links
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		id, target, mode := xmlAttr(se, "Id"), xmlAttr(se, "Target"), xmlAttr(se, "TargetMode")
		if id != "" && target != "" && strings.EqualFold(mode, "External") && len(target) <= maxDocxLinkBytes {
			links[id] = target
		}
	}
}

// docxHeadingLevels 从 styles.xml 提取 styleId→标题层级（按样式名识别，兼容中英文命名）。
func docxHeadingLevels(b []byte) map[string]int {
	levels := map[string]int{}
	if len(b) == 0 {
		return levels
	}
	d := newXMLDecoder(b)
	var curID string
	for {
		tok, err := d.Token()
		if err != nil {
			return levels
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "style":
			curID = xmlAttr(se, "styleId")
		case "name":
			if curID == "" {
				break
			}
			if m := docxHeadingNameRe.FindStringSubmatch(xmlAttr(se, "val")); m != nil {
				levels[curID] = int(m[1][0] - '0')
			}
		}
	}
}

// docxOrderedNums 从 numbering.xml 推断 numId→是否有序列表（仅看 ilvl=0 的编号格式）。
func docxOrderedNums(b []byte) map[string]bool {
	ordered := map[string]bool{}
	if len(b) == 0 {
		return ordered
	}
	d := newXMLDecoder(b)
	absOrdered, absSeen := map[string]bool{}, map[string]bool{}
	numToAbs := map[string]string{}
	var curAbs, curNum string
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "abstractNum":
				curAbs = xmlAttr(se, "abstractNumId")
			case "num":
				curNum = xmlAttr(se, "numId")
			case "abstractNumId":
				if curNum != "" {
					numToAbs[curNum] = xmlAttr(se, "val")
				}
			case "numFmt":
				if curAbs != "" && !absSeen[curAbs] {
					absSeen[curAbs] = true
					absOrdered[curAbs] = docxIsOrderedFmt(xmlAttr(se, "val"))
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "abstractNum":
				curAbs = ""
			case "num":
				curNum = ""
			}
		}
	}
	for num, abs := range numToAbs {
		ordered[num] = absOrdered[abs]
	}
	return ordered
}

func docxIsOrderedFmt(f string) bool {
	switch f {
	case "decimal", "decimalZero", "lowerLetter", "upperLetter", "lowerRoman", "upperRoman",
		"chineseCounting", "chineseCountingThousand", "ideographDigital":
		return true
	}
	return false
}

func docxHeadingLevel(style string, headings map[string]int) int {
	if style == "" {
		return 0
	}
	if lv, ok := headings[style]; ok {
		return lv
	}
	if m := docxHeadingIDRe.FindStringSubmatch(style); m != nil {
		return int(m[1][0] - '0')
	}
	return 0
}

func docxToMarkdown(b []byte, headings map[string]int, ordered map[string]bool, links map[string]string) (string, error) {
	d := newXMLDecoder(b)
	// 正文与单段落缓冲都设上限：外链等结构会重复输出文本，必须在构建期拦截放大。
	out := &cappedBuilder{limit: maxMarkdownExpandedBytes}
	para := &cappedBuilder{limit: maxMarkdownExpandedBytes}
	over := false
	inP := false
	lastWasList := false
	style, numID, curLink := "", "", ""
	ilvl := 0
	writeBlock := func(s string, isList bool) {
		if s == "" {
			return
		}
		if lastWasList && !isList {
			out.WriteString("\n")
		}
		out.WriteString(s)
		lastWasList = isList
	}
	flush := func() {
		text := strings.TrimSpace(para.String())
		if para.Overflow() {
			over = true
		}
		para.Reset()
		if text == "" {
			return
		}
		if lv := docxHeadingLevel(style, headings); lv > 0 {
			writeBlock(strings.Repeat("#", lv)+" "+text+"\n\n", false)
			return
		}
		if numID != "" {
			marker := "-"
			if ordered[numID] {
				marker = "1."
			}
			if ilvl > 4 {
				ilvl = 4
			}
			writeBlock(strings.Repeat("  ", ilvl)+marker+" "+text+"\n", true)
			return
		}
		writeBlock(text+"\n\n", false)
	}
	appendText := func(s string) {
		if curLink != "" && strings.TrimSpace(s) != "" {
			para.WriteString("[" + escapeMDLinkText(s) + "](" + escapeMDURL(curLink) + ")")
			return
		}
		para.WriteString(s)
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				break
			}
			// 非正常结束说明 XML 被截断：不能把已解析部分当作完整正文入库。
			return "", failTruncatedXML()
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if !inP {
					inP, style, numID, curLink, ilvl = true, "", "", "", 0
				}
			case "pStyle":
				if inP {
					style = xmlAttr(t, "val")
				}
			case "numPr":
				if inP {
					numID, ilvl = docxReadNumPr(d)
				}
			case "hyperlink":
				if inP {
					curLink = links[xmlAttr(t, "id")]
				}
			case "t":
				if inP {
					appendText(readCharData(d))
				}
			case "tab", "br", "cr":
				if inP {
					para.WriteString(" ")
				}
			case "tbl":
				if inP {
					flush()
					inP = false
				}
				// 表格先写入限写缓冲：参差表格按最大宽度补齐，不能先在无上限 builders 里放大。
				table := &cappedBuilder{limit: maxMarkdownExpandedBytes}
				if fail := docxTableToMarkdown(d, table); fail != nil {
					return "", fail
				}
				if table.Overflow() {
					return "", failMarkdown("file_too_large", "DOCX 提取出的正文过大，无法加入知识库")
				}
				writeBlock(table.String(), false)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inP {
					flush()
					inP = false
				}
			case "hyperlink":
				curLink = ""
			}
		}
	}
	if inP {
		flush()
	}
	if over || out.Overflow() {
		return "", failMarkdown("file_too_large", "DOCX 提取出的正文过大，无法加入知识库")
	}
	return out.String(), nil
}

// docxReadNumPr 消费 numPr 子元素，返回 numId 与 ilvl。
func docxReadNumPr(d *xml.Decoder) (numID string, ilvl int) {
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch e := tok.(type) {
		case xml.StartElement:
			depth++
			switch e.Name.Local {
			case "ilvl":
				if v := xmlAttr(e, "val"); v != "" {
					ilvl = int(v[0] - '0')
					if ilvl < 0 || ilvl > 9 {
						ilvl = 0
					}
				}
			case "numId":
				numID = xmlAttr(e, "val")
			}
		case xml.EndElement:
			depth--
		}
	}
	return
}

// docxTableToMarkdown 消费 tbl 元素流（含嵌套表容错），输出 GFM 表格。
// 表格按最大列宽补齐后会放大渲染量，因此单元格总数与输出大小都要受限。
func docxTableToMarkdown(d *xml.Decoder, out markdownWriter) error {
	var rows [][]string
	var cells []string
	var cell strings.Builder
	inRow, inCell := false, false
	nested, total := 0, 0
	for {
		tok, err := d.Token()
		if err != nil {
			if xmlEnd(err) {
				break
			}
			return failTruncatedXML()
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "tbl":
				nested++
			case "tr":
				if nested == 0 {
					inRow, cells = true, nil
				}
			case "tc":
				if nested == 0 && inRow {
					inCell = true
					cell.Reset()
				}
			case "t":
				if inCell {
					cell.WriteString(readCharData(d))
				}
			case "tab", "br":
				if inCell {
					cell.WriteString(" ")
				}
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "tbl":
				if nested == 0 {
					return writeMarkdownTable(out, rows, maxTableCells)
				}
				nested--
			case "tr":
				if inRow {
					rows = append(rows, cells)
					inRow = false
				}
			case "tc":
				if inCell {
					cells = append(cells, cell.String())
					inCell = false
					total++
					if total > maxTableCells {
						return failMarkdown("file_too_large", "DOCX 表格单元格数量过多，无法加入知识库")
					}
				}
			case "p":
				if inCell {
					cell.WriteString(" ")
				}
			}
		}
	}
	return writeMarkdownTable(out, rows, maxTableCells)
}

func escapeMDURL(u string) string {
	return strings.NewReplacer("(", "%28", ")", "%29", " ", "%20").Replace(strings.TrimSpace(u))
}

func escapeMDLinkText(s string) string {
	return strings.NewReplacer("[", "\\[", "]", "\\]").Replace(s)
}
