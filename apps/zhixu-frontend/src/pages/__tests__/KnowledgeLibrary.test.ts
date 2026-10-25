import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeLibrary from '../KnowledgeLibrary.vue'
import { listKnowledge, deleteKnowledge } from '../../services/apis/knowledge'
vi.mock('../../services/apis/knowledge', () => ({
  listKnowledge: vi.fn(),
  addKnowledge: vi.fn(),
  getKnowledge: vi.fn(),
  retryKnowledge: vi.fn(),
  deleteKnowledge: vi.fn(),
}))
vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))
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
