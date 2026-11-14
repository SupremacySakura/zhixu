import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeGraphChart from '../KnowledgeGraphChart.vue'

const mocks = vi.hoisted(() => {
  const chart = {
    setOption: vi.fn(),
    resize: vi.fn(),
    dispose: vi.fn(),
    on: vi.fn(),
    off: vi.fn(),
  }
  return { chart, init: vi.fn(() => chart), use: vi.fn() }
})

vi.mock('echarts/core', () => ({ init: mocks.init, use: mocks.use }))
vi.mock('echarts/charts', () => ({ GraphChart: {} }))
vi.mock('echarts/components', () => ({ TooltipComponent: {} }))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))
vi.mock('echarts/features', () => ({ LabelLayout: {} }))

const palette = {
  island: 'rgb(0 0 0)',
  islandRule: 'rgb(1 1 1)',
  label: 'rgb(16 17 18)',
  labelSecondary: 'rgb(19 20 21)',
  nodeFill: 'rgb(240 238 228)',
  nodeInk: 'rgb(24 24 24)',
  nodeBorder: 'rgb(96 89 73)',
  nodeSpine: 'rgb(31 78 121)',
  nodeStitch: 'rgb(251 248 239)',
  nodePage: 'rgb(226 221 208)',
  selectRing: 'rgb(74 138 192)',
  edgeAI: 'rgb(10 11 12)',
  edgeManual: 'rgb(13 14 15)',
  edgeContradicts: 'rgb(25 26 27)',
  tooltipBg: 'rgb(255 255 255)',
  tooltipInk: 'rgb(24 24 24)',
}

const nodes = [
  {
    id: 1,
    title: '中心知识',
    source_type: 'manual',
    summary: '',
    degree: 0,
    updated_at: '2026-11-10T10:00:00+08:00',
  },
  {
    id: 2,
    title: 'PDF 知识',
    source_type: 'pdf',
    summary: '',
    degree: 2,
    updated_at: '2026-11-09T10:00:00+08:00',
  },
]
const edges = [
  {
    id: 10,
    source_id: 1,
    target_id: 2,
    relation_type: 'related' as const,
    status: 'confirmed' as const,
    source: 'ai' as const,
    confidence: 0.9,
  },
  {
    id: 11,
    source_id: 2,
    target_id: 1,
    relation_type: 'depends_on' as const,
    status: 'suggested' as const,
    source: 'manual' as const,
    confidence: 1,
  },
]

function render(overrides: Record<string, unknown> = {}) {
  return mount(KnowledgeGraphChart, {
    props: { nodes, edges, palette, ...overrides },
    attachTo: document.body,
  })
}

/**
 * 取最近一次「完整配置」的 setOption 入参。
 *
 * 组件是两步渲染：第一步 notMerge 铺全部图形配置（此时还没有图元可以打标），
 * 第二步只补 labelLayout。所以不能直接取 at(-1)，那拿到的是第二步的补丁。
 */

interface FullSeries {
  type: string
  layout?: string
  roam?: boolean
  data: any[]
  links: any[]
  emphasis: { scale: boolean; focus: string; lineStyle: { opacity: number } }
  edgeLabel: { show: boolean; fontSize: number; formatter: (params: any) => string }
  labelLayout?: any
}

