/**
 * 分块结构页的契约（#41 批 4B）
 *
 * 这一页此前零测试，而它承载两个容易悄悄坏掉的东西：
 * 1. `?chunk=N` 深链 —— 引用链接、向量面板的「打开分块结构」都靠它定位并高亮；
 * 2. 长文阅读语法 —— 正文必须是 16px（`text-read`）。R5 记着：`max-w-measure`
 *    按元素自身字号算，挂 13px 就只有 520px，一行不再是 40 个汉字。
 *    happy-dom 不做布局，量不出像素，所以这里钉的是「承载宽度的那个类」。
 */
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeChunks from '../KnowledgeChunks.vue'
import {
  getKnowledge,
  getKnowledgeChunks,
  type KnowledgeChunk,
} from '../../services/apis/knowledge'

vi.mock('../../services/apis/knowledge', () => ({
  getKnowledge: vi.fn(),
  getKnowledgeChunks: vi.fn(),
}))

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
  chunks_total: 3,
  chunks_ready: 3,
  chunks_failed: 0,
  embedding_model: 'test',
  embedding_version: 1,
  vector_error: '',
}

function chunk(seq: number, section: string, extra: Partial<KnowledgeChunk> = {}): KnowledgeChunk {
  return {
    id: seq,
    seq,
    section,
    section_level: 2,
    chars: 10,
    start_char: seq * 10,
    end_char: seq * 10 + 10,
    state: 'ready',
    error_message: '',
    content: `第 ${seq} 块正文`,
    overlap: false,
    overlap_chars: 0,
    ...extra,
  }
}

async function render(path = '/home/knowledge/10/chunks', attach = false) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home/knowledge', component: { template: '<div />' } },
      { path: '/home/knowledge/:id', component: { template: '<div />' } },
      { path: '/home/knowledge/:id/chunks', component: KnowledgeChunks },
    ],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(KnowledgeChunks, {
    global: { plugins: [router], stubs: { KnowledgeVectorPanel: true } },
    ...(attach ? { attachTo: document.body } : {}),
  })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  // happy-dom 没有实现滚动；深链定位只关心「滚到了哪一块」。
  Element.prototype.scrollIntoView = vi.fn()
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('KnowledgeChunks', () => {
  it('highlights and scrolls to the chunk named by ?chunk=N', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(getKnowledgeChunks).mockResolvedValue({
      items: [chunk(1, '开头'), chunk(2, '中间'), chunk(3, '结尾')],
      total: 3,
    })
    const wrapper = await render('/home/knowledge/10/chunks?chunk=2', true)
    const target = wrapper.get('#chunk-2')
    expect(target.classes()).toContain('ring-primary')
    expect(wrapper.get('#chunk-1').classes()).not.toContain('ring-primary')
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('filters down to failed chunks only', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(getKnowledgeChunks).mockResolvedValue({
      items: [chunk(1, '开头'), chunk(2, '中间', { state: 'failed', error_message: '向量化超时' })],
      total: 2,
    })
    const wrapper = await render()
    expect(wrapper.text()).toContain('第 1 块正文')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.text()).not.toContain('第 1 块正文')
    expect(wrapper.text()).toContain('第 2 块正文')
    expect(wrapper.text()).toContain('向量化超时')
    wrapper.unmount()
  })

  it('merges neighbouring chunks of the same section into one outline entry', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(getKnowledgeChunks).mockResolvedValue({
      items: [chunk(1, '开头'), chunk(2, '开头'), chunk(3, '结尾')],
      total: 3,
    })
    const wrapper = await render()
    const outline = wrapper.get('nav[aria-label="章节大纲"]')
    const buttons = outline.findAll('button')
    expect(buttons).toHaveLength(2)
    expect(buttons[0]!.text()).toContain('#1 - #2')
    expect(buttons[1]!.text()).toContain('#3 - #3')
    wrapper.unmount()
  })

  it('reads chunk bodies at the long-form size, not the 13px body size', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(getKnowledgeChunks).mockResolvedValue({
      items: [chunk(1, '开头')],
      total: 1,
    })
    const wrapper = await render()
    // R5：40em 按元素自身字号算 —— 13px 下只有 520px，一行读不到 40 个汉字。
    expect(wrapper.get('#chunk-1 pre').classes()).toContain('text-read')
    wrapper.unmount()
  })

  it('explains an empty document instead of leaving a blank column', async () => {
    vi.mocked(getKnowledge).mockResolvedValue({ ...baseItem } as never)
    vi.mocked(getKnowledgeChunks).mockResolvedValue({ items: [], total: 0 })
    const wrapper = await render()
    expect(wrapper.text()).toContain('尚未生成分块，任务开始后会自动展示。')
    wrapper.unmount()
  })
})
