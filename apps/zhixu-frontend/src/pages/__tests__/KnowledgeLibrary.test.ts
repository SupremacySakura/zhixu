import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeLibrary from '../KnowledgeLibrary.vue'
import {
  favoriteKnowledge,
  listKnowledge,
  deleteKnowledge,
  retryKnowledgeVectorize,
  unarchiveKnowledge,
  type KnowledgeItem,
} from '../../services/apis/knowledge'
import KnowledgeActionMenu from '../../components/knowledge/KnowledgeActionMenu.vue'
vi.mock('../../services/apis/knowledge', () => ({
  listKnowledge: vi.fn(),
  addKnowledge: vi.fn(),
  getKnowledge: vi.fn(),
  retryKnowledge: vi.fn(),
  retryKnowledgeVectorize: vi.fn(),
  deleteKnowledge: vi.fn(),
  favoriteKnowledge: vi.fn(),
  archiveKnowledge: vi.fn(),
  unarchiveKnowledge: vi.fn(),
}))
vi.mock('../../services/apis/tag', () => ({
  listTags: vi.fn().mockResolvedValue({ items: [] }),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))
afterEach(() => {
  vi.clearAllMocks()
})
async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: KnowledgeLibrary }],
  })
  await router.push('/')
  await router.isReady()
  const wrapper = mount(KnowledgeLibrary, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}
