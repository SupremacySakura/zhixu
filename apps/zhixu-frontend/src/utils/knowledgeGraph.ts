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
  /** 节点外框（墨色，不承载语义色） */
  nodeBorder: string
  /** 书本节点的图书馆蓝书脊 */
  nodeSpine: string
  /** 书脊上的纸白装订线 */
  nodeStitch: string
  /** 右侧页块：比封面暗一档，让「封面 + 书页」的层次读得出来 */
  nodePage: string
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

/**
 * 书本节点的尺寸：高度基础 24px，度数每增加 1 加 3px，最多计入 12 度。
 * 宽度按 3:4 封面取整；返回 `[width, height]`，因为 ECharts 的 symbolSize
 * 只有在数组形态下才能画出纵向的书。
 */
export function graphNodeSize(degree: number): [number, number] {
  const height = 24 + Math.min(Math.max(degree, 0), 12) * 3
  return [Math.round(height * 0.75), height]
}

/** 书脊装订：实心 / 短缝线 / 点线。 */
export type GraphNodeBinding = 'solid' | 'dashed' | 'dotted'

/**
 * 来源类型 → 书脊装订样式。
 *
 * 8 种 source_type 按**解析形态**归并成三组：人直接建的、从单份文档解析的、
 * 从表格导入的。编码从旧的「外框线型」迁到书脊，外框因此可以恒为实线，
 * 书看起来不再像一张打了孔的优惠券。
 */
const SOURCE_BINDING: Record<string, GraphNodeBinding> = {
  manual: 'solid',
  web: 'solid',
  markdown: 'dashed',
  text: 'dashed',
  docx: 'dashed',
  pdf: 'dashed',
  csv: 'dotted',
  xlsx: 'dotted',
}

export const graphNodeBinding = (sourceType: string): GraphNodeBinding =>
  SOURCE_BINDING[sourceType] ?? 'solid'

export interface GraphNodeSymbolOptions {
  width: number
  height: number
  sourceType: string
  /** 局部图谱的中心节点：加一条蓝色书签带。 */
  center: boolean
  /** 当前选中节点：在封面内侧画 3px 蓝色选中环。 */
  selected: boolean
  palette: GraphPalette
}

const XML_ESCAPES: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&apos;',
}

