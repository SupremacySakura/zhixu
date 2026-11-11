import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getKnowledgeGraph, listKnowledge } from '../../services/apis/knowledge'
import { listTags } from '../../services/apis/tag'
import { useKnowledgeGraph } from '../useKnowledgeGraph'

vi.mock('../../services/apis/knowledge', () => ({
  getKnowledgeGraph: vi.fn(),
  listKnowledge: vi.fn(),
}))
vi.mock('../../services/apis/tag', () => ({ listTags: vi.fn() }))

const deferred = <T>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => {
    resolve = done
  })
  return { promise, resolve }
}

async function mountComposable() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/home/knowledge/graph', component: { template: '<div />' } }],
  })
  await router.push('/home/knowledge/graph')
  await router.isReady()
  let api: ReturnType<typeof useKnowledgeGraph> | undefined
  const host = defineComponent({
    setup() {
      api = useKnowledgeGraph()
      return () => h('div')
    },
  })
  const wrapper = mount(host, { global: { plugins: [router] } })
  await flushPromises()
  if (!api) throw new Error('composable did not initialize')
  return { api, wrapper }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(listKnowledge).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  vi.mocked(listTags).mockResolvedValue({ items: [], total: 0 })
  vi.mocked(getKnowledgeGraph).mockResolvedValue({ nodes: [], edges: [], truncated: false })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useKnowledgeGraph', () => {
  it('卸载时取消图谱与标签的未完成请求并移除键盘监听', async () => {
    const graph = deferred<Awaited<ReturnType<typeof getKnowledgeGraph>>>()
    const tags = deferred<Awaited<ReturnType<typeof listTags>>>()
    vi.mocked(getKnowledgeGraph).mockReturnValue(graph.promise)
    vi.mocked(listTags).mockReturnValue(tags.promise)
    const removeListener = vi.spyOn(window, 'removeEventListener')

    const { wrapper } = await mountComposable()
    const graphSignal = vi.mocked(getKnowledgeGraph).mock.calls[0]?.[1]
    const tagsSignal = vi.mocked(listTags).mock.calls[0]?.[0]
    expect(graphSignal?.aborted).toBe(false)
    expect(tagsSignal?.aborted).toBe(false)

    wrapper.unmount()

    expect(graphSignal?.aborted).toBe(true)
    expect(tagsSignal?.aborted).toBe(true)
    expect(removeListener).toHaveBeenCalledWith('keydown', expect.any(Function))
    removeListener.mockRestore()
  })

  it('搜索请求在 250ms 防抖后发出，卸载时取消进行中的请求', async () => {
    vi.useFakeTimers()
    const { api, wrapper } = await mountComposable()
    api.searchTerm.value = '目标'

    api.scheduleSearch()
    await vi.advanceTimersByTimeAsync(249)
    expect(listKnowledge).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    expect(listKnowledge).toHaveBeenCalledWith(1, '目标', '', '', {}, expect.any(AbortSignal))
    const signal = vi.mocked(listKnowledge).mock.calls[0]?.[5]

    wrapper.unmount()

    expect(signal?.aborted).toBe(true)
  })
})
