package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

var (
	// ErrWebURLInvalid 表示链接格式不合法或不是 http/https。
	ErrWebURLInvalid = errors.New("网页链接无效")
	// ErrWebFetchBlocked 表示链接指向受限地址（内网、回环等）或域名解析失败。
	ErrWebFetchBlocked = errors.New("网页链接不可访问")
	// ErrWebFetchFailed 表示抓取或正文提取失败。
	ErrWebFetchFailed = errors.New("网页内容获取失败")
)

const (
	webFetchTimeout      = 30 * time.Second
	webFetchMaxBytes     = 10 << 20
	webFetchMaxRedirects = 5
	webFetchMaxURLBytes  = 2048
	webFetchUserAgent    = "zhixu-knowledge-bot/1.0"
)

// WebDocument 是一次网页抓取的结果：标题与转成 Markdown 的正文。
type WebDocument struct {
	URL     string
	Title   string
	Content string
}

// WebFetcher 抓取网页并提取正文；测试可注入替身。
type WebFetcher interface {
	Fetch(ctx context.Context, rawURL string) (*WebDocument, error)
}

type webFetcher struct {
	client       *http.Client
	allowPrivate bool
}

// NewWebFetcher 创建带 SSRF 防护的网页抓取器：仅 http/https，禁止内网/回环地址，最多 5 次重定向。
func NewWebFetcher() WebFetcher {
	return newWebFetcher(false)
}

func newWebFetcher(allowPrivate bool) *webFetcher {
	f := &webFetcher{allowPrivate: allowPrivate}
	// 不走环境代理：代理会绕过本机的地址校验，也让抓取行为不可预期。
	transport := &http.Transport{
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	transport.DialContext = f.dialContext
	f.client = &http.Client{
		Transport: transport,
		Timeout:   webFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= webFetchMaxRedirects {
				return fmt.Errorf("%w: 重定向次数超过 %d 次", ErrWebFetchFailed, webFetchMaxRedirects)
			}
			if _, err := validateWebURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	return f
}

// dialContext 在连接前解析域名并拒绝受限地址，避免 DNS 重绑定绕过校验。
func (f *webFetcher) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if f.allowPrivate {
		return dialer.DialContext(ctx, network, addr)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%w: 链接地址无效", ErrWebFetchBlocked)
	}
	if ip := net.ParseIP(host); ip != nil {
		if blockedIP(ip) {
			return nil, fmt.Errorf("%w: 不允许访问内网地址", ErrWebFetchBlocked)
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法解析网页域名", ErrWebFetchBlocked)
	}
	for _, resolved := range ips {
		if blockedIP(resolved.IP) {
			continue
		}
		if conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port)); err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("%w: 不允许访问内网地址", ErrWebFetchBlocked)
}

// blockedIP 判断是否为内网、回环、链路本地或保留地址。
func blockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 0.0.0.0/8、100.64.0.0/10（运营商级 NAT）、198.18.0.0/15（基准测试保留段）。
		switch {
		case ip4[0] == 0:
			return true
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127:
			return true
		case ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19):
			return true
		}
	}
	return false
}

// validateWebURL 校验链接格式：仅 http/https、有主机名、不含用户凭据。
func validateWebURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > webFetchMaxURLBytes {
		return nil, fmt.Errorf("%w: 请填写有效网页链接（不超过 %d 字符）", ErrWebURLInvalid, webFetchMaxURLBytes)
	}
	target, err := url.Parse(raw)
	if err != nil || target.Host == "" || target.Hostname() == "" {
		return nil, fmt.Errorf("%w: 请填写完整网址，例如 https://example.com/article", ErrWebURLInvalid)
	}
	if scheme := strings.ToLower(target.Scheme); scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%w: 仅支持 http/https 链接", ErrWebURLInvalid)
	}
	if target.User != nil {
		return nil, fmt.Errorf("%w: 链接不能包含用户名或密码", ErrWebURLInvalid)
	}
	return target, nil
}

// Fetch 抓取网页并提取正文；返回的 URL 为最终地址（含重定向后）。
func (f *webFetcher) Fetch(ctx context.Context, rawURL string) (*WebDocument, error) {
	target, err := validateWebURL(rawURL)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, webFetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: 请填写有效网页链接", ErrWebURLInvalid)
	}
	request.Header.Set("User-Agent", webFetchUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, webFetchError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: 网页返回状态码 %d", ErrWebFetchFailed, response.StatusCode)
	}
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentType != "text/html" && contentType != "application/xhtml+xml" {
		return nil, fmt.Errorf("%w: 仅支持 HTML 网页", ErrWebFetchFailed)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, webFetchMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: 读取网页内容失败", ErrWebFetchFailed)
	}
	if len(body) > webFetchMaxBytes {
		return nil, fmt.Errorf("%w: 网页内容超过 10MB 限制", ErrWebFetchFailed)
	}
	title, content, err := extractWebContent(bytes.NewReader(body), response.Request.URL)
	if err != nil {
		return nil, err
	}
	return &WebDocument{URL: response.Request.URL.String(), Title: title, Content: content}, nil
}