describe('KnowledgeLibrary', () => {
  it('toggles favorite from the list row', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [
        {
          id: 10,
          title: '笔记',
          status: 'ready',
          created_at: '',
          is_favorite: false,
          archived_at: null,
          source_file_id: null,
          source_file_name: '',
        } as never,
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(favoriteKnowledge).mockResolvedValue({ id: 10, is_favorite: true })
    const wrapper = await render()
    await wrapper.get('[aria-label="收藏 笔记"]').trigger('click')
    await flushPromises()
    expect(favoriteKnowledge).toHaveBeenCalledWith(10, true)
    expect(wrapper.find('[aria-label="取消收藏 笔记"]').exists()).toBe(true)
  })

  it('sends archive and favorite filters to the list request', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    const wrapper = await render()
    await wrapper.get('[aria-label="筛选归档"]').trigger('click')
    const archivedOption = wrapper
      .findAll('[role="menuitemradio"]')
      .find(option => option.text() === '仅已归档')!
    await archivedOption.trigger('click')
    await flushPromises()
    expect(listKnowledge).toHaveBeenLastCalledWith(
      1,
      '',
      '',
      '',
      expect.objectContaining({ archived: true, sort: 'created' }),
      expect.any(AbortSignal),
    )
    await wrapper.get('[aria-label="筛选收藏"]').trigger('click')
    const favoriteOption = wrapper
      .findAll('[role="menuitemradio"]')
      .find(option => option.text() === '仅收藏')!
    await favoriteOption.trigger('click')
    await flushPromises()
    expect(listKnowledge).toHaveBeenLastCalledWith(
      1,
      '',
      '',
      '',
      expect.objectContaining({ archived: true, favorite: true }),
      expect.any(AbortSignal),
    )
  })

  it('restores an archived knowledge from the menu', async () => {
    const archivedItem = {
      id: 12,
      title: '旧笔记',
      status: 'ready',
      source_type: 'manual',
      source_file_id: null,
      source_file_name: '',
      created_at: '',
      archived_at: '2026-11-10T12:00:00+08:00',
    } as KnowledgeItem
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [archivedItem],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(unarchiveKnowledge).mockResolvedValue({ id: 12, archived: false, archived_at: null })
    const wrapper = await render()
    await wrapper.get('[aria-label="更多操作 旧笔记"]').trigger('click')
    await flushPromises()
    const restore = document.querySelector<HTMLButtonElement>('[aria-label="恢复归档 旧笔记"]')!
    expect(restore).toBeTruthy()
    restore.click()
    await flushPromises()
    expect(unarchiveKnowledge).toHaveBeenCalledWith(12)
    expect(wrapper.text()).toContain('已恢复归档')
  })

  it('uses a custom menu and deletes knowledge without deleting the source', async () => {
    vi.mocked(listKnowledge)
      .mockResolvedValueOnce({
        items: [
          {
            id: 10,
            title: '笔记',
            status: 'ready',
            created_at: '',
            source_file_name: 'note.md',
          } as never,
        ],
        total: 1,
        page: 1,
        page_size: 20,
      })
      .mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    vi.mocked(deleteKnowledge).mockResolvedValue()
    const wrapper = await render()
    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('[aria-label="筛选处理状态"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    await wrapper.get('[aria-label="更多操作 笔记"]').trigger('click')
    await flushPromises()
    const deleteAction = document.querySelector<HTMLButtonElement>('[aria-label="删除知识 笔记"]')!
    expect(deleteKnowledge).not.toHaveBeenCalled()
    deleteAction.click()
    await flushPromises()
    expect(document.body.textContent).toContain('原文件保持不变')
    const dialog = document.querySelector('[role="dialog"]')!
    const button = Array.from(dialog.querySelectorAll('button')).find(
      button => button.textContent?.trim() === '删除知识',
    )!
    button.click()
    await flushPromises()
    expect(deleteKnowledge).toHaveBeenCalledWith(10, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('让第一份资料成为知识')
    wrapper.unmount()
  })
  it('retries vector chunks from the menu and refreshes the list', async () => {
    const partial = {
      id: 11,
      title: '部分失败笔记',
      status: 'ready',
      vector_state: 'partial',
      chunks_total: 4,
      chunks_ready: 3,
      chunks_failed: 1,
      source_file_name: 'note.md',
      created_at: '',
    } as KnowledgeItem
    const updated = { ...partial, vector_state: 'pending' as const, chunks_failed: 0 }
    vi.mocked(listKnowledge)
      .mockResolvedValueOnce({ items: [partial], total: 1, page: 1, page_size: 20 })
      .mockResolvedValue({ items: [updated], total: 1, page: 1, page_size: 20 })
    vi.mocked(retryKnowledgeVectorize).mockResolvedValueOnce(updated)
    const wrapper = await render()
    try {
      await wrapper.get('[aria-label="更多操作 部分失败笔记"]').trigger('click')
      await flushPromises()
      document.querySelector<HTMLButtonElement>('[aria-label="重试向量化 部分失败笔记"]')!.click()
      await flushPromises()
      expect(retryKnowledgeVectorize).toHaveBeenCalledWith(11, expect.any(AbortSignal))
      expect(listKnowledge).toHaveBeenCalledTimes(2)
      expect(wrapper.text()).toContain('失败分块已重新排队')
      expect(wrapper.findComponent(KnowledgeActionMenu).props('item').vector_state).toBe('pending')
    } finally {
      wrapper.unmount()
    }
  })
  it('shows vector retry errors without refreshing the list', async () => {
    const partial = {
      id: 11,
      title: '部分失败笔记',
      status: 'ready',
      vector_state: 'partial',
      chunks_total: 4,
      created_at: '',
      source_file_name: 'note.md',
    } as KnowledgeItem
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [partial],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(retryKnowledgeVectorize).mockRejectedValueOnce(new Error('重试失败'))
    const wrapper = await render()
    try {
      wrapper.findComponent(KnowledgeActionMenu).vm.$emit('retryVector', partial)
      await flushPromises()
      expect(wrapper.text()).toContain('重试失败')
      expect(listKnowledge).toHaveBeenCalledTimes(1)
    } finally {
      wrapper.unmount()
    }
  })
  it('guards duplicate retries and aborts the request on unmount', async () => {
    const partial = {
      id: 11,
      title: '部分失败笔记',
      status: 'ready',
      vector_state: 'partial',
      chunks_total: 4,
      created_at: '',
      source_file_name: 'note.md',
    } as KnowledgeItem
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [partial],
      total: 1,
      page: 1,
      page_size: 20,
    })
    let resolveRequest!: (item: KnowledgeItem) => void
    vi.mocked(retryKnowledgeVectorize).mockImplementationOnce(
      () =>
        new Promise(resolve => {
          resolveRequest = resolve
        }),
    )
    const wrapper = await render()
    const actionMenu = wrapper.findComponent(KnowledgeActionMenu)
    actionMenu.vm.$emit('retryVector', partial)
    actionMenu.vm.$emit('retryVector', partial)
    expect(retryKnowledgeVectorize).toHaveBeenCalledTimes(1)
    const signal = vi.mocked(retryKnowledgeVectorize).mock.calls[0]![1]!
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    resolveRequest(partial)
    await flushPromises()
    expect(listKnowledge).toHaveBeenCalledTimes(1)
  })
  it('shows an actionable empty state', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    const wrapper = await render()
    expect(wrapper.text()).toContain('让第一份资料成为知识')
    expect(wrapper.find('a[href="/home/files"]').exists()).toBe(true)
    wrapper.unmount()
  })
  it('shows persisted failures rather than invented summaries', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [
        {
          id: 1,
          title: '空笔记',
          status: 'failed',
          error_message: '原文件为空',
          source_file_name: 'empty.md',
          created_at: '2026-10-19T12:00:00Z',
        } as never,
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    const wrapper = await render()
    expect(wrapper.text()).toContain('原文件为空')
    await wrapper.get('[aria-label="更多操作 空笔记"]').trigger('click')
    await flushPromises()
    expect(
      document.querySelector('[role="menu"][aria-label="知识操作 空笔记"]')?.textContent,
    ).toContain('查看失败原因')
    wrapper.unmount()
  })
})
