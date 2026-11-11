import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeGraph from '../KnowledgeGraph.vue'
import { getKnowledgeGraph, listKnowledge } from '../../services/apis/knowledge'
import { listTags } from '../../services/apis/tag'

vi.mock('../../services/apis/knowledge', () => ({
  getKnowledgeGraph: vi.fn(),
  listKnowledge: vi.fn(),
}))
vi.mock('../../services/apis/tag', () => ({ listTags: vi.fn() }))
vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))

const chartStub = {
  name: 'KnowledgeGraphChart',
  props: ['nodes', 'edges', 'selectedId'],
  emits: ['node-click'],
  template: `<div data-test="chart" :data-node-count="nodes.length" :data-edge-count="edges.length"><span v-for="edge in edges" :key="edge.id" :data-edge-id="edge.id" :data-edge-status="edge.status" /><button v-for="node in nodes" :key="node.id" :data-node-id="node.id" @click="$emit('node-click', node.id)">{{ node.title }}</button></div>`,
}

const graph = {
  nodes: [
    {
      id: 1,
      title: '中心知识',
      source_type: 'manual',
      summary: '摘要一',
      degree: 1,
      updated_at: '2026-11-10T10:00:00+08:00',
    },
    {
      id: 2,
      title: '相邻知识',
      source_type: 'pdf',
      summary: '',
      degree: 1,
      updated_at: '2026-11-09T10:00:00+08:00',
    },
  ],
  edges: [
    {
      id: 10,
      source_id: 1,
      target_id: 2,
      relation_type: 'related' as const,
      status: 'confirmed' as const,
      source: 'ai' as const,
      confidence: 0.9,
    },
  ],
  truncated: false,
}

const suggestedEdge = {
  id: 11,
  source_id: 2,
  target_id: 1,
  relation_type: 'related' as const,
  status: 'suggested' as const,
  source: 'manual' as const,
  confidence: 0.6,
}

async function render(path = '/home/knowledge/graph') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home/knowledge', component: { template: '<div />' } },
      { path: '/home/knowledge/graph', component: KnowledgeGraph },
      { path: '/home/knowledge/:id', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(KnowledgeGraph, {
    global: {
      plugins: [router],
      stubs: { KnowledgeGraphChart: chartStub, Motion: { template: '<div><slot /></div>' } },
    },
    attachTo: document.body,
  })
  await flushPromises()
  return { wrapper, router }
}