// webFetchError 把驱动层错误归类为受限或抓取失败。
func webFetchError(err error) error {
	for _, sentinel := range []error{ErrWebURLInvalid, ErrWebFetchBlocked, ErrWebFetchFailed} {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return fmt.Errorf("%w: 无法访问该网页，请稍后重试", ErrWebFetchFailed)
}

// webChromeTags 是 body 级正文容器被选中时一并丢弃的页面框架节点。
var webChromeTags = map[string]bool{
	"nav": true, "header": true, "footer": true, "aside": true,
}

// 需要整段丢弃的装饰性节点。
var webDropTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "iframe": true, "svg": true,
	"form": true, "button": true, "input": true, "select": true, "textarea": true,
	"canvas": true, "video": true, "audio": true,
}

// extractWebContent 解析 HTML：优先 og:title/title，正文优先 article/main，转成 Markdown。
func extractWebContent(reader io.Reader, base *url.URL) (string, string, error) {
	document, err := html.Parse(reader)
	if err != nil {
		return "", "", fmt.Errorf("%w: 网页内容解析失败", ErrWebFetchFailed)
	}
	title := findWebTitle(document)
	root := findContentRoot(document)
	removeWebNodes(root, webDropTags)
	if root.Data == "body" {
		removeWebNodes(root, webChromeTags)
	}
	content := renderMarkdown(root, base)
	content = normalizeMarkdown(content)
	if strings.TrimSpace(content) == "" {
		return "", "", fmt.Errorf("%w: 未能提取到网页正文，可能由脚本动态生成", ErrWebFetchFailed)
	}
	return title, content, nil
}

func findWebTitle(document *html.Node) string {
	var title, ogTitle string
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch node.Data {
		case "title":
			if title == "" {
				title = strings.TrimSpace(plainText(node))
			}
		case "meta":
			if ogTitle == "" && attrValue(node, "property") == "og:title" {
				ogTitle = strings.TrimSpace(attrValue(node, "content"))
			}
		}
	})
	if ogTitle != "" {
		return ogTitle
	}
	return title
}

// findContentRoot 选择正文容器：article/main 文本量足够时优先，否则回退 body。
func findContentRoot(document *html.Node) *html.Node {
	var body, main, article *html.Node
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch node.Data {
		case "body":
			if body == nil {
				body = node
			}
		case "main":
			if main == nil {
				main = node
			}
		case "article":
			if article == nil {
				article = node
			}
		}
	})
	for _, candidate := range []*html.Node{article, main} {
		if candidate != nil && textLength(candidate) >= 500 {
			return candidate
		}
	}
	if body != nil {
		return body
	}
	return document
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func attrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func textLength(node *html.Node) int {
	total := 0
	walkHTML(node, func(current *html.Node) {
		if current.Type == html.TextNode {
			total += utf8.RuneCountInString(current.Data)
		}
	})
	return total
}

func plainText(node *html.Node) string {
	var builder strings.Builder
	walkHTML(node, func(current *html.Node) {
		switch current.Type {
		case html.TextNode:
			builder.WriteString(current.Data)
		case html.ElementNode:
			if webDropTags[current.Data] {
				return
			}
			if current.Data == "br" || current.Data == "p" || strings.HasPrefix(current.Data, "h") {
				builder.WriteByte('\n')
			}
		}
	})
	return builder.String()
}

func removeWebNodes(root *html.Node, tags map[string]bool) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && tags[child.Data] {
			root.RemoveChild(child)
		} else {
			removeWebNodes(child, tags)
		}
		child = next
	}
}

// renderMarkdown 把正文节点转成 Markdown（标题/段落/列表/代码/引用/链接/图片）。
func renderMarkdown(root *html.Node, base *url.URL) string {
	writer := &webMarkdownWriter{base: base}
	writer.renderBlock(root, 0)
	return writer.builder.String()
}

type webMarkdownWriter struct {
	builder strings.Builder
	base    *url.URL
}

func (w *webMarkdownWriter) blockSeparator() {
	text := w.builder.String()
	if text == "" || strings.HasSuffix(text, "\n\n") {
		return
	}
	if strings.HasSuffix(text, "\n") {
		w.builder.WriteByte('\n')
		return
	}
	w.builder.WriteString("\n\n")
}

func (w *webMarkdownWriter) renderBlock(node *html.Node, depth int) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		w.renderNode(child, depth)
	}
}

