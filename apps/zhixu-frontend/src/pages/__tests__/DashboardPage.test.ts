import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import DashboardPage from '../DashboardPage.vue'
import { getDashboardOverview } from '../../services/apis/file'
import { getOperationLogs } from '../../services/apis/log'
import { getEvolutionSummary, listEvolutionAlerts } from '../../services/apis/evolution'
import { useUserStore } from '../../stores/user'

vi.mock('../../services/apis/file', () => ({ getDashboardOverview: vi.fn() }))
vi.mock('../../services/apis/log', () => ({ getOperationLogs: vi.fn() }))
vi.mock('../../services/apis/evolution', () => ({
  getEvolutionSummary: vi.fn(),
  listEvolutionAlerts: vi.fn(),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const overviewMock = vi.mocked(getDashboardOverview)
const logsMock = vi.mocked(getOperationLogs)
const summaryMock = vi.mocked(getEvolutionSummary)
const listMock = vi.mocked(listEvolutionAlerts)

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/home/evolution', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  await router.isReady()
  const wrapper = mount(DashboardPage, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

beforeEach(() => {
  setActivePinia(createPinia())
  useUserStore().setToken('token')
  overviewMock.mockReset()
  logsMock.mockReset()
  summaryMock.mockReset()
  listMock.mockReset()
  overviewMock.mockResolvedValue({
    storage_used: 0,
    storage_total: 0,
    storage_left: 0,
    storage_used_percent: 0,
    file_stats: [],
  })
  logsMock.mockResolvedValue({ list: [], total: 0 })
  summaryMock.mockResolvedValue({
    pending: 4,
    deferred: 1,
    by_type: { duplicate: 2, outdated: 2, conflict: 1 },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('DashboardPage 演化提醒卡片', () => {
  it('展示待处理总数与三类计数，并链到演化中心', async () => {
    const { wrapper } = await render()

    const card = wrapper.find('[data-test="evolution-card"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('待处理提醒')
    // 规格 #8：首页「待处理」只算 pending，不含 deferred（这里是 4 而不是 4+1）。
    expect(card.find('[data-test="evolution-pending-total"]').text()).toBe('4')
    expect(card.text()).toContain('重复')
    expect(card.text()).toContain('过时')
    expect(card.text()).toContain('冲突')
    expect(card.attributes('href')).toBe('/home/evolution')
  })

  it('只调摘要接口，不拉提醒列表', async () => {
    await render()
    expect(summaryMock).toHaveBeenCalledTimes(1)
    expect(listMock).not.toHaveBeenCalled()
  })

  it('暂不处理的提醒不计入首页待处理数量', async () => {
    summaryMock.mockResolvedValue({
      pending: 0,
      deferred: 3,
      by_type: { duplicate: 1, outdated: 1, conflict: 1 },
    })
    const { wrapper } = await render()

    const card = wrapper.find('[data-test="evolution-card"]')
    expect(card.find('[data-test="evolution-pending-total"]').exists()).toBe(false)
    expect(card.text()).toContain('暂无待处理提醒')
  })

  it('摘要为空时给出无提醒文案', async () => {
    summaryMock.mockResolvedValue({
      pending: 0,
      deferred: 0,
      by_type: { duplicate: 0, outdated: 0, conflict: 0 },
    })
    const { wrapper } = await render()

    expect(wrapper.find('[data-test="evolution-card"]').text()).toContain('暂无待处理提醒')
  })
})
