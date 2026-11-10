package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// webFetcherFake 供知识导入测试替换真实抓取。
type webFetcherFake struct {
	document *WebDocument
	err      error
	calls    int
	lastURL  string
}

func (f *webFetcherFake) Fetch(_ context.Context, rawURL string) (*WebDocument, error) {
	f.calls++
	f.lastURL = rawURL
	return f.document, f.err
}

func TestWebFetcherExtractsArticle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>备用标题</title>` +
			`<meta property="og:title" content="示例文章标题"></head><body>` +
			`<nav>站点导航</nav><article><h1>标题一</h1><p>第一段 <a href="/detail/1">详情</a>。</p>` +
			`<ul><li>要点一</li><li>要点二</li></ul><pre><code>fmt.Println("hi")</code></pre></article>` +
			`<footer>页脚</footer><script>var x=1</script></body></html>`))
	}))
	defer server.Close()
	document, err := newWebFetcher(true).Fetch(context.Background(), server.URL+"/post")
	require.NoError(t, err)
	require.Equal(t, "示例文章标题", document.Title)
	require.Equal(t, server.URL+"/post", document.URL)
	require.Contains(t, document.Content, "# 标题一")
	require.Contains(t, document.Content, "[详情]("+server.URL+"/detail/1)")
	require.Contains(t, document.Content, "- 要点一")
	require.Contains(t, document.Content, "fmt.Println")
	require.NotContains(t, document.Content, "站点导航")
	require.NotContains(t, document.Content, "var x=1")
}

func TestWebFetcherFollowsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><title>最终页</title></head><body><main><p>跳转后的正文段落。</p></main></body></html>`))
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer server.Close()
	document, err := newWebFetcher(true).Fetch(context.Background(), server.URL+"/start")
	require.NoError(t, err)
	require.Equal(t, server.URL+"/final", document.URL)
	require.Equal(t, "最终页", document.Title)
}

func TestWebFetcherRejectsRedirectLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()
	_, err := newWebFetcher(true).Fetch(context.Background(), server.URL+"/loop")
	require.ErrorIs(t, err, ErrWebFetchFailed)
}

func TestWebFetcherBlocksPrivateAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><p>内网页面</p></body></html>`))
	}))
	defer server.Close()
	_, err := newWebFetcher(false).Fetch(context.Background(), server.URL)
	require.ErrorIs(t, err, ErrWebFetchBlocked)
}

func TestWebFetcherRejectsInvalidURLs(t *testing.T) {
	for _, raw := range []string{
		"",
		"ftp://example.com/article",
		"file:///etc/passwd",
		"http:///missing-host",
		"https://user:pass@example.com/article",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := newWebFetcher(true).Fetch(context.Background(), raw)
			require.ErrorIs(t, err, ErrWebURLInvalid)
		})
	}
}

func TestWebFetcherRejectsUnsupportedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4"))
		case "/forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "/script-only":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body><script>renderApp()</script></body></html>`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body><p>正常页面。</p></body></html>`))
		}
	}))
	defer server.Close()
	for _, path := range []string{"/pdf", "/forbidden", "/script-only"} {
		t.Run(path, func(t *testing.T) {
			_, err := newWebFetcher(true).Fetch(context.Background(), server.URL+path)
			require.ErrorIs(t, err, ErrWebFetchFailed)
		})
	}
}

func TestWebFetcherRejectsOversizePage(t *testing.T) {
	payload := `<html><body><p>` + strings.Repeat("a", webFetchMaxBytes) + `</p></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	_, err := newWebFetcher(true).Fetch(context.Background(), server.URL)
	require.ErrorIs(t, err, ErrWebFetchFailed)
}