func (w *webMarkdownWriter) renderNode(node *html.Node, depth int) {
	if node.Type == html.TextNode {
		w.writeInlineText(node.Data)
		return
	}
	if node.Type != html.ElementNode {
		return
	}
	tag := node.Data
	if webDropTags[tag] {
		return
	}
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		w.blockSeparator()
		w.builder.WriteString(strings.Repeat("#", level) + " ")
		w.builder.WriteString(strings.TrimSpace(plainText(node)))
		w.builder.WriteString("\n\n")
	case "p":
		w.blockSeparator()
		w.renderInline(node)
		w.builder.WriteString("\n\n")
	case "br":
		w.builder.WriteByte('\n')
	case "hr":
		w.blockSeparator()
		w.builder.WriteString("---\n\n")
	case "ul", "ol":
		w.blockSeparator()
		w.renderList(node, depth, tag == "ol")
		w.builder.WriteString("\n")
	case "li":
		w.builder.WriteString("- ")
		w.renderInline(node)
		w.builder.WriteByte('\n')
	case "pre":
		w.blockSeparator()
		w.builder.WriteString("```\n")
		w.builder.WriteString(strings.TrimRight(plainText(node), "\n"))
		w.builder.WriteString("\n```\n\n")
	case "code":
		w.builder.WriteByte('`')
		w.builder.WriteString(plainText(node))
		w.builder.WriteByte('`')
	case "blockquote":
		w.blockSeparator()
		quoted := normalizeMarkdown(renderMarkdown(node, w.base))
		for _, line := range strings.Split(strings.TrimSpace(quoted), "\n") {
			w.builder.WriteString("> " + line + "\n")
		}
		w.builder.WriteString("\n")
	case "a":
		href := resolveWebLink(w.base, attrValue(node, "href"))
		text := strings.TrimSpace(plainText(node))
		if href != "" && text != "" {
			w.builder.WriteString("[" + text + "](" + href + ")")
		} else if text != "" {
			w.builder.WriteString(text)
		}
	case "img":
		src := resolveWebLink(w.base, attrValue(node, "src"))
		alt := strings.TrimSpace(attrValue(node, "alt"))
		if src != "" {
			w.blockSeparator()
			w.builder.WriteString("![" + alt + "](" + src + ")\n\n")
		}
	case "strong", "b":
		w.builder.WriteString("**")
		w.renderInline(node)
		w.builder.WriteString("**")
	case "em", "i":
		w.builder.WriteString("*")
		w.renderInline(node)
		w.builder.WriteString("*")
	case "table":
		w.blockSeparator()
		w.renderBlock(node, depth)
		w.builder.WriteString("\n")
	case "tr":
		w.builder.WriteString("|")
		for cell := node.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type == html.ElementNode && (cell.Data == "td" || cell.Data == "th") {
				w.builder.WriteString(" " + strings.TrimSpace(plainText(cell)) + " |")
			}
		}
		w.builder.WriteByte('\n')
	case "div", "section", "article", "main", "body", "html", "header", "footer", "nav", "aside":
		w.renderBlock(node, depth)
		w.builder.WriteString("\n")
	default:
		w.renderBlock(node, depth)
	}
}

func (w *webMarkdownWriter) renderInline(node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		w.renderNode(child, 0)
	}
}

func (w *webMarkdownWriter) renderList(node *html.Node, depth int, ordered bool) {
	index := 1
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.Data != "li" {
			continue
		}
		indent := strings.Repeat("  ", depth)
		marker := "-"
		if ordered {
			marker = fmt.Sprintf("%d.", index)
			index++
		}
		w.builder.WriteString(indent + marker + " ")
		w.renderListItem(child, depth)
		w.builder.WriteByte('\n')
	}
}

// renderListItem 渲染列表项本身，嵌套列表换行缩进后递归。
func (w *webMarkdownWriter) renderListItem(node *html.Node, depth int) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && (child.Data == "ul" || child.Data == "ol") {
			w.builder.WriteByte('\n')
			w.renderList(child, depth+1, child.Data == "ol")
			continue
		}
		w.renderNode(child, depth)
	}
}

func (w *webMarkdownWriter) writeInlineText(text string) {
	collapsed := strings.Join(strings.Fields(text), " ")
	if collapsed == "" {
		if len(text) > 0 && !strings.HasSuffix(w.builder.String(), "\n") {
			w.builder.WriteByte(' ')
		}
		return
	}
	w.builder.WriteString(collapsed)
}

// resolveWebLink 把相对链接解析为绝对地址；非法或不支持的链接返回空。
func resolveWebLink(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "javascript:") || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return ""
	}
	reference, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if base != nil {
		reference = base.ResolveReference(reference)
	}
	if reference.Scheme != "http" && reference.Scheme != "https" {
		return ""
	}
	return reference.String()
}

// normalizeMarkdown 去掉行尾空白并压缩多余空行，保证输出的 Markdown 紧凑可读。
func normalizeMarkdown(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed = append(trimmed, strings.TrimRight(line, " \t"))
	}
	result := strings.Join(trimmed, "\n")
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result)
}
