/**
 * 图谱编码：六类关系的四轴（线型 / 箭头 / 线宽 / 透明度）与深色岛配色。
 *
 * 为什么这里不写死色值：颜色全部来自 style.css 的 `--color-graph-*` 令牌。
 * ECharts 画布无法解析 var()，所以在这里把令牌解析成具体颜色再交给图表。
 *
 * 深色岛（任务单 #42）是一条**新造**的画布带，不是保留 —— 本仓历史里画布一直是纸色。
 * 岛上不能用纸世界的墨色：`--color-label` 是 `#181818`，在 `#16191c` 上只有 1.01:1，
 * 等于隐形。所以岛上文字单独一套 `--color-graph-island-label*`。
 */
import type { RelationType } from '../services/apis/knowledge'

/** 关系类型在图上出现的完整集合；顺序即图例顺序。 */
export const GRAPH_EDGE_TYPES: RelationType[] = [
  'related',
  'depends_on',
  'supplements',
  'contrasts',
  'updates',
  'contradicts',
]

/** ECharts 线型：`'solid'` 或 dash 数组（`'1 3'` 是 SVG 写法，ECharts 不认）。 */
export type GraphDash = 'solid' | number[]
/** 边两端的箭头：`['none','none']` / `['none','arrow']` / `['arrow','arrow']`。 */
export type GraphArrow = ['none' | 'arrow', 'none' | 'arrow']

export interface GraphEdgeStyle {
  type: GraphDash
  symbol: GraphArrow
  width: number
  opacity: number
  color: string
}

export interface GraphPalette {
  /** 深色岛底色 */
  island: string
  /** 岛上装饰线（图例分隔线） */
  islandRule: string
  /** 岛上主文字：节点之外的标题、图例正文 */
  label: string
  /** 岛上次要文字：边标签、图例辅助说明 */
  labelSecondary: string
  /** 节点填充（纸白：在深底上 15.23:1，也是节点文字的背景） */
  nodeFill: string
  /** 节点文字（深墨：在纸白上 15.33:1） */
  nodeInk: string
  /** 节点描边（承载来源类型，不承载语义色） */
  nodeBorder: string
  /** 选中环：必须同时在深底（4.78:1）与纸白节点（3.19:1）上达标 */
  selectRing: string
  /** AI 建议的边色 */
  edgeAI: string
  /** 手动建立的边色 */
  edgeManual: string
  /** 矛盾关系的边色：六类里唯一带彩色的 */
  edgeContradicts: string
  /** tooltip 底（复用纸面：深底上 15.23:1，不必另造一档） */
  tooltipBg: string
  /** tooltip 文字 */
  tooltipInk: string
}

/**
 * 六类关系的四轴编码（任务单 #42 AC①）。
 *
 * 四轴组合的**唯一性**是硬要求，单个取值不是：以后调 1.5→1.6 或 0.88→0.9
 * 都不该让测试变红，所以测试断言的是四元组指纹而不是具体数值。
 *
 * 透明度下限来自 DESIGN.md「证据边界」：`related` 在 0.55 下只有 2.00:1，
 * 验收口径是**实线不得低于 0.7**。这里实线最低是 depends_on 的 0.88×0.9=0.792。
 */
const EDGE_ENCODING: Record<RelationType, Omit<GraphEdgeStyle, 'color'>> = {
  // 相关：点线、无箭头、最细、最淡 —— 弱关系里最弱的一档
  related: { type: [1, 3], symbol: ['none', 'none'], width: 1, opacity: 0.55 },
  // 依赖：实线、单向、加粗、较亮 —— 方向性与强度都最强的蓝灰
  depends_on: { type: 'solid', symbol: ['none', 'arrow'], width: 1.5, opacity: 0.88 },
  // 补充：实线、单向、细、较淡 —— 与依赖只差线宽和透明度
  supplements: { type: 'solid', symbol: ['none', 'arrow'], width: 1, opacity: 0.8 },
  // 对照：点线、双向、细、中 —— 与相关只差箭头和透明度
  contrasts: { type: [1, 3], symbol: ['arrow', 'arrow'], width: 1, opacity: 0.65 },
  // 更新：虚线、单向、加粗 —— 与依赖只差线型
  updates: { type: [5, 3], symbol: ['none', 'arrow'], width: 1.5, opacity: 0.95 },
  // 矛盾：虚线、双向、最粗、最亮、唯一彩色
  contradicts: { type: [5, 3], symbol: ['arrow', 'arrow'], width: 2, opacity: 0.95 },
}