const escapeXml = (value: string) => value.replace(/[&<>"']/g, char => XML_ESCAPES[char] ?? char)

/**
 * 书本节点的原始 SVG（75×100 视图盒）。
 *
 * 为什么不用单个 `path://`：ECharts 的 path symbol 只有一套填充/描边，
 * 画不出「纸白封面 + 蓝书脊 + 纸白缝线」三种颜色。SVG 符号可以保留多色，
 * 代价是选中环也得画进符号里（见 KnowledgeGraphChart 的选中变体逻辑）。
 *
 * 所有线宽都按屏幕像素换算回视图盒单位，保证 24px 的小书和 60px 的大书
 * 外框都是 2px、内部线都是 1px，而不是随尺寸一起放大。
 */
export function graphNodeSvg({
  width,
  height,
  sourceType,
  center,
  selected,
  palette,
}: GraphNodeSymbolOptions): string {
  const unit = (75 / width + 100 / height) / 2
  const outer = 2 * unit
  const hairline = unit
  const ringStroke = 3 * unit
  const inset = outer / 2
  const spineRight = 13.5
  const coverRight = 67
  const coverBottom = 92
  const binding = graphNodeBinding(sourceType)
  const stitchDash =
    binding === 'dashed'
      ? `${3 * unit} ${3 * unit}`
      : binding === 'dotted'
        ? `${unit} ${2 * unit}`
        : undefined
  const stitch =
    binding === 'solid'
      ? ''
      : `<path d="M6.75 28 V88" fill="none" stroke="${escapeXml(palette.nodeStitch)}" stroke-width="${hairline}" stroke-dasharray="${stitchDash}" stroke-linecap="${binding === 'dotted' ? 'round' : 'butt'}"/>`
  const pageLine =
    height >= 42
      ? `<path d="M70 6 V96 M73 6 V96" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}" stroke-opacity="0.7"/>`
      : `<path d="M71.5 6 V96" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}" stroke-opacity="0.7"/>`
  const titleLines =
    height >= 30
      ? `<path d="M20 28 H44 M20 38 H38" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}" stroke-opacity="0.45"/>`
      : ''
  const bookmark = center
    ? `<path d="M58 0 H64 V32 L61 28 L58 32 Z" fill="${escapeXml(palette.nodeSpine)}" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}"/>`
    : ''
  const spineHeadband = `<path d="M0 3 H${spineRight}" fill="none" stroke="${escapeXml(palette.nodeStitch)}" stroke-width="${hairline}"/>`
  const spineTailband = `<path d="M0 ${coverBottom - 3} H${spineRight}" fill="none" stroke="${escapeXml(palette.nodeStitch)}" stroke-width="${hairline}"/>`
  const spineLabel = `<rect x="3.5" y="12" width="6.5" height="10" fill="${escapeXml(palette.nodeStitch)}"/>`
  const ringInset = outer + unit + ringStroke / 2
  const ring = selected
    ? `<rect x="${ringInset}" y="${ringInset}" width="${75 - ringInset * 2}" height="${100 - ringInset * 2}" fill="none" stroke="${escapeXml(palette.selectRing)}" stroke-width="${ringStroke}"/>`
    : ''

  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 75 100" preserveAspectRatio="none">` +
    // 页块先画，封面盖住它的左/上部分；右侧与底部留出 5 个单位的书页厚度。
    `<rect x="0" y="0" width="75" height="100" fill="${escapeXml(palette.nodePage)}"/>` +
    `<rect x="${hairline / 2}" y="${hairline / 2}" width="${75 - hairline}" height="${100 - hairline}" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}" stroke-opacity="0.7"/>` +
    `<rect x="0" y="0" width="${coverRight}" height="${coverBottom}" fill="${escapeXml(palette.nodeFill)}"/>` +
    `<rect x="0" y="0" width="${spineRight}" height="${coverBottom}" fill="${escapeXml(palette.nodeSpine)}"/>` +
    `<path d="M${spineRight} 0 V${coverBottom}" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${hairline}" stroke-opacity="0.55"/>` +
    spineHeadband +
    spineTailband +
    spineLabel +
    stitch +
    pageLine +
    titleLines +
    bookmark +
    `<rect x="${inset}" y="${inset}" width="${coverRight - outer}" height="${coverBottom - outer}" fill="none" stroke="${escapeXml(palette.nodeBorder)}" stroke-width="${outer}"/>` +
    ring +
    `</svg>`
  )
}

const symbolCache = new Map<string, string>()

/** 书本节点的 SVG data URI；按尺寸、来源、中心/选中态与调色板缓存。 */
export function graphNodeSymbol(options: GraphNodeSymbolOptions): string {
  const key = [
    options.width,
    options.height,
    options.sourceType,
    options.center ? 1 : 0,
    options.selected ? 1 : 0,
    options.palette.nodeFill,
    options.palette.nodeBorder,
    options.palette.nodeSpine,
    options.palette.nodeStitch,
    options.palette.nodePage,
    options.palette.selectRing,
  ].join('|')
  const cached = symbolCache.get(key)
  if (cached) return cached
  if (symbolCache.size > 512) symbolCache.clear()
  // ECharts 只认 `image://` 前缀；少了它会把符号当默认 rect，节点退化成蓝方块。
  const symbol = `image://data:image/svg+xml;charset=UTF-8,${encodeURIComponent(graphNodeSvg(options))}`
  symbolCache.set(key, symbol)
  return symbol
}

/** 深色岛令牌 → GraphPalette 字段。顺序即解析顺序。 */
const PALETTE_TOKENS: Array<[keyof GraphPalette, string]> = [
  ['island', '--color-graph-island'],
  ['islandRule', '--color-graph-island-rule'],
  ['label', '--color-graph-island-label'],
  ['labelSecondary', '--color-graph-island-label-secondary'],
  ['nodeFill', '--color-surface'],
  ['nodeInk', '--color-label'],
  ['nodeBorder', '--color-graph-node-border'],
  ['nodeSpine', '--color-primary'],
  ['nodeStitch', '--color-surface-tertiary'],
  ['nodePage', '--color-surface-secondary'],
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
 * 注意它读的是 `--color-graph-*` 与三个纸世界令牌：`--color-surface` / `--color-label`
 * 供节点填充与墨色用，`--color-primary` 只用于**纸白节点上的书脊**（6.39:1）。
 * `--color-primary` / `--color-info` / `--color-danger` 在深底上分别是
 * 2.04 / 2.04 / 2.65:1，仍不得直接拿来画岛上的文字或边。
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
