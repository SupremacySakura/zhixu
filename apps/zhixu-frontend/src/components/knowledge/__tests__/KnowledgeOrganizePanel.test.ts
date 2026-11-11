import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeOrganizePanel from '../KnowledgeOrganizePanel.vue'
import {
  acceptKnowledgeOrganize,
  getKnowledgeOrganize,
  regenerateKnowledgeOrganize,
  type KnowledgeItem,
  type KnowledgeOrganizeDraft,
  type KnowledgeOrganizeState,
} from '../../../services/apis/knowledge'

vi.mock('../../../services/apis/knowledge', async importOriginal => {
  const mod = await importOriginal<typeof import('../../../services/apis/knowledge')>()
  return {
    ...mod,
    getKnowledgeOrganize: vi.fn(),
    acceptKnowledgeOrganize: vi.fn(),
    regenerateKnowledgeOrganize: vi.fn(),
  }
})

const mockGet = vi.mocked(getKnowledgeOrganize)
const mockAccept = vi.mocked(acceptKnowledgeOrganize)
const mockRegenerate = vi.mocked(regenerateKnowledgeOrganize)

const mounted: ReturnType<typeof mount>[] = []
afterEach(() => {
  mounted.forEach(wrapper => wrapper.unmount())
  mounted.length = 0
  vi.clearAllMocks()
  vi.useRealTimers()
})

const makeItem = (overrides: Partial<KnowledgeItem> = {}): KnowledgeItem =>
  ({
    id: 10,
    source_file_id: null,
    source_file_name: '',
    source_folder_id: 0,
    source_folder_name: '',
    source_type: 'manual',
    source_url: '',
    title: '知识分块与检索',
    content: '正文',
    summary: '',
    keywords: [],
    version: 3,
    is_favorite: false,
    archived_at: null,
    status: 'ready',
    error_code: '',
    error_message: '',
    imported_at: '2026-10-29T10:00:00Z',
    created_at: '2026-10-29T10:00:00Z',
    updated_at: '2026-10-29T10:00:00Z',
    vector_state: 'ready',
    chunks_total: 1,
    chunks_ready: 1,
    chunks_failed: 0,
    embedding_model: 'text-embedding-3',
    embedding_version: 1,
    vector_error: '',
    ...overrides,
  }) as KnowledgeItem

const makeDraft = (overrides: Partial<KnowledgeOrganizeDraft> = {}): KnowledgeOrganizeDraft => ({
  summary: '一篇讲分块与检索的笔记。',
  keywords: ['分块', '检索'],
  suggested_tags: [
    { name: '检索增强', tag_id: 4, existing: true },
    { name: '新标签', tag_id: 0, existing: false },
  ],
  concepts: ['RAG'],
  accepted: false,
  accepted_at: null,
  stale: false,
  base_version: 3,
  ...overrides,
})

const makeState = (
  status: 'pending' | 'running' | 'succeeded' | 'failed',
  draft: KnowledgeOrganizeDraft | null,
  errorMessage = '',
): KnowledgeOrganizeState => ({
  task: {
    id: 1,
    task_type: 'organize',
    status,
    error_message: errorMessage,
    retry_count: 0,
    updated_at: '2026-11-11T10:00:00Z',
  },
  draft,
})

async function render(item: KnowledgeItem = makeItem()) {
  const wrapper = mount(KnowledgeOrganizePanel, { props: { item } })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('knowledge organize panel', () => {
  it('offers to generate when the knowledge has no task yet', async () => {
    mockGet.mockResolvedValue({ task: null, draft: null })
    const wrapper = await render()
    expect(wrapper.text()).toContain('还没有整理结果')
    expect(wrapper.text()).toContain('生成整理结果')
    expect(mockGet).toHaveBeenCalledWith(10, expect.any(AbortSignal))
  })

  it('renders the draft for preview before anything is accepted', async () => {
    mockGet.mockResolvedValue(makeState('succeeded', makeDraft()))
    const wrapper = await render()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe(
      '一篇讲分块与检索的笔记。',
    )
    expect(wrapper.text()).toContain('分块')
    expect(wrapper.text()).toContain('检索增强')
    expect(wrapper.text()).toContain('已有')
    expect(wrapper.text()).toContain('新标签')
    expect(wrapper.text()).toContain('采纳整理结果')
  })

  it('submits edited content and splits tag ids from new tag names', async () => {
    mockGet.mockResolvedValue(makeState('succeeded', makeDraft()))
    const updated = makeItem({ summary: '改后的摘要', keywords: ['分块'] })
    mockAccept.mockResolvedValue({
      item: updated,
      tags: [],
      draft: makeDraft({ summary: '改后的摘要', keywords: ['分块'], accepted: true }),
    })
    const wrapper = await render()
    await wrapper.get('textarea').setValue('改后的摘要')
    const acceptButton = wrapper
      .findAll('button')
      .find(entry => entry.text().includes('采纳整理结果'))
    expect(acceptButton).toBeTruthy()
    await acceptButton!.trigger('click')
    await flushPromises()
    expect(mockAccept).toHaveBeenCalledWith(10, {
      summary: '改后的摘要',
      keywords: ['分块', '检索'],
      tag_ids: [4],
      tag_names: ['新标签'],
    })
    expect(wrapper.emitted('accepted')?.[0]?.[0]).toMatchObject({ id: 10, summary: '改后的摘要' })
    expect(wrapper.text()).toContain('已采纳')
  })

  it('shows the failure reason and allows a retry', async () => {
    mockGet.mockResolvedValue(makeState('failed', null, '未配置可用的对话模型，请前往模型设置'))
    const wrapper = await render()
    expect(wrapper.text()).toContain('未配置可用的对话模型，请前往模型设置')
    mockRegenerate.mockResolvedValue(makeState('pending', null))
    const retry = wrapper.findAll('button').find(entry => entry.text().includes('重新生成'))
    expect(retry).toBeTruthy()
    await retry!.trigger('click')
    await flushPromises()
    expect(mockRegenerate).toHaveBeenCalledWith(10)
    expect(wrapper.text()).toContain('排队中')
  })

  it('polls while the task is queued and stops once it settles', async () => {
    vi.useFakeTimers()
    mockGet
      .mockResolvedValueOnce(makeState('pending', null))
      .mockResolvedValueOnce(makeState('succeeded', makeDraft()))
    const wrapper = mount(KnowledgeOrganizePanel, { props: { item: makeItem() } })
    mounted.push(wrapper)
    await vi.advanceTimersByTimeAsync(0)
    expect(wrapper.text()).toContain('排队中')
    expect(mockGet).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3000)
    expect(mockGet).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('采纳整理结果')
    await vi.advanceTimersByTimeAsync(9000)
    expect(mockGet).toHaveBeenCalledTimes(2)
  })
})
