import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeGraphChart from '../KnowledgeGraphChart.vue'

const mocks = vi.hoisted(() => {
  const chart = {
    setOption: vi.fn(),
    dispatchAction: vi.fn(),
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

beforeEach(() => {
  vi.clearAllMocks()
})

describe('KnowledgeGraphChart', () => {
  it('初始化力导向图配置：节点大小按度数、来源类型靠描边样式区分', async () => {
    const wrapper = render()
    await nextTick()
    expect(mocks.init).toHaveBeenCalledTimes(1)
    expect(mocks.use).toHaveBeenCalled()
    const option = mocks.chart.setOption.mock.calls.at(-1)?.[0]
    expect(option.series[0].type).toBe('graph')
    expect(option.series[0].layout).toBe('force')
    expect(option.series[0].roam).toBe(true)
    const data = option.series[0].data
    const center = data.find((item: { id: string }) => item.id === '1')
    const neighbor = data.find((item: { id: string }) => item.id === '2')
    expect(center.symbolSize).toBe(18)
    expect(neighbor.symbolSize).toBe(26)
    // 方形节点：票面 AC③ 要求节点是方形纸白块，不再是圆形色点。
    expect(center.symbol).toBe('rect')
    expect(neighbor.symbol).toBe('rect')
    // 填充统一纸白（深底上才看得见），语义由描边承载。
    expect(center.itemStyle.color).toBe(palette.nodeFill)
    expect(neighbor.itemStyle.color).toBe(palette.nodeFill)
    // 标签画在方块外侧的深底上，必须用岛上文字色（纸世界墨色 #181818 在深底上等于隐形）。
    expect(center.label.color).toBe(palette.label)
    // 来源类型靠描边样式区分：手动创建是实线，PDF 是虚线。
    expect(center.itemStyle.borderType).toBe('solid')
    expect(neighbor.itemStyle.borderType).toBe('dashed')
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
    const links = mocks.chart.setOption.mock.calls.at(-1)?.[0].series[0].links
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
    const links = mocks.chart.setOption.mock.calls.at(-1)?.[0].series[0].links
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
    const series = mocks.chart.setOption.mock.calls.at(-1)?.[0].series[0]
    expect(series.edgeLabel.show).toBe(true)
    expect(series.edgeLabel.fontSize).toBe(10)
    expect(series.edgeLabel.formatter({ data: { relation_type: 'related' } })).toBe('相关')
    expect(series.edgeLabel.formatter({ data: { relation_type: 'depends_on' } })).toBe('依赖')
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
    const series = mocks.chart.setOption.mock.calls.at(-1)?.[0].series[0]
    const link = series.links.find((item: { id: string }) => item.id === '12')
    expect(link.lineStyle.color).toBe('rgb(22 23 24)')
    expect(series.edgeLabel.formatter({ data: { relation_type: 'contradicts' } })).toBe('矛盾')
    wrapper.unmount()
  })

  it('筛选后节点与边集合变化时重建 option，只保留剩余节点与边', async () => {
    const wrapper = render()
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ edges: edges.filter(edge => edge.status === 'confirmed') })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(2)
    let option = mocks.chart.setOption.mock.calls.at(-1)?.[0]
    expect(option.series[0].data).toHaveLength(2)
    expect(option.series[0].links).toHaveLength(1)
    expect(option.series[0].links[0].id).toBe('10')

    await wrapper.setProps({ nodes: [nodes[0]], edges: [] })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(3)
    option = mocks.chart.setOption.mock.calls.at(-1)?.[0]
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

  it('选中样式由 series.select 提供，节点数据内置选中标志已移除', async () => {
    const wrapper = render({ selectedId: 1 })
    await nextTick()
    const option = mocks.chart.setOption.mock.calls.at(-1)?.[0]
    expect(option.series[0].select.itemStyle.borderWidth).toBe(3)
    expect(
      option.series[0].data.every((item: Record<string, unknown>) => !('selected' in item)),
    ).toBe(true)
    wrapper.unmount()
  })

  it('切换选中节点只派发 select/unselect，不重建图表', async () => {
    const wrapper = render({ selectedId: 1 })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(1)
    expect(mocks.chart.dispatchAction).toHaveBeenCalledWith({
      type: 'select',
      seriesIndex: 0,
      dataIndex: 0,
    })

    await wrapper.setProps({ selectedId: 2 })
    await nextTick()
    // 仍是同一张图：没有新的 setOption（重建会重置力导向布局与缩放）。
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(1)
    expect(mocks.chart.dispatchAction).toHaveBeenCalledWith({
      type: 'unselect',
      seriesIndex: 0,
      dataIndex: 0,
    })
    expect(mocks.chart.dispatchAction).toHaveBeenCalledWith({
      type: 'select',
      seriesIndex: 0,
      dataIndex: 1,
    })

    await wrapper.setProps({ selectedId: null })
    await nextTick()
    expect(mocks.chart.setOption).toHaveBeenCalledTimes(1)
    expect(mocks.chart.dispatchAction).toHaveBeenLastCalledWith({
      type: 'unselect',
      seriesIndex: 0,
      dataIndex: 1,
    })
    wrapper.unmount()
  })

  it('selectedId 指向图外节点时不选中任何节点', async () => {
    const wrapper = render({ selectedId: 99 })
    await nextTick()
    expect(mocks.chart.dispatchAction).not.toHaveBeenCalled()
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
