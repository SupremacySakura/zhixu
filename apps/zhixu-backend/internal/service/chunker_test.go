package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestChunkMarkdownUnicodeAndExactOverlap(t *testing.T) {
	content := "# 笔记\n" + strings.Repeat("知序😀", 600) + "\n## 第二节\n尾部"
	chunks, err := ChunkMarkdown(content, 512, 64)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 3)
	source := []rune(content)
	covered := make([]bool, len(source))
	for i, chunk := range chunks {
		require.Equal(t, i+1, chunk.Seq)
		require.LessOrEqual(t, chunk.Chars, 512)
		require.Equal(t, utf8.RuneCountInString(chunk.Content), chunk.Chars)
		require.Equal(t, string(source[chunk.StartChar:chunk.EndChar]), chunk.Content)
		for k := chunk.StartChar; k < chunk.EndChar; k++ {
			covered[k] = true
		}
		if chunk.OverlapChars > 0 {
			previous := []rune(chunks[i-1].Content)
			require.Equal(t, string(previous[len(previous)-chunk.OverlapChars:]), string([]rune(chunk.Content)[:chunk.OverlapChars]))
		}
	}
	for _, value := range covered {
		require.True(t, value)
	}
	last := chunks[len(chunks)-1]
	require.Equal(t, "笔记 / 第二节", last.Section)
	require.Zero(t, last.OverlapChars)
}

func TestChunkMarkdownIgnoresFencedHeadingsAndEmptySections(t *testing.T) {
	content := "# 标题\n```md\n# 代码里的标题\n```\n### 小节\n正文\n"
	chunks, err := ChunkMarkdown(content, 512, 64)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	require.Equal(t, "标题", chunks[0].Section)
	require.Equal(t, "标题 / 小节", chunks[1].Section)
	require.Equal(t, 3, chunks[1].SectionLevel)
	chunks, err = ChunkMarkdown(" \n\t", 512, 64)
	require.NoError(t, err)
	require.Empty(t, chunks)
	for _, window := range [][2]int{{0, 0}, {64, 64}, {5, -1}} {
		_, err = ChunkMarkdown("正文", window[0], window[1])
		require.Error(t, err)
	}
}
