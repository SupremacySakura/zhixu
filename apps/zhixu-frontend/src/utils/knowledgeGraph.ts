/**
 * 图谱颜色全部从 style.css 的 @theme CSS 变量读取，组件不写死色值。
 * ECharts 画布无法解析 var()，因此在这里把令牌解析为具体颜色再交给图表。
 */
export interface GraphPalette {
  source: Record<string, string>
  edgeAI: string
  edgeManual: string
  /** 矛盾关系的线色：类型本身需要一眼可辨，优先于来源分色。 */
  edgeContradicts: string
  label: string
  labelSecondary: string
  surface: string
  hairline: string
  primary?: string
}

/** 来源类型 → CSS 变量令牌；未知类型走中性文本色。 */
export const GRAPH_SOURCE_TOKENS: Record<string, string> = {
  manual: '--color-primary',
  web: '--color-info',
  markdown: '--color-purple',
  text: '--color-teal',
  pdf: '--color-danger',
  docx: '--color-info',
  csv: '--color-warning',
  xlsx: '--color-teal',
  default: '--color-label-secondary',
}

/** 节点直径：基础 18px，度数每增加 1 加 4px，最多计入 12 度。 */
export const graphNodeSize = (degree: number) => 18 + Math.min(Math.max(degree, 0), 12) * 4

function readToken(root: HTMLElement | null, token: string) {
  if (!root || typeof getComputedStyle !== 'function') return ''
  return getComputedStyle(root).getPropertyValue(token).trim()
}

export function resolveGraphPalette(element?: HTMLElement | null): GraphPalette {
  const root = element ?? (typeof document !== 'undefined' ? document.documentElement : null)
  const source: Record<string, string> = {}
  for (const [type, token] of Object.entries(GRAPH_SOURCE_TOKENS)) {
    source[type] = readToken(root, token)
  }
  return {
    source,
    edgeAI: readToken(root, '--color-info'),
    edgeManual: readToken(root, '--color-label-tertiary'),
    edgeContradicts: readToken(root, '--color-danger'),
    label: readToken(root, '--color-label'),
    labelSecondary: readToken(root, '--color-label-secondary'),
    surface: readToken(root, '--color-surface'),
    hairline: readToken(root, '--color-hairline'),
    primary: readToken(root, '--color-primary'),
  }
}

/**
 * 关系线色：矛盾关系单独用矛盾色（类型比来源更需要一眼可辨），
 * 其余类型按来源区分——AI 建议与手动建立使用不同线色。
 */
export const graphEdgeColor = (palette: GraphPalette, source: string, relationType?: string) => {
  if (relationType === 'contradicts') return palette.edgeContradicts
  return source === 'manual' ? palette.edgeManual : palette.edgeAI
}
