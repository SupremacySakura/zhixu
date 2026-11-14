/**
 * 图谱编码契约（任务单 #42）
 *
 * 这一组测的是**六类关系在图上是否真的彼此可辨**，不是 ECharts option 长什么样。
 * 判据取自票面 AC①：线型 + 箭头 + 线宽 + 透明度四轴，六类两两不等；
 * AC②：只有「矛盾」带彩色，其余全是深浅不同的蓝灰。
 *
 * 为什么断言「四元组」而不是逐个字段：
 * 逐个字段会把测试钉死在某一套具体取值上（线宽 1.5 还是 1.6、透明度 0.8 还是 0.85），
 * 以后调参就全红；四元组只要求「六类互不相同」，参数怎么改都成立。
 * 这正是规格 #35「测试决策」禁止断言实现细节的意思。
 */
import { describe, expect, it } from 'vitest'
import type { RelationType } from '../../services/apis/knowledge'
import {
  GRAPH_EDGE_TYPES,
  graphEdgeStyle,
  graphNodeBinding,
  graphNodeSize,
  graphNodeSvg,
  graphNodeSymbol,
  resolveGraphPalette,
  type GraphPalette,
} from '../knowledgeGraph'

const palette: GraphPalette = {
  island: '#16191c',
  islandRule: '#3a424b',
  label: '#a8b0b8',
  labelSecondary: '#8b939c',
  nodeFill: '#f2eee4',
  nodeInk: '#181818',
  nodeBorder: '#605949',
  nodeSpine: '#1f4e79',
  nodeStitch: '#fbf8ef',
  nodePage: '#e2ddd0',
  selectRing: '#4a8ac0',
  edgeAI: '#b2cae1',
  edgeManual: '#c3c8cb',
  edgeContradicts: '#d16559',
  tooltipBg: '#f2eee4',
  tooltipInk: '#181818',
}

/** 四轴指纹：线型 / 箭头 / 线宽 / 透明度。 */
const fingerprint = (type: RelationType) => {
  const style = graphEdgeStyle(palette, { relation_type: type, source: 'ai', status: 'confirmed' })
  return JSON.stringify([style.type, style.symbol, style.width, style.opacity])
}

