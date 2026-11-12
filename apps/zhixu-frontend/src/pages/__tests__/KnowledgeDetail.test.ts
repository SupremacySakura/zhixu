import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeDetail from '../KnowledgeDetail.vue'
import {
  archiveKnowledge,
  favoriteKnowledge,
  getKnowledge,
  regenerateKnowledgeEvolve,
  unarchiveKnowledge,
} from '../../services/apis/knowledge'

vi.mock('../../services/apis/knowledge', () => ({
  getKnowledge: vi.fn(),
  favoriteKnowledge: vi.fn(),
  archiveKnowledge: vi.fn(),
  unarchiveKnowledge: vi.fn(),
  getKnowledgeOrganize: vi.fn(),
  acceptKnowledgeOrganize: vi.fn(),
  regenerateKnowledgeOrganize: vi.fn(),
  regenerateKnowledgeEvolve: vi.fn(),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const baseItem = {
  id: 10,
  source_file_id: null,
  source_file_name: '',
  source_folder_id: 0,
  source_folder_name: '',
  source_type: 'manual',
  title: '手写笔记',
  content: '# 正文',
  summary: '',
  keywords: [],
  version: 2,
  is_favorite: false,
  archived_at: null,
  outdated_at: null,
  outdated_reason: '',
  status: 'ready',
  error_code: '',
  error_message: '',
  imported_at: '2026-11-10T10:00:00+08:00',
  created_at: '2026-11-10T10:00:00+08:00',
  updated_at: '2026-11-10T10:00:00+08:00',
  vector_state: 'ready',
  chunks_total: 1,
  chunks_ready: 1,
  chunks_failed: 0,
  embedding_model: 'test',
  embedding_version: 1,
  vector_error: '',
}

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home/knowledge', component: { template: '<div />' } },
      { path: '/home/knowledge/:id', component: KnowledgeDetail },
      { path: '/home/knowledge/:id/edit', component: { template: '<div />' } },
      { path: '/home/files', component: { template: '<div />' } },
    ],
  })
  await router.push('/home/knowledge/10')
  await router.isReady()
  const wrapper = mount(KnowledgeDetail, {
    global: {
      plugins: [router],
      stubs: {
        KnowledgeRelationsPanel: true,
        KnowledgeOrganizePanel: true,
        KnowledgeVectorPanel: true,
        KnowledgeTagEditor: true,
        KnowledgeHistoryDialog: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

afterEach(() => {
  vi.clearAllMocks()
})

describe('KnowledgeDetail', () => {
  it('toggles favorite state', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(favoriteKnowledge).mockResolvedValue({ id: 10, is_favorite: true })
    const wrapper = await render()
    const button = wrapper.findAll('button').find(entry => entry.text().includes('收藏'))
    expect(button).toBeTruthy()
    await button!.trigger('click')
    await flushPromises()
    expect(favoriteKnowledge).toHaveBeenCalledWith(10, true)
    expect(wrapper.text()).toContain('已收藏')
  })

  it('shows related knowledge above the AI organize panel', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    const wrapper = await render()
    const html = wrapper.html()
    const relationsPanel = html.indexOf('knowledge-relations-panel-stub')
    const organizePanel = html.indexOf('knowledge-organize-panel-stub')
    expect(relationsPanel).toBeGreaterThan(-1)
    expect(relationsPanel).toBeLessThan(organizePanel)
  })

  it('shows the outdated badge with its reason', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({
      ...baseItem,
      outdated_at: '2026-11-12T09:00:00+08:00',
      outdated_reason: '新导入的《Vue 3.5 说明》覆盖了同样的 API 用法。',
    } as never)
    const wrapper = await render()
    expect(wrapper.text()).toContain('这条知识可能已过时')
    expect(wrapper.text()).toContain('新导入的《Vue 3.5 说明》覆盖了同样的 API 用法。')
    expect(wrapper.text()).toContain('判定时间')
  })

  it('hides the outdated badge for verified knowledge', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    const wrapper = await render()
    expect(wrapper.text()).not.toContain('这条知识可能已过时')
  })

  it('submits a recheck and reports the task state', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(regenerateKnowledgeEvolve).mockResolvedValue({
      id: 5,
      task_type: 'evolve',
      status: 'pending',
      error_message: '',
      retry_count: 0,
      updated_at: '2026-11-12T09:00:00+08:00',
    })
    const wrapper = await render()
    const button = wrapper.findAll('button').find(entry => entry.text().includes('重新检查'))
    expect(button).toBeTruthy()
    await button!.trigger('click')
    await flushPromises()
    expect(regenerateKnowledgeEvolve).toHaveBeenCalledWith(10)
    expect(wrapper.text()).toContain('已提交演化检查')
  })

  it('archives and restores knowledge', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(archiveKnowledge).mockResolvedValue({
      id: 10,
      archived: true,
      archived_at: '2026-11-10T12:00:00+08:00',
    })
    const wrapper = await render()
    const archiveButton = wrapper.findAll('button').find(entry => entry.text() === '归档')
    expect(archiveButton).toBeTruthy()
    await archiveButton!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('该知识已归档，不会出现在默认列表与问答检索中')
    vi.mocked(unarchiveKnowledge).mockResolvedValue({ id: 10, archived: false, archived_at: null })
    const restoreButton = wrapper.findAll('button').find(entry => entry.text() === '恢复归档')
    expect(restoreButton).toBeTruthy()
    await restoreButton!.trigger('click')
    await flushPromises()
    expect(unarchiveKnowledge).toHaveBeenCalledWith(10)
    expect(wrapper.text()).not.toContain('该知识已归档，不会出现在默认列表与问答检索中')
  })
})
