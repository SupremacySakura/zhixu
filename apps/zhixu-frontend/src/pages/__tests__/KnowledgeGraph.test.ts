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
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const chartStub = {
  name: 'KnowledgeGraphChart',
  props: ['nodes', 'edges', 'selectedId'],
  emits: ['node-click'],
  template: `<div data-test="chart" :data-node-count="nodes.length" :data-edge-count="edges.length"><span v-for="edge in edges" :key="edge.id" :data-edge-id="edge.id" :data-edge-status="edge.status" :data-edge-type="edge.relation_type" /><button v-for="node in nodes" :key="node.id" :data-node-id="node.id" @click="$emit('node-click', node.id)">{{ node.title }}</button></div>`,
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

/** 打开项目内的 FilterMenu 并点选指定文案：页面不再使用浏览器原生下拉框。 */
async function chooseFilter(wrapper: ReturnType<typeof mount>, label: string, option: string) {
  const trigger = wrapper
    .findAll('button[aria-haspopup="menu"]')
    .find(button => button.attributes('aria-label') === label)
  expect(trigger).toBeTruthy()
  await trigger!.trigger('click')
  await flushPromises()
  const item = wrapper
    .findAll('[role="menuitemradio"]')
    .find(button => button.text().trim() === option)
  expect(item).toBeTruthy()
  await item!.trigger('click')
  await flushPromises()
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

  it('标签下拉复用项目内的筛选菜单并带全部标签选项', async () => {
    vi.mocked(listTags).mockResolvedValue({
      items: [{ id: 7, name: '研究', usage_count: 2 }],
      total: 1,
    })
    const { wrapper } = await render()
    const trigger = wrapper.find('[data-test="tag-filter"] button[aria-haspopup="menu"]')
    expect(trigger.text()).toContain('全部标签')
    await trigger.trigger('click')
    await flushPromises()
    const options = wrapper.findAll('[role="menuitemradio"]')
    expect(options.map(option => option.text().trim())).toEqual(['全部标签', '研究'])
    expect(options[0].attributes('aria-checked')).toBe('true')
    expect(wrapper.find('select').exists()).toBe(false)
  })

  it('切换标签后重新请求并同步减少节点与边', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.tagId === 7 ? { nodes: [graph.nodes[0]], edges: [], truncated: false } : { ...graph },
    )
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选标签', '研究')
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
    await chooseFilter(wrapper, '筛选标签', '研究')
    await chooseFilter(wrapper, '筛选来源类型', '网页导入')
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
    await wrapper.find('[data-test="source-filter"] button[aria-haspopup="menu"]').trigger('click')
    await flushPromises()
    const options = wrapper.findAll('[role="menuitemradio"]')
    expect(options.map(option => option.text().trim())).toEqual([
      '全部来源',
      '手动创建',
      '网页导入',
      'Markdown',
      '文本',
      'PDF',
      'Word',
      'CSV',
      'Excel',
    ])
    await options.find(option => option.text().trim() === 'PDF')!.trigger('click')
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
    const toggle = wrapper.find('[data-test="suggested-toggle"]')
    expect(toggle.attributes('aria-checked')).toBe('false')
    await toggle.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="suggested-toggle"]').attributes('aria-checked')).toBe('true')
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeSuggested: true }),
      expect.any(AbortSignal),
    )
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-edge-count')).toBe('2')
    expect(chart.find('[data-edge-status="suggested"]').exists()).toBe(true)
    await wrapper.find('[data-test="suggested-toggle"]').trigger('click')
    await flushPromises()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeSuggested: false }),
      expect.any(AbortSignal),
    )
    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('1')
  })

  it('关系类型筛选提供全部与六种类型，文案与图例同源', async () => {
    const { wrapper } = await render()
    const trigger = wrapper.find('[data-test="relation-filter"] button[aria-haspopup="menu"]')
    expect(trigger.text()).toContain('全部关系')
    await trigger.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[role="menuitemradio"]').map(option => option.text().trim())).toEqual([
      '全部关系',
      '相关',
      '依赖',
      '补充说明',
      '对比',
      '更新替代',
      '矛盾',
    ])
  })

  it('选关系类型后在客户端裁剪边，不发新请求', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({
      ...graph,
      edges: [
        ...graph.edges,
        {
          id: 13,
          source_id: 2,
          target_id: 1,
          relation_type: 'depends_on' as const,
          status: 'confirmed' as const,
          source: 'manual' as const,
          confidence: 1,
        },
      ],
    })
    const { wrapper } = await render()
    const chart = wrapper.find('[data-test="chart"]')
    expect(chart.attributes('data-edge-count')).toBe('2')

    await chooseFilter(wrapper, '筛选关系类型', '依赖')

    // 关系类型只在客户端裁剪：后端请求次数不变，节点也不被裁掉。
    expect(getKnowledgeGraph).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('1')
    expect(wrapper.find('[data-edge-type="depends_on"]').exists()).toBe(true)
    expect(wrapper.find('[data-edge-type="related"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="chart"]').attributes('data-node-count')).toBe('2')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
  })

  it('关系类型筛选与图例同步：只剩矛盾边时才出现矛盾图例', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({
      ...graph,
      edges: [
        ...graph.edges,
        {
          id: 12,
          source_id: 1,
          target_id: 2,
          relation_type: 'contradicts' as const,
          status: 'confirmed' as const,
          source: 'manual' as const,
          confidence: 1,
        },
      ],
    })
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选关系类型', '相关')

    expect(wrapper.find('[data-test="graph-legend"]').text()).not.toContain('矛盾关系')
    expect(wrapper.find('[data-edge-type="contradicts"]').exists()).toBe(false)
  })

  it('选中的关系类型没有边时给出提示并可清空', async () => {
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选关系类型', '矛盾')

    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('0')
    expect(wrapper.find('[data-test="relation-filter-empty"]').text()).toContain('矛盾')
    // 节点还在，不该退化成「还没有可展示的知识」。
    expect(wrapper.text()).not.toContain('还没有可展示的知识')

    await wrapper.find('[data-test="clear-filters"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="chart"]').attributes('data-edge-count')).toBe('1')
    expect(wrapper.find('[data-test="relation-filter-empty"]').exists()).toBe(false)
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

  it('图中有矛盾边时图例补一条矛盾关系', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({
      ...graph,
      edges: [
        ...graph.edges,
        {
          id: 12,
          source_id: 1,
          target_id: 2,
          relation_type: 'contradicts' as const,
          status: 'confirmed' as const,
          source: 'manual' as const,
          confidence: 1,
        },
      ],
    })
    const { wrapper } = await render()
    const legend = wrapper.find('[data-test="graph-legend"]')
    expect(legend.text()).toContain('矛盾关系')
    expect(legend.find('[data-test="legend-contradicts"]').exists()).toBe(true)
  })

  it('没有矛盾边时不显示矛盾关系图例项', async () => {
    const { wrapper } = await render()
    expect(wrapper.find('[data-test="graph-legend"]').text()).not.toContain('矛盾关系')
  })

  it('清空筛选后恢复默认请求与完整图', async () => {
    vi.mocked(getKnowledgeGraph).mockImplementation(async (options = {}) =>
      options.tagId === 7 ? { nodes: [graph.nodes[0]], edges: [], truncated: false } : { ...graph },
    )
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选标签', '研究')
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
    await chooseFilter(wrapper, '筛选标签', '研究')
    expect(wrapper.text()).toContain('没有符合筛选条件的知识')
    expect(wrapper.text()).not.toContain('还没有可展示的知识')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="chart"]').exists()).toBe(false)
  })

  it('快速切换筛选会取消上一个图谱请求', async () => {
    // 让请求一直挂起，才能观察到切换筛选时对旧请求的取消。
    vi.mocked(getKnowledgeGraph).mockImplementation(() => new Promise(() => {}) as never)
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选标签', '研究')
    const tagSignal = vi.mocked(getKnowledgeGraph).mock.calls.at(-1)?.[1] as AbortSignal
    expect(tagSignal.aborted).toBe(false)
    await chooseFilter(wrapper, '筛选来源类型', '网页导入')
    expect(tagSignal.aborted).toBe(true)
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(
      expect.objectContaining({ tagId: 7, sourceType: 'web' }),
      expect.any(AbortSignal),
    )
  })

  it('仅打开建议边而图为空时仍显示默认空状态', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
    const { wrapper } = await render()
    await wrapper.find('[data-test="suggested-toggle"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('还没有可展示的知识')
    expect(wrapper.text()).not.toContain('没有符合筛选条件')
    expect(wrapper.find('[data-test="clear-filters"]').exists()).toBe(true)
  })

  it('筛选状态下仍显示截断提示', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ ...graph, truncated: true })
    const { wrapper } = await render()
    await chooseFilter(wrapper, '筛选标签', '研究')
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
    await chooseFilter(wrapper, '筛选标签', '研究')
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
    await chooseFilter(wrapper, '筛选标签', '研究')
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
