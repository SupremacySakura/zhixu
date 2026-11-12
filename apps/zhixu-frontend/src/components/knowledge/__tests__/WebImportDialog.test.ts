import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WebImportDialog from '../WebImportDialog.vue'
import { importKnowledgeFromWeb } from '../../../services/apis/knowledge'
vi.mock('../../ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('../../../services/apis/knowledge', async importOriginal => {
  const actual = await importOriginal<typeof import('../../../services/apis/knowledge')>()
  return { ...actual, importKnowledgeFromWeb: vi.fn() }
})
const created = { id: 12, title: '网页文章', source_type: 'web' } as never
const mountDialog = () => mount(WebImportDialog, { attachTo: document.body })
const buttonByText = (text: string) =>
  [...document.body.querySelectorAll('button')].find(b => b.textContent?.includes(text))
async function typeUrl(value: string) {
  const input = document.body.querySelector('input') as HTMLInputElement
  input.value = value
  await input.dispatchEvent(new Event('input'))
}
describe('WebImportDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  it('imports a web page and emits the created knowledge', async () => {
    vi.mocked(importKnowledgeFromWeb).mockResolvedValue(created)
    const wrapper = mountDialog()
    await flushPromises()
    expect(document.body.textContent).toContain('从网页导入')
    expect(buttonByText('开始导入')?.disabled).toBe(true)
    await typeUrl('  https://example.com/post  ')
    await flushPromises()
    buttonByText('开始导入')!.click()
    await flushPromises()
    expect(importKnowledgeFromWeb).toHaveBeenCalledWith('https://example.com/post')
    expect(wrapper.emitted('imported')?.[0]).toEqual([created])
    wrapper.unmount()
  })
  it('shows the failure message and stays open', async () => {
    vi.mocked(importKnowledgeFromWeb).mockRejectedValue(new Error('网页链接不可访问'))
    const wrapper = mountDialog()
    await flushPromises()
    await typeUrl('http://127.0.0.1/private')
    await flushPromises()
    buttonByText('开始导入')!.click()
    await flushPromises()
    expect(document.body.textContent).toContain('网页链接不可访问')
    expect(wrapper.emitted('imported')).toBeUndefined()
    wrapper.unmount()
  })
})
