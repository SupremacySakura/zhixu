import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeTagEditor from '../KnowledgeTagEditor.vue'
import { getKnowledgeTags, listTags, replaceKnowledgeTags } from '../../../services/apis/tag'
vi.mock('../../../services/apis/tag', async importOriginal => {
  const actual = await importOriginal<typeof import('../../../services/apis/tag')>()
  return {
    ...actual,
    getKnowledgeTags: vi.fn(),
    listTags: vi.fn(),
    replaceKnowledgeTags: vi.fn(),
  }
})
const tag = (id: number, name: string, usage_count = 0) => ({ id, name, usage_count })
const mountEditor = () => mount(KnowledgeTagEditor, { props: { knowledgeId: 10 } })
describe('KnowledgeTagEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getKnowledgeTags).mockResolvedValue({
      items: [tag(1, '工作')],
      total: 1,
    })
    vi.mocked(listTags).mockResolvedValue({
      items: [tag(1, '工作', 2), tag(2, '学习', 1), tag(3, '随笔')],
      total: 3,
    })
  })
  it('loads bound tags and existing tag suggestions', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    expect(getKnowledgeTags).toHaveBeenCalledWith(10)
    expect(listTags).toHaveBeenCalled()
    const text = wrapper.text()
    expect(text).toContain('工作')
    expect(text).toContain('学习')
    expect(text).toContain('随笔')
    // 已绑定标签不再出现在联想里，避免重复添加。
    const suggestionButtons = wrapper
      .findAll('button')
      .filter(button => ['学习', '随笔'].some(name => button.text().includes(name)))
    expect(suggestionButtons).toHaveLength(2)
  })
  it('adds a tag from suggestions and saves the full set', async () => {
    vi.mocked(replaceKnowledgeTags).mockResolvedValue({
      items: [tag(1, '工作'), tag(2, '学习')],
      total: 2,
    })
    const wrapper = mountEditor()
    await flushPromises()
    const suggestion = wrapper.findAll('button').find(button => button.text().includes('学习'))
    await suggestion!.trigger('click')
    const save = wrapper.findAll('button').find(button => button.text().includes('保存标签'))
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()
    expect(replaceKnowledgeTags).toHaveBeenCalledWith(10, ['工作', '学习'])
    expect(wrapper.emitted('saved')?.[0]?.[0]).toHaveLength(2)
  })
  it('removes a tag and saves without it', async () => {
    vi.mocked(replaceKnowledgeTags).mockResolvedValue({ items: [], total: 0 })
    const wrapper = mountEditor()
    await flushPromises()
    const remove = wrapper.find('button[aria-label="移除标签 工作"]')
    await remove.trigger('click')
    const save = wrapper.findAll('button').find(button => button.text().includes('保存标签'))
    await save!.trigger('click')
    await flushPromises()
    expect(replaceKnowledgeTags).toHaveBeenCalledWith(10, [])
  })
  it('adds a typed tag on enter and reports save failures', async () => {
    vi.mocked(replaceKnowledgeTags).mockRejectedValue(new Error('标签服务暂时不可用'))
    const wrapper = mountEditor()
    await flushPromises()
    const input = wrapper.find('input')
    await input.setValue('新标签')
    await input.trigger('keydown', { key: 'Enter' })
    const save = wrapper.findAll('button').find(button => button.text().includes('保存标签'))
    await save!.trigger('click')
    await flushPromises()
    expect(replaceKnowledgeTags).toHaveBeenCalledWith(10, ['工作', '新标签'])
    expect(wrapper.text()).toContain('标签服务暂时不可用')
  })
})
