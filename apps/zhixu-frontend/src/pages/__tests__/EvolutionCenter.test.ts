import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import EvolutionCenter from '../EvolutionCenter.vue'
import {
  getEvolutionSummary,
  listEvolutionAlerts,
  resolveEvolutionAlert,
} from '../../services/apis/evolution'
import type { EvolutionAlert } from '../../services/apis/evolution'

vi.mock('../../services/apis/evolution', async importOriginal => {
  const actual = await importOriginal<typeof import('../../services/apis/evolution')>()
  return {
    // pendingTotal 是纯计算，保留真实实现；三个接口打桩。
    pendingTotal: actual.pendingTotal,
    evolutionStatusesParam: actual.evolutionStatusesParam,
    listEvolutionAlerts: vi.fn(),
    getEvolutionSummary: vi.fn(),
    resolveEvolutionAlert: vi.fn(),
  }
})
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const listMock = vi.mocked(listEvolutionAlerts)
const summaryMock = vi.mocked(getEvolutionSummary)
const resolveMock = vi.mocked(resolveEvolutionAlert)

const makeAlert = (overrides: Partial<EvolutionAlert> = {}): EvolutionAlert => ({
  id: 1,
  alert_type: 'duplicate',
  status: 'pending',
  severity: 'review',
  subjects: [
    { id: 11, title: '本次检查的知识', source_type: 'manual', archived: false, outdated: false },
    { id: 12, title: '被比对的知识', source_type: 'web', archived: false, outdated: false },
  ],
  payload: {
    type: 'duplicate',
    reason: '两条知识正文高度相似',
    evidence: [
      { knowledge_id: 11, quote: '演化提醒需要人工确认' },
      { knowledge_id: 12, quote: '人工确认是必要的' },
    ],
    suggested_action: 'archive_one',
  },
  resolved_at: null,
  resolved_action: '',
  resolution_note: '',
  created_at: '2026-11-12T09:00:00+08:00',
  updated_at: '2026-11-12T09:00:00+08:00',
  ...overrides,
})

