package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 规范化只抹掉「看起来一样」的差异，不改变语义。
func TestNormalizeKnowledgeContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"bom", "\ufeff标题", "标题"},
		{"crlf", "a\r\nb", "a\nb"},
		{"cr", "a\rb", "a\nb"},
		{"trailing spaces", "a   \nb\t\n", "a\nb"},
		{"leading and trailing blank", "\n\n a \n\n", "a"},
		{"inner blank lines kept", "a\n\n\nb", "a\n\n\nb"},
		{"case kept", "AbC", "AbC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, NormalizeKnowledgeContent(tc.in))
		})
	}
}

// 同一篇正文的不同书写方式必须算出同一个指纹，否则精确重复会漏判。
func TestKnowledgeContentHashStable(t *testing.T) {
	base := KnowledgeContentHash("第一段\n第二段")
	require.Len(t, base, 64)
	require.Equal(t, base, KnowledgeContentHash("\ufeff第一段\r\n第二段  \r\n"))
	require.NotEqual(t, base, KnowledgeContentHash("第一段\n第二段改过"))
}

// 空正文不参与比对：返回空串而不是空内容的哈希。
func TestKnowledgeContentHashEmpty(t *testing.T) {
	require.Empty(t, KnowledgeContentHash(""))
	require.Empty(t, KnowledgeContentHash(" \n\t\r\n"))
}
