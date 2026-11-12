/** 高亮切段：hit 为 true 的段是两侧证据共有的片段，需要标出来。 */
export interface QuoteSegment {
  text: string
  hit: boolean
}

/** 少于两个字的重合只是偶发字符，标出来反而干扰阅读。 */
const MIN_PHRASE_LENGTH = 2

/**
 * 求两段引文的最长公共片段。
 *
 * 重复与冲突提醒的两侧证据都指向同一段正文，把共有片段标出来，
 * 用户一眼就能看出「哪里撞了」，不需要自己逐字比对。
 */
export function longestCommonPhrase(left: string, right: string): string {
  if (!left || !right) return ''
  let best = ''
  for (let start = 0; start < left.length; start += 1) {
    for (let end = start + best.length + 1; end <= left.length; end += 1) {
      const candidate = left.slice(start, end)
      if (right.includes(candidate)) best = candidate
      else break
    }
  }
  return best.length >= MIN_PHRASE_LENGTH ? best : ''
}

/** 按共有片段把引文切成普通段与命中段，供模板渲染 <mark>。 */
export function highlightQuote(quote: string, phrase: string): QuoteSegment[] {
  if (!quote) return []
  if (!phrase) return [{ text: quote, hit: false }]
  const segments: QuoteSegment[] = []
  let cursor = 0
  while (cursor < quote.length) {
    const index = quote.indexOf(phrase, cursor)
    if (index < 0) {
      segments.push({ text: quote.slice(cursor), hit: false })
      break
    }
    if (index > cursor) segments.push({ text: quote.slice(cursor, index), hit: false })
    segments.push({ text: phrase, hit: true })
    cursor = index + phrase.length
  }
  return segments
}