function lastFullOption(): { series: FullSeries[] } {
  const calls = mocks.chart.setOption.mock.calls
  for (let i = calls.length - 1; i >= 0; i -= 1) {
    const arg = calls[i]?.[0] as { series?: { type?: string }[] } | undefined
    if (arg?.series?.[0]?.type === 'graph') {
      return arg as unknown as { series: FullSeries[] }
    }
  }
  throw new Error('没有找到带完整图形配置的 setOption 调用')
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('KnowledgeGraphChart', () => {
  it('初始化力导向图配置：书本节点按度数定尺寸、来源靠书脊装订区分', async () => {
    const wrapper = render()
    await nextTick()
    expect(mocks.init).toHaveBeenCalledTimes(1)
    expect(mocks.use).toHaveBeenCalled()
    const option = lastFullOption()
    expect(option.series[0].type).toBe('graph')
    expect(option.series[0].layout).toBe('force')
    expect(option.series[0].roam).toBe(true)
    // 书本节点不随 hover 放大；邻接边提亮是唯一的状态变化。
    expect(option.series[0].emphasis).toEqual({
      scale: false,
      focus: 'adjacency',
      lineStyle: { opacity: 1 },
    })
    const data = option.series[0].data
    const center = data.find((item: { id: string }) => item.id === '1')
    const neighbor = data.find((item: { id: string }) => item.id === '2')
    expect(center.symbolSize).toEqual([18, 24])
    expect(neighbor.symbolSize).toEqual([23, 30])
    // 多色书本走 SVG data URI，不再是 rect 方块。
    expect(center.symbol.startsWith('image://data:image/svg+xml')).toBe(true)
    expect(neighbor.symbol.startsWith('image://data:image/svg+xml')).toBe(true)
    const centerSvg = decodeURIComponent(center.symbol.slice(center.symbol.indexOf(',') + 1))
    const neighborSvg = decodeURIComponent(neighbor.symbol.slice(neighbor.symbol.indexOf(',') + 1))
    // 纸白封面 + 图书馆蓝书脊
    expect(centerSvg).toContain(`fill="${palette.nodeFill}"`)
    expect(centerSvg).toContain(`fill="${palette.nodeSpine}"`)
    // 标签画在书本外侧的深底上，必须用岛上文字色（纸世界墨色 #181818 在深底上等于隐形）。
    expect(center.label.color).toBe(palette.label)
    // 来源类型靠书脊装订区分：手动创建是实心蓝脊，PDF 是纸白短缝线。
    expect(centerSvg).not.toContain('stroke-dasharray=')
    expect(neighborSvg).toContain('stroke-dasharray=')
    wrapper.unmount()
  })

  it('六类关系两两可辨：线型、箭头、线宽、透明度四轴组合互不相同', async () => {
    const allTypes = [
      'related',
      'depends_on',
      'supplements',
      'contrasts',
      'updates',
      'contradicts',
    ] as const
    const wrapper = render({
      edges: allTypes.map((relation_type, index) => ({
        id: 100 + index,
        source_id: 1,
        target_id: 2,
        relation_type,
        status: 'confirmed' as const,
        source: 'ai' as const,
        confidence: 1,
      })),
    })
    await nextTick()
    const links = lastFullOption().series[0].links
    const fingerprint = (link: {
      lineStyle: { type: unknown; width: number; opacity: number }
      symbol: unknown
    }) =>
      JSON.stringify([
        link.lineStyle.type,
        link.symbol,
        link.lineStyle.width,
        link.lineStyle.opacity,
      ])

    // 四轴指纹两两不等 —— 这是 AC① 的可观察形态，与具体取值无关。
    const prints = links.map(fingerprint)
    expect(new Set(prints).size).toBe(allTypes.length)

    // 线型必须是数值数组（'1 3' 是 SVG 写法，ECharts 会当成无效值静默忽略）。
    for (const link of links) {
      const type = link.lineStyle.type
      if (type === 'solid') continue
      expect(Array.isArray(type)).toBe(true)
      expect((type as number[]).every((part: unknown) => typeof part === 'number')).toBe(true)
    }

    // 箭头逐边设置，不是 series 级统一值。
    const byType = (type: string) =>
      links.find((link: { relation_type: string }) => link.relation_type === type)
    expect(byType('related').symbol).toEqual(['none', 'none'])
    expect(byType('contrasts').symbol).toEqual(['arrow', 'arrow'])
    expect(byType('contradicts').symbol).toEqual(['arrow', 'arrow'])
    for (const type of ['depends_on', 'supplements', 'updates']) {
      expect(byType(type).symbol).toEqual(['none', 'arrow'])
    }
    wrapper.unmount()
  })

  it('边按来源分色：AI 与手动不同色，矛盾无视来源用唯一彩色', async () => {
    const wrapper = render()
    await nextTick()
    const links = lastFullOption().series[0].links
    const ai = links.find((link: { id: string }) => link.id === '10')
    const manual = links.find((link: { id: string }) => link.id === '11')
    expect(ai.lineStyle.color).toBe(palette.edgeAI)
    expect(manual.lineStyle.color).toBe(palette.edgeManual)
    expect(ai.source).toBe('1')
    expect(ai.target).toBe('2')
    wrapper.unmount()
  })

  it('边标签显示中文关系类型且使用小字号', async () => {
    const wrapper = render()
    await nextTick()
    const series = lastFullOption().series[0]
    expect(series.edgeLabel.show).toBe(true)
    expect(series.edgeLabel.fontSize).toBe(10)
    expect(series.edgeLabel.formatter({ data: { relation_type: 'related' } })).toBe('相关')
    expect(series.edgeLabel.formatter({ data: { relation_type: 'depends_on' } })).toBe('依赖')
    wrapper.unmount()
  })

  it('节点 tooltip 显示标题、度数与确切来源类型', async () => {
    const wrapper = render()
    await nextTick()
    const option = lastFullOption() as unknown as {
      tooltip: { formatter: (params: unknown) => string }
    }
    expect(
      option.tooltip.formatter({
        dataType: 'node',
        data: { name: 'PDF 知识', value: 2, source_type: 'pdf' },
      }),
    ).toBe('PDF 知识（度数 2） · 来源：PDF')
    wrapper.unmount()
  })

  it('矛盾边使用矛盾色并标注「矛盾」', async () => {
    const contradiction = {
      id: 12,
      source_id: 1,
      target_id: 2,
      relation_type: 'contradicts' as const,
      status: 'confirmed' as const,
      source: 'manual' as const,
      confidence: 1,
    }
    const wrapper = render({
      edges: [...edges, contradiction],
      palette: { ...palette, edgeContradicts: 'rgb(22 23 24)' },
    })
    await nextTick()
    const series = lastFullOption().series[0]
    const link = series.links.find((item: { id: string }) => item.id === '12')
    expect(link.lineStyle.color).toBe('rgb(22 23 24)')
    expect(series.edgeLabel.formatter({ data: { relation_type: 'contradicts' } })).toBe('矛盾')
    wrapper.unmount()
  })

  it('筛选后节点与边集合变化时重建 option，只保留剩余节点与边', async () => {
    const wrapper = render()
    await nextTick()
    // 两步渲染：铺图形 + 补 labelLayout。
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(2)

    await wrapper.setProps({ edges: edges.filter(edge => edge.status === 'confirmed') })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(4)
    let option = lastFullOption()
    expect(option.series[0].data).toHaveLength(2)
    expect(option.series[0].links).toHaveLength(1)
    expect(option.series[0].links[0].id).toBe('10')

    await wrapper.setProps({ nodes: [nodes[0]], edges: [] })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(6)
    option = lastFullOption()
    expect(option.series[0].data).toHaveLength(1)
    expect(option.series[0].links).toHaveLength(0)
    wrapper.unmount()
  })

  it('点击节点时向父组件抛出节点 id', async () => {
    const wrapper = render()
    await nextTick()
    const click = mocks.chart.on.mock.calls.find(call => call[0] === 'click')?.[1]
    expect(click).toBeTypeOf('function')
    click({ dataType: 'node', data: { id: '2' } })
    expect(wrapper.emitted('node-click')).toEqual([[2]])
    click({ dataType: 'edge', data: { id: '10' } })
    expect(wrapper.emitted('node-click')).toHaveLength(1)
    wrapper.unmount()
  })

  it('按需注册力导向图、提示框、Canvas 渲染器与标签布局', async () => {
    const wrapper = render()
    await nextTick()
    expect(mocks.use.mock.calls[0]?.[0]).toHaveLength(4)
    wrapper.unmount()
  })

  it('选中节点用 SVG 变体画出 3px 蓝色环，其他书本没有', async () => {
    const wrapper = render({ selectedId: 1 })
    await nextTick()
    const option = lastFullOption()
    const selected = option.series[0].data.find((item: { id: string }) => item.id === '1')
    const other = option.series[0].data.find((item: { id: string }) => item.id === '2')
    const selectedSvg = decodeURIComponent(selected.symbol.slice(selected.symbol.indexOf(',') + 1))
    const otherSvg = decodeURIComponent(other.symbol.slice(other.symbol.indexOf(',') + 1))
    expect(selectedSvg).toContain(`stroke="${palette.selectRing}"`)
    expect(otherSvg).not.toContain(`stroke="${palette.selectRing}"`)
    wrapper.unmount()
  })

  it('只有中心节点带书签带', async () => {
    const wrapper = render({ centerId: 2 })
    await nextTick()
    const option = lastFullOption()
    const center = option.series[0].data.find((item: { id: string }) => item.id === '2')
    const other = option.series[0].data.find((item: { id: string }) => item.id === '1')
    const centerSvg = decodeURIComponent(center.symbol.slice(center.symbol.indexOf(',') + 1))
    const otherSvg = decodeURIComponent(other.symbol.slice(other.symbol.indexOf(',') + 1))
    expect(centerSvg).toContain('M58 ')
    expect(otherSvg).not.toContain('M58 ')
    wrapper.unmount()
  })

  it('切换选中节点只更新节点与邻接边样式，不重建整张图', async () => {
    const third = {
      id: 3,
      title: '第三本知识',
      source_type: 'web',
      summary: '',
      degree: 1,
      updated_at: '2026-11-08T10:00:00+08:00',
    }
    const bridge = {
      id: 12,
      source_id: 2,
      target_id: 3,
      relation_type: 'related' as const,
      status: 'confirmed' as const,
      source: 'ai' as const,
      confidence: 0.8,
    }
    const wrapper = render({ nodes: [...nodes, third], edges: [...edges, bridge] })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(2)
    const baseline = lastFullOption().series[0].links

    await wrapper.setProps({ selectedId: 3 })
    await nextTick()
    // 只补 data/links，不带 type，避免重建 series 丢力导向位置。
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(3)
    const patch = mocks.chart.setOption.mock.calls.at(-1)?.[0]
    expect(patch.series[0].type).toBeUndefined()
    const adjacent = patch.series[0].links.find((item: { id: string }) => item.id === '12')
    const other = patch.series[0].links.find((item: { id: string }) => item.id === '10')
    const baselineOther = baseline.find((item: { id: string }) => item.id === '10')
    expect(adjacent.lineStyle.opacity).toBe(1)
    expect(other.lineStyle.opacity).toBe(baselineOther.lineStyle.opacity)
    const selected = patch.series[0].data.find((item: { id: string }) => item.id === '3')
    const selectedSvg = decodeURIComponent(selected.symbol.slice(selected.symbol.indexOf(',') + 1))
    expect(selectedSvg).toContain(`stroke="${palette.selectRing}"`)

    await wrapper.setProps({ selectedId: null })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(4)
    const cleared = mocks.chart.setOption.mock.calls.at(-1)?.[0]
    const clearedAdjacent = cleared.series[0].links.find((item: { id: string }) => item.id === '12')
    const baselineAdjacent = baseline.find((item: { id: string }) => item.id === '12')
    expect(clearedAdjacent.lineStyle.opacity).toBe(baselineAdjacent.lineStyle.opacity)
    for (const item of cleared.series[0].data) {
      const svg = decodeURIComponent(item.symbol.slice(item.symbol.indexOf(',') + 1))
      expect(svg).not.toContain(`stroke="${palette.selectRing}"`)
    }
    wrapper.unmount()
  })

  it('selectedId 指向图外节点时所有书本都是常态', async () => {
    const wrapper = render({ selectedId: 99 })
    await nextTick()
    for (const item of lastFullOption().series[0].data) {
      const svg = decodeURIComponent(item.symbol.slice(item.symbol.indexOf(',') + 1))
      expect(svg).not.toContain(`stroke="${palette.selectRing}"`)
    }
    wrapper.unmount()
  })

  it('边标签退出 labelLayout 接管：先铺图元、打标 disableLabelLayout、再挂 labelLayout', async () => {
    // 这是一条防回归契约。ECharts 的 LabelManager 只要看到非空 labelLayout，就会把所有
    // 标签宿主的 textConfig.local 强制改成 false（LabelManager.js updateLayoutConfig）。
    // 边标签的 x/y 是曲线中点的局部坐标，一旦按全局解读，缩放时会被按 1/scale 反向放大
    // 再推远 —— 实测缩 12 次后偏离关系线 67–311px。唯一的挡板是给边标签打
    // disableLabelLayout，而打标需要图元已存在，所以顺序必须是「渲染 → 打标 → 挂布局」。
    const wrapper = render()
    await nextTick()

    const calls = mocks.chart.setOption.mock.calls
    expect(calls).toHaveLength(2)

    // 第一步：完整图形配置，且 labelLayout 为 null（此时还没有图元可打标）。
    const first = calls[0][0]
    expect(first.series[0].type).toBe('graph')
    expect(first.series[0].labelLayout).toBeNull()

    // 第二步：只补 labelLayout，且必须按 dataType 把边排除掉。
    const second = calls[1][0]
    expect(second.series).toHaveLength(1)
    expect(second.series[0].data).toBeUndefined()
    const labelLayout = second.series[0].labelLayout
    expect(labelLayout({ dataType: 'edge' })).toBeUndefined()
    expect(labelLayout({ dataType: 'node' })).toEqual({ hideOverlap: true })

    wrapper.unmount()
  })

  it('图表缺少 getModel 时不崩（测试替身与旧版本 ECharts 的兜底）', async () => {
    // attachEdgeLabelOptOut 走的是 ECharts 的 private API，替身里没有 getModel 是常态；
    // 这条保证它只是静默跳过打标，而不是把整个图表初始化带崩。
    const wrapper = render()
    await nextTick()
    expect(() => window.dispatchEvent(new Event('resize'))).not.toThrow()
    expect(mocks.chart.setOption).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('监听 resize，卸载时释放图表与监听', async () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const wrapper = render()
    await nextTick()
    window.dispatchEvent(new Event('resize'))
    expect(mocks.chart.resize).toHaveBeenCalled()
    wrapper.unmount()
    expect(mocks.chart.dispose).toHaveBeenCalled()
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
    removeSpy.mockRestore()
  })
})
