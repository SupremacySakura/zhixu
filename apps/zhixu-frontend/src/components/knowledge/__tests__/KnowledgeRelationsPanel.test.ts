import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeRelationsPanel from '../KnowledgeRelationsPanel.vue'
import {
  confirmKnowledgeRelation,
  createKnowledgeRelation,
  deleteKnowledgeRelation,
  listKnowledge,
  listKnowledgeRelations,
  regenerateKnowledgeRelations,
  rejectKnowledgeRelation,
  updateKnowledgeRelationType,
  type KnowledgeRelation,
  type RelationTask,
} from '../../../services/apis/knowledge'
import { RELATION_TYPE_LABELS, RELATION_TYPE_OPTIONS } from '../../../utils/relation'

vi.mock('../../../services/apis/knowledge', () => ({
  listKnowledgeRelations: vi.fn(),
  createKnowledgeRelation: vi.fn(),
  deleteKnowledgeRelation: vi.fn(),
  listKnowledge: vi.fn(),
  confirmKnowledgeRelation: vi.fn(),
  rejectKnowledgeRelation: vi.fn(),
  updateKnowledgeRelationType: vi.fn(),
  regenerateKnowledgeRelations: vi.fn(),
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
  confirmed_at: null,
  ...over,
})

const task = (over: Partial<RelationTask> = {}): RelationTask => ({
  id: 7,
  status: 'pending',
  error_message: '',
  retry_count: 0,
  updated_at: '2026-11-11T10:00:00+08:00',
  ...over,
})

const suggestion = (over: Partial<KnowledgeRelation> = {}): KnowledgeRelation =>
  relation({
    id: 43,
    status: 'suggested',
    source: 'ai',
    relation_type: 'contrasts',
    confidence: 0.72,
    evidence: JSON.stringify({
      reason: '两者从相反角度论证同一问题',
      source_quote: '本文的论证引文',
      target_quote: '对端的论证引文',
    }),
    counterpart_title: '对比知识',
    ...over,
  })

/** 在假定时器下用微任务推进组件里已 resolve 的 Promise。 */
const settle = async () => {
  for (let index = 0; index < 8; index += 1) await Promise.resolve()
}

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

/** 打开项目内的 FilterMenu（面板已不再使用原生下拉框）并点选指定文案。 */
const chooseOption = async (
  wrapper: ReturnType<typeof mountPanel>,
  label: string,
  option: string,
) => {
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

describe('relation type labels', () => {
  it('exposes the six Chinese labels and keeps their order', () => {
    expect(RELATION_TYPE_LABELS).toEqual({
      related: '相关',
      depends_on: '依赖',
      supplements: '补充说明',
      contrasts: '对比',
      updates: '更新替代',
      contradicts: '矛盾',
    })
    expect(RELATION_TYPE_OPTIONS.map(option => option.value)).toEqual([
      'related',
      'depends_on',
      'supplements',
      'contrasts',
      'updates',
      'contradicts',
    ])
  })
})

describe('KnowledgeRelationsPanel', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

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
    await chooseOption(wrapper, '关系类型', '对比')
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
describe('KnowledgeRelationsPanel suggestions', () => {
  const suggestedList = (items: KnowledgeRelation[], currentTask: RelationTask | null = null) => ({
    confirmed: [] as KnowledgeRelation[],
    suggested: items,
    task: currentTask,
  })

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads once on mount and polls every three seconds only while a task is running', async () => {
    vi.useFakeTimers()
    vi.mocked(listKnowledgeRelations).mockResolvedValue(suggestedList([], task()))
    const wrapper = mountPanel()
    await settle()
    expect(listKnowledgeRelations).toHaveBeenCalledTimes(1)
    expect(listKnowledgeRelations).toHaveBeenCalledWith(10, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('正在查找')

    await vi.advanceTimersByTimeAsync(3000)
    await settle()
    expect(listKnowledgeRelations).toHaveBeenCalledTimes(2)

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(9000)
    await settle()
    expect(listKnowledgeRelations).toHaveBeenCalledTimes(2)
  })

  it('shows AI suggestions with Chinese type, counterpart and confidence, and expands evidence', async () => {
    vi.mocked(listKnowledgeRelations).mockResolvedValue(suggestedList([suggestion()]))
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('待确认建议')
    expect(wrapper.text()).toContain('对比')
    expect(wrapper.text()).toContain('对比知识')
    expect(wrapper.text()).toContain('72%')
    expect(wrapper.text()).not.toContain('两者从相反角度论证同一问题')

    await findButton(wrapper, '查看理由')!.trigger('click')
    expect(wrapper.text()).toContain('两者从相反角度论证同一问题')
    expect(wrapper.text()).toContain('本文的论证引文')
    expect(wrapper.text()).toContain('对端的论证引文')
    // 长引文靠 line-clamp 与断词截断，避免撑破面板。
    expect(wrapper.findAll('.line-clamp-3')).toHaveLength(2)
  })

  it('confirms a suggestion and moves it into the confirmed list', async () => {
    vi.mocked(listKnowledgeRelations).mockResolvedValue(suggestedList([suggestion()]))
    vi.mocked(confirmKnowledgeRelation).mockResolvedValue(
      suggestion({ status: 'confirmed', confirmed_at: '2026-11-11T11:00:00+08:00' }),
    )
    const wrapper = mountPanel()
    await flushPromises()
    await findButton(wrapper, '确认')!.trigger('click')
    await flushPromises()
    expect(confirmKnowledgeRelation).toHaveBeenCalledWith(43)
    expect(wrapper.text()).toContain('暂无待确认建议')
    expect(wrapper.text()).toContain('对比知识')
    expect(wrapper.find('a[href="/home/knowledge/9"]').exists()).toBe(true)
  })

  it('rejects a suggestion and keeps it suppressed after regenerating', async () => {
    const list = vi.mocked(listKnowledgeRelations)
    list.mockResolvedValueOnce(suggestedList([suggestion()]))
    vi.mocked(rejectKnowledgeRelation).mockResolvedValue(suggestion({ status: 'rejected' }))
    const wrapper = mountPanel()
    await flushPromises()
    await findButton(wrapper, '忽略')!.trigger('click')
    await flushPromises()
    expect(rejectKnowledgeRelation).toHaveBeenCalledWith(43)
    expect(wrapper.text()).toContain('暂无待确认建议')

    list.mockResolvedValue(suggestedList([]))
    vi.mocked(regenerateKnowledgeRelations).mockResolvedValue({ task: task() })
    await findButton(wrapper, '查找相关知识')!.trigger('click')
    await flushPromises()
    expect(regenerateKnowledgeRelations).toHaveBeenCalledWith(10)
    expect(wrapper.text()).not.toContain('对比知识')
  })

  it('disables regenerate while running and shows progress', async () => {
    vi.mocked(listKnowledgeRelations).mockResolvedValue(
      suggestedList([], task({ status: 'running' })),
    )
    const wrapper = mountPanel()
    await flushPromises()
    const button = findButton(wrapper, '正在查找')
    expect(button?.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('正在查找')
  })

  it('shows a Chinese failure reason and retries the task', async () => {
    const list = vi.mocked(listKnowledgeRelations)
    list.mockResolvedValueOnce(
      suggestedList([], task({ status: 'failed', error_message: 'AI 服务暂时不可用' })),
    )
    list.mockResolvedValue(suggestedList([], task({ status: 'running' })))
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('AI 服务暂时不可用')
    vi.mocked(regenerateKnowledgeRelations).mockResolvedValue({ task: task() })
    await findButton(wrapper, '重试')!.trigger('click')
    await flushPromises()
    expect(regenerateKnowledgeRelations).toHaveBeenCalledWith(10)
    expect(wrapper.text()).toContain('正在查找')
  })

  it('updates the type of a confirmed relation in place', async () => {
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [relation()],
      suggested: [],
      task: null,
    })
    vi.mocked(updateKnowledgeRelationType).mockResolvedValue(relation({ relation_type: 'updates' }))
    const wrapper = mountPanel()
    await flushPromises()
    await chooseOption(wrapper, '修改与目标知识的关系类型', '更新替代')
    expect(updateKnowledgeRelationType).toHaveBeenCalledWith(41, 'updates')
    const trigger = wrapper
      .findAll('button[aria-haspopup="menu"]')
      .find(button => button.attributes('aria-label') === '修改与目标知识的关系类型')
    expect(trigger?.text()).toContain('更新替代')
    expect(wrapper.find('select').exists()).toBe(false)
  })

  it('links to the graph centered on the current knowledge', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    const link = wrapper.find('a[href="/home/knowledge/graph?center=10"]')
    expect(link.exists()).toBe(true)
    expect(link.text()).toContain('在图谱中查看')
  })
})
