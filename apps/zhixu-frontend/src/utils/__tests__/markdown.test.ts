import { describe, expect, it } from 'vitest'
import { renderKnowledgeContent, renderMarkdown } from '../markdown'
import { isMarkdownFile } from '../knowledge'
describe('Markdown knowledge', () => {
  it('renders headings, lists, tables, and code', () => {
    const html = renderMarkdown('# 标题\n\n- 正文\n\n```js\nalert(1)\n```\n\n|列|\n|---|\n|值|')
    expect(html).toContain('<h1>标题</h1>')
    expect(html).toContain('<li>正文</li>')
    expect(html).toContain('<table>')
    expect(html).toContain('<pre>')
  })
  it('does not execute HTML or load images', () => {
    const html = renderMarkdown(
      '<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n![图](https://example.com/track)',
    )
    expect(html).not.toContain('<script>')
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;script&gt;')
    expect(html).toContain('暂不加载')
  })
  it('rejects unsafe URLs and local authenticated links', () => {
    const html = renderMarkdown(
      '[bad](javascript:alert(1)) [file](/api/file/delete) [good](https://example.com)',
    )
    expect(html).not.toContain('href="javascript:')
    expect(html).not.toContain('href="/api/')
    expect(html).toContain('href="https://example.com"')
  })
  it('only accepts Markdown files, not folders', () => {
    expect(isMarkdownFile({ type: 'file', name: 'note.MD' })).toBe(true)
    expect(isMarkdownFile({ type: 'file', name: 'note.markdown' })).toBe(true)
    expect(isMarkdownFile({ type: 'folder', name: 'note.md' })).toBe(false)
    expect(isMarkdownFile({ type: 'file', name: 'note.pdf' })).toBe(false)
  })
  it('keeps line breaks for extracted plain text, not for markdown', () => {
    expect(renderKnowledgeContent('第一行\n第二行', 'text')).toContain('<br')
    expect(renderKnowledgeContent('第一行\n第二行', 'markdown')).not.toContain('<br')
    expect(renderKnowledgeContent('第一行\n第二行', '')).not.toContain('<br')
  })
})