beforeEach(() => {
  vi.mocked(getKnowledgeGraph).mockReset()
  vi.mocked(listKnowledge).mockReset()
  vi.mocked(listTags).mockReset()
  vi.mocked(getKnowledgeGraph).mockResolvedValue({ ...graph })
  vi.mocked(listKnowledge).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  vi.mocked(listTags).mockResolvedValue({
    items: [{ id: 7, name: '研究', usage_count: 2 }],
    total: 1,
  })
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('KnowledgeGraph 页面', () => {
  it('挂载后装载最近图谱并渲染标题与说明', async () => {
    const { wrapper } = await render()
    expect(getKnowledgeGraph).toHaveBeenCalledWith(
      { centerId: undefined, includeSuggested: false, tagId: undefined, sourceType: undefined },
      expect.any(AbortSignal),
    )
    expect(wrapper.text()).toContain('知识图谱')
    expect(wrapper.find('[data-test="chart"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="chart"]').text()).toContain('中心知识')
  })

  it('标签下拉展示接口返回的标签并带全部标签选项', async () => {
    vi.mocked(listTags).mockResolvedValue({
      items: [{ id: 7, name: '研究', usage_count: 2 }],
      total: 1,
    })
    const { wrapper } = await render()
    const options = wrapper.find('[data-test="tag-filter"]').findAll('option')
    expect(options.map(option => option.text())).toEqual(['全部标签', '研究'])
    expect(options.map(option => option.attributes('value'))).toEqual(['0', '7'])
  })

  it('切换标签后重新请求并同步减少节点与边', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.tagId === 7 ? { nodes: [graph.nodes[0]], edges: [], truncated: false } : { ...graph },
    )
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 7, sourceType: undefined, centerId: undefined }),
      expect.any(AbortSignal),
    )
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-node-count')).toBe('1')
    expect(chart.attributes('data-edge-count')).toBe('0')
    expect(chart.text()).toContain('中心知识')
    expect(chart.text()).not.toContain('相邻知识')
  })

  it('来源类型筛选与标签取交集一起透传', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.sourceType === 'web'
        ? { nodes: [graph.nodes[1]], edges: [], truncated: false }
        : { ...graph },
    )
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    await wrapper.find('[data-test="source-filter"]').setValue('web')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 7, sourceType: 'web' }),
      expect.any(AbortSignal),
    )
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-node-count')).toBe('1')
    expect(chart.text()).toContain('相邻知识')
  })

  it('来源筛选项覆盖文件导入格式并能提交 pdf', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.sourceType === 'pdf'
        ? { nodes: [graph.nodes[1]], edges: [], truncated: false }
        : { ...graph },
    )
    const { wrapper } = await render()
    const sourceSelect = wrapper.find('[data-test="source-filter"]')
    const optionValues = sourceSelect.findAll('option').map(option => option.attributes('value'))
    expect(optionValues).toEqual([
      '',
      'manual',
      'web',
      'markdown',
      'text',
      'pdf',
      'docx',
      'csv',
      'xlsx',
    ])
    await sourceSelect.setValue('pdf')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ sourceType: 'pdf' }),
      expect.any(AbortSignal),
    )
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-node-count')).toBe('1')
    expect(chart.text()).toContain('相邻知识')
  })

  it('打开建议边开关后请求 include_suggested 并给图表补上建议边', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.includeSuggested
        ? { ...graph, edges: [...graph.edges, suggestedEdge] }
        : { ...graph },
    )
    const { wrapper } = await render()
    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('1')
    await wrapper.find('[data-test="suggested-toggle"]').setValue(true)
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeSuggested: true }),
      expect.any(AbortSignal),
    )
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-edge-count')).toBe('2')
    expect(chart.find('[data-edge-status="suggested"]').exists()).toBe(true)
    await wrapper.find('[data-test="suggested-toggle"]').setValue(false)
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeSuggested: false }),
      expect.any(AbortSignal),
    )
    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('1')
  })

  it('图例区分已确认、建议与 AI/手动关联', async () => {
    const { wrapper } = await render()
    const legend = wrapper.find('[data-test="graph-legend"]')
    expect(legend.exists()).toBe(true)
    expect(legend.text()).toContain('已确认关系')
    expect(legend.text()).toContain('建议关系')
    expect(legend.text()).toContain('AI 关联')
    expect(legend.text()).toContain('手动关联')
    expect(legend.find('.border-solid').exists()).toBe(true)
    expect(legend.find('.border-dashed').exists()).toBe(true)
  })

  it('清空筛选后恢复默认请求与完整图', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.tagId === 7 ? { nodes: [graph.nodes[0]], edges: [], truncated: false } : { ...graph },
    )
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
    await wrapper.find('[data-test="clear-filters"]').trigger('click')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      { centerId: undefined, includeSuggested: false, tagId: undefined, sourceType: undefined },
      expect.any(AbortSignal),
    )
    expect(wrapper.find('[data-test="chart"]').attributes('data-node-count')).toBe('2')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(false)
  })

  it('筛选后无匹配时展示筛选空态并可清空', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(wrapper.text()).toContain('没有符合筛选条件的知识')
    expect(wrapper.text()).not.toContain('还没有可展示的知识')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="chart"]').exists()).toBe(false)
  })

  it('快速切换筛选会取消上一个图谱请求', async () => {
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    const tagSignal = vi.mocked(getKnowledgeGraph).mock.calls.at(-1)?.[1] as AbortSignal
    expect(tagSignal.aborted).toBe(false)
    await wrapper.find('[data-test="source-filter"]').setValue('web')
    expect(tagSignal.aborted).toBe(true)
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 7, sourceType: 'web' }),
      expect.any(AbortSignal),
    )
  })

  it('仅打开建议边而图为空时仍显示默认空状态', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
    const { wrapper } = await render()
    await wrapper.find('[data-test="suggested-toggle"]').setValue(true)
    await flushPromises()
    expect(wrapper.text()).toContain('还没有可展示的知识')
    expect(wrapper.text()).not.toContain('没有符合筛选条件')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
  })

  it('筛选状态下仍显示截断提示', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ ...graph, truncated: true })
    const { wrapper } = await render()
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(wrapper.text()).toContain('已截断')
  })

  it('没有节点时展示空状态引导而不是空白画布', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('还没有可展示的知识')
    expect(wrapper.find('[data-test="chart"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/home/knowledge"]').exists()).toBe(true)
  })

  it('有节点但没有关系时提示先去建立关系', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({
      nodes: [graph.nodes[0]],
      edges: [],
      truncated: false,
    })
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('还没有已确认关系')
  })

  it('点击节点打开抽屉，展示标题、来源、度数与一跳关系', async () => {
    const { wrapper } = await render()
    const nodeButton = wrapper
      .findAll('[data-test="chart"] button')
      .find(button => button.text().includes('中心知识'))
    await nodeButton?.trigger('click')
    expect(wrapper.text()).toContain('摘要一')
    expect(wrapper.text()).toContain('手动创建')
    expect(wrapper.text()).toContain('度数 1')
    expect(wrapper.text()).toContain('相关')
    expect(wrapper.text()).toContain('相邻知识')
    expect(wrapper.text()).toContain('查看详情')
  })

  it('筛选变化时保留仍在图中的选中节点与抽屉', async () => {
    const { wrapper } = await render()
    const nodeButton = wrapper
      .findAll('[data-test="chart"] button')
      .find(button => button.text().includes('中心知识'))
    await nodeButton?.trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('摘要一')
  })

  it('以此为中心重绘并同步 URL', async () => {
    const { wrapper, router } = await render()
    const nodeButton = wrapper
      .findAll('[data-test="chart"] button')
      .find(button => button.text().includes('相邻知识'))
    await nodeButton?.trigger('click')
    const recenter = wrapper.findAll('button').find(button => button.text().includes('以此为中心'))
    await recenter?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.center).toBe('2')
    expect(getKnowledgeGraph).toHaveBeenCalledWith(
      expect.objectContaining({ centerId: 2 }),
      expect.any(AbortSignal),
    )
  })

  it('从 ?center= 进入时以该知识为中心并可与标签筛选叠加', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
    const { wrapper } = await render('/home/knowledge/graph?center=2')
    expect(getKnowledgeGraph).toHaveBeenCalledWith(
      expect.objectContaining({ centerId: 2 }),
      expect.any(AbortSignal),
    )
    await wrapper.find('[data-test="tag-filter"]').setValue('7')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ centerId: 2, tagId: 7 }),
      expect.any(AbortSignal),
    )
    expect(wrapper.text()).toContain('没有符合筛选条件的知识')
  })

  it('搜索知识并选中后以它为中心重绘', async () => {
    vi.useFakeTimers()
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [{ id: 42, title: '目标知识', source_type: 'markdown' }],
      total: 1,
      page: 1,
      page_size: 20,
    } as never)
    const { wrapper, router } = await render()
    const input = wrapper.find('input[placeholder*="搜索"]')
    await input.setValue('目标')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(listKnowledge).toHaveBeenCalled()
    const result = wrapper.findAll('button').find(button => button.text().includes('目标知识'))
    expect(result).toBeTruthy()
    await result?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.center).toBe('42')
    expect(getKnowledgeGraph).toHaveBeenCalledWith(
      expect.objectContaining({ centerId: 42 }),
      expect.any(AbortSignal),
    )
  })

  it('回到最近会清空中心参数并重新加载', async () => {
    const { wrapper, router } = await render('/home/knowledge/graph?center=2')
    const back = wrapper.findAll('button').find(button => button.text().includes('回到最近'))
    await back?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.center).toBeUndefined()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      { centerId: undefined, includeSuggested: false, tagId: undefined, sourceType: undefined },
      expect.any(AbortSignal),
    )
  })
})