describe('六类关系的四轴编码', () => {
  it('六种关系两两可辨：四轴指纹全部不同', () => {
    const prints = GRAPH_EDGE_TYPES.map(fingerprint)
    expect(new Set(prints).size).toBe(GRAPH_EDGE_TYPES.length)
  })

  it('线型用数值数组表达点线与虚线（ECharts 不认 SVG 的 "1 3" 写法）', () => {
    const dotted = graphEdgeStyle(palette, {
      relation_type: 'related',
      source: 'ai',
      status: 'confirmed',
    })
    const dashed = graphEdgeStyle(palette, {
      relation_type: 'updates',
      source: 'ai',
      status: 'confirmed',
    })
    expect(Array.isArray(dotted.type)).toBe(true)
    expect(Array.isArray(dashed.type)).toBe(true)
    // 点线的实线段比虚线短
    expect((dotted.type as number[])[0]).toBeLessThan((dashed.type as number[])[0])
    // 六类里每一种的线型要么是 solid，要么是纯数字数组（不出现 '1 3' 这种字符串）
    for (const type of GRAPH_EDGE_TYPES) {
      const { type: dash } = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'ai',
        status: 'confirmed',
      })
      if (dash === 'solid') continue
      expect(Array.isArray(dash)).toBe(true)
      expect((dash as number[]).every(value => typeof value === 'number')).toBe(true)
    }
  })

  it('箭头逐边表达：相关无箭头、对照与矛盾双向、其余单向', () => {
    const arrow = (type: RelationType) =>
      graphEdgeStyle(palette, { relation_type: type, source: 'ai', status: 'confirmed' }).symbol
    expect(arrow('related')).toEqual(['none', 'none'])
    expect(arrow('contrasts')).toEqual(['arrow', 'arrow'])
    expect(arrow('contradicts')).toEqual(['arrow', 'arrow'])
    for (const type of ['depends_on', 'supplements', 'updates'] as const) {
      expect(arrow(type)).toEqual(['none', 'arrow'])
    }
  })

  it('只有矛盾带彩色，其余五类都是蓝灰（低彩度）', () => {
    const chroma = (hex: string) => {
      const [r, g, b] = hex
        .replace('#', '')
        .match(/../g)!
        .map(part => parseInt(part, 16))
      const max = Math.max(r!, g!, b!)
      const min = Math.min(r!, g!, b!)
      return (max - min) / Math.max(max, 1)
    }
    const contradicts = graphEdgeStyle(palette, {
      relation_type: 'contradicts',
      source: 'ai',
      status: 'confirmed',
    })
    expect(chroma(contradicts.color)).toBeGreaterThan(0.4)
    for (const type of ['related', 'depends_on', 'supplements', 'contrasts', 'updates'] as const) {
      const style = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'ai',
        status: 'confirmed',
      })
      expect(chroma(style.color)).toBeLessThan(0.3)
    }
  })

  it('AI 与手动来源用不同色，但同一关系类型下四轴不变', () => {
    // 矛盾除外：它的彩色本身是语义（「这两条知识冲突」），不该被来源分色稀释。
    for (const type of GRAPH_EDGE_TYPES.filter(item => item !== 'contradicts')) {
      const ai = graphEdgeStyle(palette, { relation_type: type, source: 'ai', status: 'confirmed' })
      const manual = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'manual',
        status: 'confirmed',
      })
      expect(ai.color).not.toBe(manual.color)
      expect(ai.type).toEqual(manual.type)
      expect(ai.symbol).toEqual(manual.symbol)
      expect(ai.width).toBe(manual.width)
      expect(ai.opacity).toBe(manual.opacity)
    }
  })

  it('矛盾边的颜色无视来源：AI 建议与手动建立都是同一个彩色', () => {
    const ai = graphEdgeStyle(palette, {
      relation_type: 'contradicts',
      source: 'ai',
      status: 'confirmed',
    })
    const manual = graphEdgeStyle(palette, {
      relation_type: 'contradicts',
      source: 'manual',
      status: 'confirmed',
    })
    expect(ai.color).toBe(palette.edgeContradicts)
    expect(manual.color).toBe(palette.edgeContradicts)
  })

  it('建议边比已确认边淡，但关系类型本身的编码不变', () => {
    for (const type of GRAPH_EDGE_TYPES) {
      const confirmed = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'ai',
        status: 'confirmed',
      })
      const suggested = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'ai',
        status: 'suggested',
      })
      expect(suggested.opacity).toBeLessThan(confirmed.opacity)
      expect(suggested.type).toEqual(confirmed.type)
      expect(suggested.symbol).toEqual(confirmed.symbol)
    }
  })

  it('实线在建议态下的有效透明度不低于 0.7（对比度验收口径）', () => {
    for (const type of GRAPH_EDGE_TYPES) {
      const style = graphEdgeStyle(palette, {
        relation_type: type,
        source: 'ai',
        status: 'suggested',
      })
      if (style.type !== 'solid') continue
      expect(style.opacity).toBeGreaterThanOrEqual(0.7)
    }
  })
})

