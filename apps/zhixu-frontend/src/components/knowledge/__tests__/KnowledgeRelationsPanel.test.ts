import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeRelationsPanel from '../KnowledgeRelationsPanel.vue'
import {
  createKnowledgeRelation,
  deleteKnowledgeRelation,
  listKnowledge,
  listKnowledgeRelations,
  type KnowledgeRelation,
} from '../../../services/apis/knowledge'
import { RELATION_TYPE_LABELS, RELATION_TYPE_OPTIONS } from '../../../utils/relation'

vi.mock('../../../services/apis/knowledge', () => ({
  listKnowledgeRelations: vi.fn(),
  createKnowledgeRelation: vi.fn(),
  deleteKnowledgeRelation: vi.fn(),
  listKnowledge: vi.fn(),
}))

const current = {
  id: 10,
  title: '当前知识',
  status: 'ready',
  archived_at: null,
} as never

const relation = (over: Partial<KnowledgeRelation> = {}): KnowledgeRelation => ({
  id: 41,
  source_id: 10,
  target_id: 9,
  relation_type: 'depends_on',
  status: 'confirmed',
  source: 'manual',
  confidence: 0,
  evidence: '',
  counterpart_id: 9,
  counterpart_title: '目标知识',
  created_at: '2026-11-11T10:00:00+08:00',
  ...over,
})

const mountPanel = () =>
  mount(KnowledgeRelationsPanel, {
    props: { item: current },
    global: {
      stubs: {
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      },
    },
  })

const findButton = (wrapper: ReturnType<typeof mountPanel>, label: string) =>
  wrapper.findAll('button').find(button => button.text().includes(label))

describe('relation type labels', () => {
  it('exposes the five Chinese labels and keeps their order', () => {
    expect(RELATION_TYPE_LABELS).toEqual({
      related: '相关',
      depends_on: '依赖',
      supplements: '补充说明',
      contrasts: '对比',
      updates: '更新替代',
    })
    expect(RELATION_TYPE_OPTIONS.map(option => option.value)).toEqual([
      'related',
      'depends_on',
      'supplements',
      'contrasts',
      'updates',
    ])
  })
})

describe('KnowledgeRelationsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [relation()],
      suggested: [],
      task: null,
    })
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [
        { id: 11, title: '候选知识', archived_at: null },
        { id: 10, title: '当前知识', archived_at: null },
        { id: 12, title: '已归档知识', archived_at: '2026-11-01T00:00:00+08:00' },
      ],
      total: 3,
      page: 1,
      page_size: 20,
    } as never)
  })

  it('loads confirmed relations with Chinese labels and counterpart links', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect(vi.mocked(listKnowledgeRelations).mock.calls[0][0]).toBe(10)
    expect(wrapper.text()).toContain('依赖')
    expect(wrapper.text()).toContain('目标知识')
    const link = wrapper.find('a[href="/home/knowledge/9"]')
    expect(link.exists()).toBe(true)
  })

  it('shows a guide when there is no relation yet', async () => {
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [],
      suggested: [],
      task: null,
    })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('还没有建立关系')
  })

  it('searches knowledge by title and creates a manual relation', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    const input = wrapper.find('input[placeholder="搜索知识标题"]')
    await input.setValue('候选')
    await input.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(vi.mocked(listKnowledge).mock.calls[0][1]).toBe('候选')
    // 搜索只展示可添加的对端：排除自身、已归档与已经建立关系的知识。
    const options = wrapper.findAll('button').filter(button => button.text().includes('候选知识'))
    expect(options).toHaveLength(1)
    expect(wrapper.text()).not.toContain('已归档知识')
    expect(wrapper.text()).not.toContain('当前知识')
    await options[0].trigger('click')
    await wrapper.find('select').setValue('contrasts')
    vi.mocked(createKnowledgeRelation).mockResolvedValue(
      relation({
        id: 42,
        relation_type: 'contrasts',
        counterpart_id: 11,
        counterpart_title: '候选知识',
      }),
    )
    await findButton(wrapper, '添加关系')!.trigger('click')
    await flushPromises()
    expect(createKnowledgeRelation).toHaveBeenCalledWith(10, {
      target_id: 11,
      relation_type: 'contrasts',
    })
    expect(wrapper.text()).toContain('对比')
    expect(wrapper.text()).toContain('目标知识')
  })

  it('asks for confirmation before deleting a relation', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.find('button[aria-label="删除与目标知识的关系"]').trigger('click')
    expect(deleteKnowledgeRelation).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('确认删除')
    vi.mocked(deleteKnowledgeRelation).mockResolvedValue(undefined)
    await findButton(wrapper, '确认删除')!.trigger('click')
    await flushPromises()
    expect(deleteKnowledgeRelation).toHaveBeenCalledWith(41)
    expect(wrapper.text()).not.toContain('目标知识')
  })

  it('reports create failures and keeps the form usable', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    const input = wrapper.find('input[placeholder="搜索知识标题"]')
    await input.setValue('候选')
    await input.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    const option = wrapper.findAll('button').find(button => button.text().includes('候选知识'))
    await option!.trigger('click')
    vi.mocked(createKnowledgeRelation).mockRejectedValue(new Error('关系类型无效'))
    await findButton(wrapper, '添加关系')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('关系类型无效')
  })
})