const pageOf = (items: EvolutionAlert[]) => ({
  items,
  total: items.length,
  page: 1,
  page_size: 20,
})

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home/evolution', component: EvolutionCenter },
      { path: '/home/knowledge/:id', component: { template: '<div />' } },
    ],
  })
  await router.push('/home/evolution')
  await router.isReady()
  const wrapper = mount(EvolutionCenter, {
    global: {
      plugins: [router],
      stubs: { Motion: { template: '<div><slot /></div>' } },
    },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

async function openDetail(wrapper: Awaited<ReturnType<typeof render>>) {
  await wrapper.find('[data-test="alert-card"]').trigger('click')
  await flushPromises()
}

/** 取当前详情面板里可用的裁决动作，按渲染顺序。 */
function actionIds(wrapper: Awaited<ReturnType<typeof render>>) {
  return wrapper
    .findAll('[role="dialog"] button[data-action]')
    .map(button => button.attributes('data-action'))
}

async function chooseFilter(
  wrapper: Awaited<ReturnType<typeof render>>,
  label: string,
  option: string,
) {
  const trigger = wrapper
    .findAll('button[aria-haspopup="menu"]')
    .find(button => button.attributes('aria-label') === label)
  expect(trigger, `未找到筛选器 ${label}`).toBeTruthy()
  await trigger!.trigger('click')
  const item = wrapper
    .findAll('[role="menuitemradio"]')
    .find(button => button.text().trim() === option)
  expect(item, `未找到筛选项 ${option}`).toBeTruthy()
  await item!.trigger('click')
  await flushPromises()
}

beforeEach(() => {
  listMock.mockReset()
  summaryMock.mockReset()
  resolveMock.mockReset()
  listMock.mockResolvedValue(pageOf([]))
  summaryMock.mockResolvedValue({
    pending: 0,
    deferred: 0,
    by_type: { duplicate: 0, outdated: 0, conflict: 0 },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('EvolutionCenter', () => {
  it('按提醒类型裁剪裁决动作', async () => {
    listMock.mockResolvedValue(
      pageOf([
        makeAlert({ id: 1, alert_type: 'duplicate' }),
        makeAlert({
          id: 2,
          alert_type: 'outdated',
          payload: {
            type: 'outdated',
            reason: '',
            evidence: [],
            suggested_action: 'mark_outdated',
          },
        }),
        makeAlert({
          id: 3,
          alert_type: 'conflict',
          payload: {
            type: 'conflict',
            reason: '',
            evidence: [],
            suggested_action: 'accept_revision',
            revised_title: '修订标题',
            revised_content: '修订正文',
          },
        }),
      ]),
    )
    const wrapper = await render()

    const cards = wrapper.findAll('[data-test="alert-card"]')
    const expectations: Record<number, string[]> = {
      0: ['archive_one', 'keep_both', 'mark_verified', 'dismiss', 'defer'],
      1: ['mark_outdated', 'mark_verified', 'dismiss', 'defer'],
      2: ['accept_revision', 'keep_both', 'create_relation', 'mark_verified', 'dismiss', 'defer'],
    }
    for (const [index, actions] of Object.entries(expectations)) {
      await cards[Number(index)]!.trigger('click')
      await flushPromises()
      expect(actionIds(wrapper)).toEqual(actions)
      await wrapper.find('[role="dialog"] button[aria-label="关闭"]').trigger('click')
      await flushPromises()
    }
  })

  it('涉及知识已归档时标注并禁用采纳修订与归档其一', async () => {
    listMock.mockResolvedValue(
      pageOf([
        makeAlert({
          alert_type: 'conflict',
          subjects: [
            {
              id: 11,
              title: '本次检查的知识',
              source_type: 'manual',
              archived: false,
              outdated: false,
            },
            { id: 12, title: '被比对的知识', source_type: 'web', archived: true, outdated: false },
          ],
          payload: {
            type: 'conflict',
            reason: '两条结论互斥',
            evidence: [],
            suggested_action: 'accept_revision',
            revised_title: '修订标题',
            revised_content: '修订正文',
          },
        }),
      ]),
    )
    const wrapper = await render()

    expect(wrapper.text()).toContain('涉及知识已归档')
    await openDetail(wrapper)

    expect(
      wrapper.find('[role="dialog"] button[data-action="accept_revision"]').attributes('disabled'),
    ).toBeDefined()
    expect(wrapper.find('[role="dialog"] button[data-action="archive_one"]').exists()).toBe(false)
    expect(
      wrapper.find('[role="dialog"] button[data-action="dismiss"]').attributes('disabled'),
    ).toBeUndefined()
    expect(
      wrapper.find('[role="dialog"] button[data-action="mark_verified"]').attributes('disabled'),
    ).toBeUndefined()
    // 归档知识也不能归档其一、保留两者或建立矛盾关系——除忽略与标记已核实外全部置灰
    for (const action of ['keep_both', 'defer']) {
      expect(
        wrapper.find(`[role="dialog"] button[data-action="${action}"]`).attributes('disabled'),
      ).toBeDefined()
    }
    expect(wrapper.find('[role="dialog"]').text()).toContain('涉及知识已归档，只能忽略或标记已核实')
  })

  it('冲突提醒缺少修订稿时置灰采纳修订并说明原因', async () => {
    listMock.mockResolvedValue(
      pageOf([
        makeAlert({
          alert_type: 'conflict',
          payload: {
            type: 'conflict',
            reason: '两条结论互斥',
            evidence: [],
            suggested_action: 'accept_revision',
          },
        }),
      ]),
    )
    const wrapper = await render()
    await openDetail(wrapper)

    const accept = wrapper.find('[role="dialog"] button[data-action="accept_revision"]')
    expect(accept.attributes('disabled')).toBeDefined()
    expect(wrapper.find('[role="dialog"]').text()).toContain('该提醒没有修订稿，暂不能采纳修订')
  })

  it('区分「没有提醒」与「筛选无结果」两种空状态', async () => {
    const wrapper = await render()
    expect(wrapper.find('[data-test="empty-none"]').text()).toContain('当前没有需要处理的演化提醒')

    await chooseFilter(wrapper, '提醒类型', '冲突')
    expect(wrapper.find('[data-test="empty-none"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="empty-filtered"]').text()).toContain('没有符合筛选条件的提醒')
  })

  it('并排展示两侧证据引文并高亮共有片段', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert()]))
    const wrapper = await render()
    await openDetail(wrapper)

    const panel = wrapper.find('[role="dialog"]')
    expect(panel.text()).toContain('本次检查的知识')
    expect(panel.text()).toContain('被比对的知识')

    const marks = panel.findAll('mark')
    expect(marks.map(mark => mark.text())).toEqual(['人工确认', '人工确认'])
    expect(panel.text()).toContain('演化提醒需要人工确认')
    expect(panel.text()).toContain('人工确认是必要的')
  })

  it('裁决时带上备注并在成功后关闭面板', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert()]))
    resolveMock.mockResolvedValue(makeAlert({ status: 'resolved', resolved_action: 'keep_both' }))
    const wrapper = await render()
    await openDetail(wrapper)

    await wrapper.find('[role="dialog"] textarea').setValue('两条都保留，后续合并摘要')
    await wrapper.find('[role="dialog"] button[data-action="keep_both"]').trigger('click')
    await flushPromises()

    expect(resolveMock).toHaveBeenCalledWith(
      1,
      { action: 'keep_both', note: '两条都保留，后续合并摘要', baseStatus: 'pending' },
      expect.any(AbortSignal),
    )
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('处理记录展示裁决动作、备注与时间', async () => {
    listMock.mockResolvedValue(
      pageOf([
        makeAlert({
          id: 5,
          status: 'resolved',
          resolved_action: 'keep_both',
          resolution_note: '两条都保留',
          resolved_at: '2026-11-12T10:30:00+08:00',
        }),
      ]),
    )
    const wrapper = await render()

    await wrapper.findAll('[role="tab"]')[1]!.trigger('click')
    await flushPromises()

    const card = wrapper.find('[data-test="alert-card"]')
    expect(card.text()).toContain('保留两者')
    expect(card.text()).toContain('两条都保留')
    expect(card.text()).toContain('2026')
    // 记录页签一次请求两个状态，排序与 total 都由服务端给。
    expect(listMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'resolved,dismissed' }),
      expect.any(AbortSignal),
    )
  })

  it('页头待办数含暂不处理，与首页卡片的待处理数语义不同', async () => {
    summaryMock.mockResolvedValue({
      pending: 4,
      deferred: 1,
      by_type: { duplicate: 2, outdated: 2, conflict: 1 },
    })
    const wrapper = await render()

    // 演化中心页头是待办视野总数（pending + deferred），首页卡片只算 pending。
    expect(wrapper.find('header').text()).toContain('5')
  })

  it('裁决失败时把服务端文案显示在面板里', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert()]))
    resolveMock.mockRejectedValue(new Error('该裁决动作尚未启用'))
    const wrapper = await render()
    await openDetail(wrapper)

    await wrapper.find('[role="dialog"] button[data-action="archive_one"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="dialog"]').text()).toContain('该裁决动作尚未启用')
  })
})