describe('节点', () => {
  it('书本尺寸按度数递增：度数 0 是 18×24、度数 2 是 23×30', () => {
    expect(graphNodeSize(0)).toEqual([18, 24])
    expect(graphNodeSize(2)).toEqual([23, 30])
    // 度数封顶 12，避免超级节点吃掉整张画布
    expect(graphNodeSize(99)).toEqual(graphNodeSize(12))
  })

  it('来源类型靠书脊装订区分：非文件 / 单文档 / 表格三档', () => {
    // 非文件来源（人直接建立或网页摘录）
    expect(graphNodeBinding('manual')).toBe('solid')
    expect(graphNodeBinding('web')).toBe('solid')
    // 单文档（可解析出正文）
    expect(graphNodeBinding('markdown')).toBe('dashed')
    expect(graphNodeBinding('text')).toBe('dashed')
    expect(graphNodeBinding('docx')).toBe('dashed')
    expect(graphNodeBinding('pdf')).toBe('dashed')
    // 表格（结构化）
    expect(graphNodeBinding('csv')).toBe('dotted')
    expect(graphNodeBinding('xlsx')).toBe('dotted')
    // 未知类型退回最中性的一档，不抛错
    expect(graphNodeBinding('unknown-type')).toBe('solid')
  })

  it('书本 SVG 同时画出封面、蓝书脊、页口与外框', () => {
    const svg = graphNodeSvg({
      width: 18,
      height: 24,
      sourceType: 'manual',
      center: false,
      selected: false,
      palette,
    })
    expect(svg).toContain('viewBox="0 0 75 100"')
    expect(svg).toContain(`fill="${palette.nodeFill}"`)
    expect(svg).toContain(`fill="${palette.nodeSpine}"`)
    expect(svg).toContain('M71.5 6 V96')
    // 24px 高不画书名线，避免糊成一团
    expect(svg).not.toContain('M20 28 H44')
    // 非中心节点没有书签带
    expect(svg).not.toContain('M58 ')
    // 非选中节点没有选中环
    expect(svg).not.toContain(palette.selectRing)
  })

  it('书脊装订、中心书签、书名线与选中环只在对应状态下出现', () => {
    const svg = graphNodeSvg({
      width: 23,
      height: 30,
      sourceType: 'pdf',
      center: true,
      selected: true,
      palette,
    })
    // PDF 是文档导入：蓝脊上有一条纸白短缝线
    expect(svg).toContain(`stroke="${palette.nodeStitch}"`)
    expect(svg).toContain('stroke-dasharray=')
    // 30px 高开始画两条书名线
    expect(svg).toContain('M20 28 H44')
    // 中心节点书签带
    expect(svg).toContain('M58 ')
    // 选中环
    expect(svg).toContain(`stroke="${palette.selectRing}"`)
  })

  it('书本符号是可直接交给 ECharts 的 SVG data URI', () => {
    const symbol = graphNodeSymbol({
      width: 18,
      height: 24,
      sourceType: 'manual',
      center: false,
      selected: false,
      palette,
    })
    expect(symbol.startsWith('image://data:image/svg+xml')).toBe(true)
  })
})

describe('深色岛调色板', () => {
  it('从 CSS 变量解析出深色岛令牌，而不是纸世界令牌', () => {
    const values: Record<string, string> = {
      '--color-graph-island': '#16191c',
      '--color-graph-island-rule': '#3a424b',
      '--color-graph-island-label': '#a8b0b8',
      '--color-graph-island-label-secondary': '#8b939c',
      '--color-graph-node-border': '#605949',
      '--color-graph-select-ring': '#4a8ac0',
      '--color-graph-edge-ai': '#b2cae1',
      '--color-graph-edge-manual': '#c3c8cb',
      '--color-graph-edge-contradicts': '#d16559',
      '--color-surface': '#f2eee4',
      '--color-surface-secondary': '#e2ddd0',
      '--color-surface-tertiary': '#fbf8ef',
      '--color-label': '#181818',
      '--color-primary': '#1f4e79',
    }
    const root = {
      nodeType: 1,
      ownerDocument: null,
    } as unknown as HTMLElement
    const original = globalThis.getComputedStyle
    globalThis.getComputedStyle = (() => ({
      getPropertyValue: (token: string) => values[token] ?? '',
    })) as unknown as typeof globalThis.getComputedStyle
    try {
      const resolved = resolveGraphPalette(root)
      expect(resolved.island).toBe('#16191c')
      expect(resolved.edgeAI).toBe('#b2cae1')
      expect(resolved.edgeContradicts).toBe('#d16559')
      // 节点填充与墨色直接复用纸世界令牌：纸白填充在深底上 15.23:1
      expect(resolved.nodeFill).toBe('#f2eee4')
      expect(resolved.nodeInk).toBe('#181818')
      // 书脊用图书馆蓝、缝线用纸白，二者都落在纸白节点内部
      expect(resolved.nodeSpine).toBe('#1f4e79')
      expect(resolved.nodeStitch).toBe('#fbf8ef')
      expect(resolved.nodePage).toBe('#e2ddd0')
      // 岛上文字绝不能是纸世界的 #181818（在 #16191c 上只有 1.01:1，等于隐形）
      expect(resolved.label).not.toBe('#181818')
    } finally {
      globalThis.getComputedStyle = original
    }
  })

  it('拿不到元素时返回空串而不是抛错', () => {
    const resolved = resolveGraphPalette(null)
    expect(resolved.island).toBe('')
    expect(resolved.edgeAI).toBe('')
  })
})
