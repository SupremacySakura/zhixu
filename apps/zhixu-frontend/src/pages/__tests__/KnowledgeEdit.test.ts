import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeEdit from '../KnowledgeEdit.vue'
import { createKnowledge, getKnowledge, updateKnowledge } from '../../services/apis/knowledge'
vi.mock('../../services/apis/knowledge', () => ({
  createKnowledge: vi.fn(),
  getKnowledge: vi.fn(),
  updateKnowledge: vi.fn(),
}))
afterEach(() => {
  vi.clearAllMocks()
})
async function render(path = '/') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: KnowledgeEdit },
      { path: '/home/knowledge', component: { template: '<div />' } },
      { path: '/home/knowledge/new', component: KnowledgeEdit },
      { path: '/home/knowledge/:id/edit', component: KnowledgeEdit },
      { path: '/home/knowledge/:id', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(KnowledgeEdit, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}
describe('KnowledgeEdit', () => {
  it('saves the trimmed title and opens the new detail page', async () => {
    vi.mocked(createKnowledge).mockResolvedValue({ id: 11 } as never)
    const { wrapper, router } = await render()
    await wrapper.get('#knowledge-title').setValue('  手写笔记  ')
    await wrapper.get('#knowledge-content').setValue('  # 正文\n内容  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(createKnowledge).toHaveBeenCalledWith(
      '手写笔记',
      '  # 正文\n内容  ',
      expect.any(AbortSignal),
    )
    expect(router.currentRoute.value.path).toBe('/home/knowledge/11')
    wrapper.unmount()
  })
  it('disables saving until title and content are present', async () => {
    const { wrapper } = await render()
    const button = wrapper.get('button[type="submit"]')
    expect(button.attributes('disabled')).toBeDefined()
    await wrapper.get('#knowledge-title').setValue('标题')
    expect(button.attributes('disabled')).toBeDefined()
    await wrapper.get('#knowledge-content').setValue('正文')
    expect(button.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('shows request errors without navigating away', async () => {
    vi.mocked(createKnowledge).mockRejectedValue(new Error('保存失败'))
    const { wrapper, router } = await render()
    await wrapper.get('#knowledge-title').setValue('标题')
    await wrapper.get('#knowledge-content').setValue('正文')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('保存失败')
    expect(router.currentRoute.value.path).toBe('/')
    wrapper.unmount()
  })
  it('loads an existing item and saves with the base version', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({
      id: 7,
      title: '旧标题',
      content: '旧正文',
      version: 3,
      status: 'ready',
    } as never)
    vi.mocked(updateKnowledge).mockResolvedValue({ id: 7 } as never)
    const { wrapper, router } = await render('/home/knowledge/7/edit')
    expect(getKnowledge).toHaveBeenCalledWith(7, expect.any(AbortSignal))
    expect((wrapper.get('#knowledge-title').element as HTMLInputElement).value).toBe('旧标题')
    expect((wrapper.get('#knowledge-content').element as HTMLTextAreaElement).value).toBe('旧正文')
    await wrapper.get('#knowledge-title').setValue('新标题')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(updateKnowledge).toHaveBeenCalledWith(7, '新标题', '旧正文', 3, expect.any(AbortSignal))
    expect(router.currentRoute.value.path).toBe('/home/knowledge/7')
    wrapper.unmount()
  })
  it('keeps the editor open on a version conflict', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({
      id: 7,
      title: '旧标题',
      content: '旧正文',
      version: 3,
      status: 'ready',
    } as never)
    vi.mocked(updateKnowledge).mockRejectedValue({
      isAxiosError: true,
      response: { status: 409, data: { msg: '当前状态不支持该操作' } },
    })
    const { wrapper, router } = await render('/home/knowledge/7/edit')
    await wrapper.get('#knowledge-title').setValue('新标题')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('请刷新后重新编辑')
    expect(router.currentRoute.value.path).toBe('/home/knowledge/7/edit')
    wrapper.unmount()
  })
})
