package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestSanitizeDraftClampsOversizedSummary(t *testing.T) {
	draft := OrganizeDraft{
		Summary:  strings.Repeat("长", OrganizeSummaryMaxRunes+30),
		Keywords: []string{"React Server Components"},
	}
	clean, err := sanitizeDraft(draft)
	require.NoError(t, err)
	require.Equal(t, OrganizeSummaryMaxRunes, utf8.RuneCountInString(clean.Summary))
}

func TestSanitizeDraftKeepsLongEnglishKeyword(t *testing.T) {
	draft := OrganizeDraft{
		Summary: "一段足够长的摘要内容。",
		Keywords: []string{
			"React Server Components",
			strings.Repeat("词", OrganizeKeywordMaxRunes+1),
		},
	}
	clean, err := sanitizeDraft(draft)
	require.NoError(t, err)
	require.Equal(t, []string{"React Server Components"}, clean.Keywords)
}

func TestEncodeKeywordsRoundTripsLongKeywords(t *testing.T) {
	encoded := EncodeKeywords([]string{"React Server Components"})
	require.JSONEq(t, `["React Server Components"]`, encoded)
	require.Equal(t, []string{"React Server Components"}, ParseKeywords(encoded))
}
