import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeLibrary from '../KnowledgeLibrary.vue'
import {
  favoriteKnowledge,
  listKnowledge,
  listKnowledgeRelations,
  deleteKnowledge,
  retryKnowledgeVectorize,
  unarchiveKnowledge,
  type KnowledgeItem,
  type KnowledgeRelation,
} from '../../services/apis/knowledge'
import KnowledgeActionMenu from '../../components/knowledge/KnowledgeActionMenu.vue'
import AppToast from '../../components/ui/AppToast.vue'
import { defineComponent } from 'vue'
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
  // 列表页每张卡右侧的交叉引用纸条要逐卡取关系（#40 R1）。默认返回空关系，
  // 既有 9 条断言一条都不涉及纸条，因此它们照常绿。
  listKnowledgeRelations: vi.fn().mockResolvedValue({ confirmed: [], suggested: [], task: null }),
}))
vi.mock('../../services/apis/tag', () => ({
  listTags: vi.fn().mockResolvedValue({ items: [] }),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))
afterEach(() => {
  vi.clearAllMocks()
})
// 页面在生产里永远活在带 toast 宿主的外壳里；测试如实照搬这个结构，
// 而不是让页面自己渲染提示——那样测的是旧架构，不是用户看到的东西。
const Harness = defineComponent({
  components: { Page: KnowledgeLibrary, AppToast },
  template: '<div><Page /><AppToast /></div>',
})

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: KnowledgeLibrary }],
  })
  await router.push('/')
  await router.isReady()
  const wrapper = mount(Harness, { global: { plugins: [router] } })
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

  /**
   * 交叉引用纸条（#40 R1）。
   *
   * 测的是「用户不点任何东西就能看到这张卡依赖哪一条、和哪一条矛盾」——
   * 这是票面 AC 的原文。因此断言必须落在**卡面上可见的文字**，
   * 而不是内部是否调过接口：接口调用是手段，纸条读得出内容是目的。
   */
  it('在卡片右侧逐卡展示交叉引用纸条', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [
        { id: 10, title: '笔记', status: 'ready', created_at: '' } as never,
        { id: 11, title: '另一篇', status: 'ready', created_at: '' } as never,
      ],
      total: 2,
      page: 1,
      page_size: 20,
    })
    const relation = (id: number, type: string, title: string) =>
      ({
        id,
        source_id: 10,
        target_id: 2,
        relation_type: type,
        status: 'confirmed',
        source: 'manual',
        confidence: 1,
        evidence: '',
        counterpart_id: 2,
        counterpart_title: title,
        created_at: '',
        confirmed_at: '',
      }) as KnowledgeRelation
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [relation(1, 'depends_on', '基础篇'), relation(2, 'contradicts', '旧结论')],
      suggested: [],
      task: null,
    })
    const wrapper = await render()
    // 当前页每一条都要取一次关系，不是只取第一条。
    expect(listKnowledgeRelations).toHaveBeenCalledTimes(2)
    expect(listKnowledgeRelations).toHaveBeenCalledWith(10, expect.any(AbortSignal))
    expect(listKnowledgeRelations).toHaveBeenCalledWith(11, expect.any(AbortSignal))
    // 纸条文案必须来自 RELATION_TYPE_LABELS，且带对端标题。
    expect(wrapper.text()).toContain('依赖')
    expect(wrapper.text()).toContain('基础篇')
    expect(wrapper.text()).toContain('矛盾')
    expect(wrapper.text()).toContain('旧结论')
    wrapper.unmount()
  })

  it('没有交叉引用时纸条写明「暂无交叉引用」而不是留白', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [{ id: 10, title: '笔记', status: 'ready', created_at: '' } as never],
      total: 1,
      page: 1,
      page_size: 20,
    })
    // afterEach 只 clearAllMocks（清调用记录），不清实现，所以别的 it 的
    // mockResolvedValue / mockRejectedValue 会留下来 —— 显式写回，不靠测试顺序。
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [],
      suggested: [],
      task: null,
    })
    const wrapper = await render()
    // 成功但为空：纸条仍然要给一句话，不能什么都不显示。
    expect(wrapper.text()).toContain('暂无交叉引用')
    wrapper.unmount()
  })

  it('纸条取不到时说明「读取失败」，不能假装这条知识没有交叉引用', async () => {
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [{ id: 10, title: '笔记', status: 'ready', created_at: '' } as never],
      total: 1,
      page: 1,
      page_size: 20,
    })
    // 后端限流（60 req/min/IP）或单卡 500 时，关系请求会失败。
    // 失败与「确实没有关系」是两件事：前者不能渲染成「暂无交叉引用」——
    // 那正是 R1 拒绝图谱接口的理由（假阴性比不显示更糟）。
    vi.mocked(listKnowledgeRelations).mockRejectedValue(new Error('too many requests'))
    const wrapper = await render()

    expect(wrapper.text()).toContain('读取失败')
    expect(wrapper.text()).not.toContain('暂无交叉引用')
    wrapper.unmount()
  })

  /**
   * 命中计数（#40 R2）。
   *
   * 分母只能来自「首次无筛选加载」那一次的 total —— 不能为了拿分母再打一次
   * listKnowledge（会打红既有三条 toHaveBeenCalledTimes 断言）。
   */
  it('工具栏显示「命中 N / 总数」，无筛选时只显示命中数', async () => {
    vi.mocked(listKnowledge).mockResolvedValueOnce({
      items: [{ id: 10, title: '笔记', status: 'ready', created_at: '' } as never],
      total: 142,
      page: 1,
      page_size: 20,
    })
    const wrapper = await render()
    expect(wrapper.get('[data-test="hit-count"]').text()).toContain('命中 142')

    // 带筛选再加载：total 变成命中数，分母仍是首次缓存的 142。
    vi.mocked(listKnowledge).mockResolvedValue({
      items: [{ id: 10, title: '笔记', status: 'ready', created_at: '' } as never],
      total: 9,
      page: 1,
      page_size: 20,
    })
    await wrapper.get('[aria-label="筛选归档"]').trigger('click')
    await wrapper
      .findAll('[role="menuitemradio"]')
      .find(option => option.text() === '仅已归档')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="hit-count"]').text()).toContain('命中 9 / 142')
    // 两次加载：初次 + 筛选，没有为分母多打一次。
    expect(listKnowledge).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  /**
   * 表格视图（#35 用户故事 4 / #40 AC 第 3 条）。
   *
   * 断言落在**用户看到的东西**上：切过去之后屏幕上是不是真的有一屏 20 行。
   * 同时钉住「换视图不是换查询」——切视图不许再打一次列表接口，
   * 否则 60 req/min/IP 的后端限流会在快速来回切时把页面打成 429。
   */
  it('切到表格视图后一屏 20 行，且不重新请求列表', async () => {
    const rows = Array.from({ length: 20 }, (_, index) => ({
      id: index + 1,
      title: `第 ${index + 1} 条`,
      status: 'ready',
      created_at: '',
      chunks_total: 0,
    }))
    vi.mocked(listKnowledge).mockResolvedValue({
      items: rows as never,
      total: 20,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listKnowledgeRelations).mockResolvedValue({
      confirmed: [],
      suggested: [],
      task: null,
    })
    const wrapper = await render()
    // 默认是卡片视图：此时没有表格。
    expect(wrapper.find('table').exists()).toBe(false)

    await wrapper.get('[aria-label="表格"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr')).toHaveLength(20)
    // 切视图复用已有 items：列表接口只被调用过首次那一次。
    expect(listKnowledge).toHaveBeenCalledTimes(1)

    // 切回卡片视图：表格消失，卡片回来。
    await wrapper.get('[aria-label="卡片"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
})
