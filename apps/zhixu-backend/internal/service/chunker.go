package service

import (
	"fmt"
	"strings"
	"unicode"

	"zhixu-backend/internal/model"
)

const ChunkSize = 512
const ChunkOverlap = 64

// Offsets and overlap use Unicode characters, not bytes. Sections never overlap.
func ChunkMarkdown(content string, size, overlap int) ([]model.KnowledgeChunk, error) {
	if size < 1 || overlap < 0 || overlap >= size {
		return nil, fmt.Errorf("invalid chunk window")
	}
	runes := []rune(content)
	type section struct {
		start, level int
		title        string
	}
	sections := []section{{title: "正文"}}
	headings := make([]string, 6)
	offset := 0
	fence := ""
	for _, line := range strings.SplitAfter(content, "\n") {
		text := strings.TrimSpace(line)
		marker := ""
		if strings.HasPrefix(text, "```") {
			marker = "`"
		} else if strings.HasPrefix(text, "~~~") {
			marker = "~"
		}
		if marker != "" {
			if fence == "" {
				fence = marker
			} else if fence == marker && strings.Trim(text, marker) == "" {
				fence = ""
			}
		} else if fence == "" {
			level := 0
			for level < len(text) && text[level] == '#' {
				level++
			}
			if level >= 1 && level <= 6 && level < len(text) && (text[level] == ' ' || text[level] == '\t') {
				title := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(text[level:]), "#"))
				if title != "" {
					headings[level-1] = title
					for i := level; i < len(headings); i++ {
						headings[i] = ""
					}
					path := []string{}
					for _, h := range headings[:level] {
						if h != "" {
							path = append(path, h)
						}
					}
					s := section{offset, level, strings.Join(path, " / ")}
					if offset == 0 {
						sections[0] = s
					} else {
						sections = append(sections, s)
					}
				}
			}
		}
		offset += len([]rune(line))
	}
	chunks := make([]model.KnowledgeChunk, 0)
	for i, s := range sections {
		limit := len(runes)
		if i+1 < len(sections) {
			limit = sections[i+1].start
		}
		previousEnd := s.start
		for start := s.start; start < limit; {
			end := min(start+size, limit)
			if end < limit {
				// Prefer a nearby paragraph/word boundary without creating tiny windows.
				for k := end; k > start+size*3/4 && k > start+overlap; k-- {
					if unicode.IsSpace(runes[k-1]) {
						end = k
						break
					}
				}
			}
			text := string(runes[start:end])
			if strings.TrimSpace(text) != "" {
				n := max(0, previousEnd-start)
				chunks = append(chunks, model.KnowledgeChunk{
					Seq: len(chunks) + 1, Section: s.title, SectionLevel: s.level,
					Content: text, Chars: end - start, StartChar: start, EndChar: end,
					Overlap: n > 0, OverlapChars: n, State: "pending",
				})
				previousEnd = end
			}
			if end == limit {
				break
			}
			start = end - overlap
		}
	}
	return chunks, nil
}