/** 建议边整体压暗一档；乘数不能把实线压到 0.7 以下。 */
const SUGGESTED_DIM = 0.9

/**
 * 某一类关系的四轴编码（不含颜色）。
 *
 * 图例要用它画样例线：图例和图表必须取自**同一份**编码表，
 * 否则以后调一次线宽，图例和图上就说的不是一回事了。
 */
export const graphEdgeEncoding = (type: RelationType) =>
  EDGE_ENCODING[type] ?? EDGE_ENCODING.related

/** 节点直径：基础 18px，度数每增加 1 加 4px，最多计入 12 度。 */
export const graphNodeSize = (degree: number) => 18 + Math.min(Math.max(degree, 0), 12) * 4

/**
 * 来源类型 → 节点描边样式。
 *
 * 为什么用描边而不是填充色：8 种 source_type 只有 4 个可见填充色
 * （manual/web/markdown/docx 曾全部同色），填充色这条路已经用到极限。
 * 描边样式只有三档，所以 8 种来源按**解析形态**归并 —— 这正是节点要表达的信息：
 * 人直接建的、从单份文档解析的、从表格导入的。
 */
const SOURCE_BORDER_TYPE: Record<string, 'solid' | 'dashed' | 'dotted'> = {
  manual: 'solid',
  web: 'solid',
  markdown: 'dashed',
  text: 'dashed',
  docx: 'dashed',
  pdf: 'dashed',
  csv: 'dotted',
  xlsx: 'dotted',
}

export const graphNodeBorderType = (sourceType: string) => SOURCE_BORDER_TYPE[sourceType] ?? 'solid'

/** 深色岛令牌 → GraphPalette 字段。顺序即解析顺序。 */
const PALETTE_TOKENS: Array<[keyof GraphPalette, string]> = [
  ['island', '--color-graph-island'],
  ['islandRule', '--color-graph-island-rule'],
  ['label', '--color-graph-island-label'],
  ['labelSecondary', '--color-graph-island-label-secondary'],
  ['nodeFill', '--color-surface'],
  ['nodeInk', '--color-label'],
  ['nodeBorder', '--color-graph-node-border'],
  ['selectRing', '--color-graph-select-ring'],
  ['edgeAI', '--color-graph-edge-ai'],
  ['edgeManual', '--color-graph-edge-manual'],
  ['edgeContradicts', '--color-graph-edge-contradicts'],
  ['tooltipBg', '--color-surface'],
  ['tooltipInk', '--color-label'],
]

function readToken(root: HTMLElement | null, token: string) {
  if (!root || typeof getComputedStyle !== 'function') return ''
  return getComputedStyle(root).getPropertyValue(token).trim()
}

/**
 * 从 CSS 变量解析深色岛调色板。
 *
 * 注意它读的是 `--color-graph-*` 与两个纸世界令牌（`--color-surface` / `--color-label`
 * 供节点填充与墨色用），**不再读 `--color-primary` / `--color-info` / `--color-danger`**
 * —— 那三个在深底上分别是 2.04 / 2.04 / 2.65:1，全都低于非文本 3:1 下限。
 */
export function resolveGraphPalette(element?: HTMLElement | null): GraphPalette {
  const root = element ?? (typeof document !== 'undefined' ? document.documentElement : null)
  const palette = {} as GraphPalette
  for (const [key, token] of PALETTE_TOKENS) palette[key] = readToken(root, token)
  return palette
}

/**
 * 一条边的完整样式：四轴编码 × 来源配色 × 状态压暗。
 *
 * 线色只按**来源**分（AI / 手动），不按关系类型分 —— 关系类型由四轴承载。
 * 唯一的例外是矛盾：它的彩色本身是语义（「这两条知识冲突」），
 * 不该被来源分色稀释，所以无视 source。
 */
export function graphEdgeStyle(
  palette: GraphPalette,
  edge: { relation_type: RelationType; source: string; status: string },
): GraphEdgeStyle {
  const encoding = EDGE_ENCODING[edge.relation_type] ?? EDGE_ENCODING.related
  const color =
    edge.relation_type === 'contradicts'
      ? palette.edgeContradicts
      : edge.source === 'manual'
        ? palette.edgeManual
        : palette.edgeAI
  return {
    ...encoding,
    color,
    opacity: edge.status === 'suggested' ? encoding.opacity * SUGGESTED_DIM : encoding.opacity,
  }
}
