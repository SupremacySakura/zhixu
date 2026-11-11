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

const palette = {
  source: {
    manual: 'rgb(1 2 3)',
    pdf: 'rgb(4 5 6)',
    default: 'rgb(7 8 9)',
  },
  edgeAI: 'rgb(10 11 12)',
  edgeManual: 'rgb(13 14 15)',
  label: 'rgb(16 17 18)',
  labelSecondary: 'rgb(19 20 21)',
  surface: 'rgb(255 255 255)',
  hairline: 'rgb(0 0 0 / 0.08)',
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
  it('初始化力导向图配置：节点大小按度数、颜色按来源类型', async () => {
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
    expect(center.itemStyle.color).toBe(palette.source.manual)
    expect(neighbor.itemStyle.color).toBe(palette.source.pdf)
    wrapper.unmount()
  })

  it('边区分 ai/manual，建议边使用虚线', async () => {
    const wrapper = render()
    await nextTick()
    const links = mocks.chart.setOption.mock.calls.at(-1)?.[0].series[0].links
    const ai = links.find((link: { id: string }) => link.id === '10')
    const manual = links.find((link: { id: string }) => link.id === '11')
    expect(ai.lineStyle.color).toBe(palette.edgeAI)
    expect(ai.lineStyle.type).toBe('solid')
    expect(manual.lineStyle.color).toBe(palette.edgeManual)
    expect(manual.lineStyle.type).toBe('dashed')
    expect(ai.source).toBe('1')
    expect(ai.target).toBe('2')
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
