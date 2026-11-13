import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeVectorPanel from '../KnowledgeVectorPanel.vue'
import { retryKnowledgeVectorize, type KnowledgeItem } from '../../../services/apis/knowledge'

vi.mock('../../../services/apis/knowledge', async importOriginal => {
  const mod = await importOriginal<typeof import('../../../services/apis/knowledge')>()
  return { ...mod, retryKnowledgeVectorize: vi.fn() }
})
vi.mock('../../ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const mockRetry = vi.mocked(retryKnowledgeVectorize)
const mounted: ReturnType<typeof mount>[] = []
afterEach(() => {
  mounted.forEach(wrapper => wrapper.unmount())
  mounted.length = 0
  vi.clearAllMocks()
})

const makeItem = (overrides: Partial<KnowledgeItem> = {}): KnowledgeItem =>
  ({
    id: 10,
    source_file_id: 1,
    source_file_name: 'note.md',
    source_folder_id: 0,
    source_folder_name: '工作',
    source_type: 'markdown',
    title: '笔记',
    content: '',
    status: 'ready',
    error_code: '',
    error_message: '',
    imported_at: '2026-10-29T10:00:00Z',
    created_at: '2026-10-29T10:00:00Z',
    updated_at: '2026-10-29T10:00:00Z',
    vector_state: 'partial',
    chunks_total: 4,
    chunks_ready: 3,
    chunks_failed: 1,
    embedding_model: 'text-embedding-3',
    embedding_version: 1,
    vector_error: '部分分块未能完成向量化',
    ...overrides,
  }) as KnowledgeItem

async function render(item: KnowledgeItem) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/home/knowledge/:id/chunks', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  const wrapper = mount(KnowledgeVectorPanel, {
    props: { item },
    global: { plugins: [router] },
  })
  mounted.push(wrapper)
  return { wrapper }
}

describe('knowledge vector panel retry', () => {
  it('shows the retry button only when vector state is partial', async () => {
    const partial = await render(makeItem())
    expect(partial.wrapper.find('button').exists()).toBe(true)
    expect(partial.wrapper.text()).toContain('重试向量化')

    const ready = await render(makeItem({ vector_state: 'ready', chunks_failed: 0 }))
    expect(ready.wrapper.find('button').exists()).toBe(false)

    const running = await render(makeItem({ vector_state: 'running', vector_error: '' }))
    expect(running.wrapper.find('button').exists()).toBe(false)
  })

  it('calls the API and emits retried with the updated item', async () => {
    const updated = makeItem({ vector_state: 'pending', chunks_failed: 1, vector_error: '' })
    mockRetry.mockResolvedValue(updated)
    const { wrapper } = await render(makeItem())
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(mockRetry).toHaveBeenCalledWith(10)
    const emitted = wrapper.emitted('retried')
    expect(emitted).toHaveLength(1)
    expect((emitted![0]![0] as KnowledgeItem).vector_state).toBe('pending')
  })

  it('disables the button while pending and shows inline error on failure', async () => {
    let rejectRequest!: (reason?: unknown) => void
    mockRetry.mockImplementation(
      () =>
        new Promise<KnowledgeItem>((_resolve, reject) => {
          rejectRequest = reject
        }),
    )
    const { wrapper } = await render(makeItem())
    const button = wrapper.get('button')
    await button.trigger('click')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.text()).toContain('正在重新排队')
    rejectRequest(new Error('网络异常'))
    await flushPromises()
    expect(button.attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[role="alert"]').text()).toBe('网络异常')
    expect(wrapper.emitted('retried')).toBeUndefined()
  })
})
