import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeGraph from '../KnowledgeGraph.vue'
import { getKnowledgeGraph, listKnowledge } from '../../services/apis/knowledge'

vi.mock('../../services/apis/knowledge', () => ({
  getKnowledgeGraph: vi.fn(),
  listKnowledge: vi.fn(),
}))
vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))

const chartStub = {
  name: 'KnowledgeGraphChart',
  props: ['nodes', 'edges', 'selectedId'],
  emits: ['node-click'],
  template: `<div data-test="chart"><button v-for="node in nodes" :key="node.id" :data-node-id="node.id" @click="$emit('node-click', node.id)">{{ node.title }}</button></div>`,
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
  vi.mocked(getKnowledgeGraph).mockResolvedValue({ ...graph })
  vi.mocked(listKnowledge).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('KnowledgeGraph 页面', () => {
  it('挂载后装载最近图谱并渲染标题与说明', async () => {
    const { wrapper } = await render()
    expect(getKnowledgeGraph).toHaveBeenCalledWith(undefined, false, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('知识图谱')
    expect(wrapper.find('[data-test="chart"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="chart"]').text()).toContain('中心知识')
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

  it('截断时给出明确提示', async () => {
    vi.mocked(getKnowledgeGraph).mockResolvedValue({ ...graph, truncated: true })
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('已截断')
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
    expect(getKnowledgeGraph).toHaveBeenCalledWith(2, false, expect.any(AbortSignal))
  })

  it('从 ?center= 进入时以该知识为中心', async () => {
    await render('/home/knowledge/graph?center=2')
    expect(getKnowledgeGraph).toHaveBeenCalledWith(2, false, expect.any(AbortSignal))
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
    expect(getKnowledgeGraph).toHaveBeenCalledWith(42, false, expect.any(AbortSignal))
  })

  it('回到最近会清空中心参数并重新加载', async () => {
    const { wrapper, router } = await render('/home/knowledge/graph?center=2')
    const back = wrapper.findAll('button').find(button => button.text().includes('回到最近'))
    await back?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.center).toBeUndefined()
    expect(getKnowledgeGraph).toHaveBeenLastCalledWith(undefined, false, expect.any(AbortSignal))
  })
})
