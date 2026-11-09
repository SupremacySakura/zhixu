import MarkdownIt from 'markdown-it'

const createMarkdown = (breaks: boolean) => {
  const markdown = new MarkdownIt({ html: false, linkify: false, typographer: false, breaks })
  const defaultLink = markdown.renderer.rules.link_open
  markdown.renderer.rules.link_open = (tokens, index, options, env, self) => {
    const token = tokens[index]!
    const href = String(token.attrGet('href') || '')
    if (!/^https?:\/\//i.test(href) && !/^mailto:/i.test(href) && !href.startsWith('#'))
      token.attrSet('href', '#')
    token.attrSet('rel', 'noopener noreferrer')
    return defaultLink
      ? defaultLink(tokens, index, options, env, self)
      : self.renderToken(tokens, index, options)
  }
  // Imported documents must not load remote tracking images or authenticated local URLs.
  markdown.renderer.rules.image = (tokens, index) =>
    `<span class="markdown-image-note">[图片：${markdown.utils.escapeHtml(tokens[index]?.content || '未加载')}，暂不加载]</span>`
  return markdown
}

const markdown = createMarkdown(false)
// 纯文本知识（txt 提取）开启 breaks：单个换行渲染为 <br>，保留原始换行结构。
const plaintext = createMarkdown(true)

export const renderMarkdown = (content: string) => markdown.render(content)

// 知识正文统一入口：所有格式在后端已归一为 Markdown，仅纯文本需要保留换行的渲染差异。
export const renderKnowledgeContent = (content: string, sourceType: string) =>
  (sourceType === 'text' ? plaintext : markdown).render(content)
