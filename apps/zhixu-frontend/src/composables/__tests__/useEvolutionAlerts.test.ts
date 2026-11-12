import { beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick } from 'vue'
import {
  getEvolutionSummary,
  listEvolutionAlerts,
  resolveEvolutionAlert,
} from '../../services/apis/evolution'
import type {
  EvolutionAlert,
  EvolutionAlertPage,
  EvolutionSummary,
} from '../../services/apis/evolution'
import { EVOLUTION_PAGE_SIZE, useEvolutionAlerts } from '../useEvolutionAlerts'

vi.mock('../../services/apis/evolution', () => ({
  listEvolutionAlerts: vi.fn(),
  getEvolutionSummary: vi.fn(),
  resolveEvolutionAlert: vi.fn(),
}))

const listMock = vi.mocked(listEvolutionAlerts)
const summaryMock = vi.mocked(getEvolutionSummary)
const resolveMock = vi.mocked(resolveEvolutionAlert)

const deferred = <T>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => {
    resolve = done
  })
  return { promise, resolve }
}

const makeAlert = (id: number, overrides: Partial<EvolutionAlert> = {}): EvolutionAlert => ({
  id,
  alert_type: 'duplicate',
  status: 'pending',
  severity: 'review',
  subjects: [],
  payload: { type: 'duplicate', reason: '', evidence: [], suggested_action: 'archive_one' },
  resolved_at: null,
  resolved_action: '',
  resolution_note: '',
  created_at: '2026-11-12T09:00:00+08:00',
  updated_at: '2026-11-12T09:00:00+08:00',
  ...overrides,
})

const pageOf = (items: EvolutionAlert[], total = items.length): EvolutionAlertPage => ({
  items,
  total,
  page: 1,
  page_size: EVOLUTION_PAGE_SIZE,
})

const summaryOf = (overrides: Partial<EvolutionSummary> = {}): EvolutionSummary => ({
  pending: 0,
  deferred: 0,
  by_type: { duplicate: 0, outdated: 0, conflict: 0 },
  ...overrides,
})

const runComposable = () => {
  const scope = effectScope()
  const api = scope.run(() => useEvolutionAlerts())!
  return { scope, ...api }
}

beforeEach(() => {
  vi.clearAllMocks()
  listMock.mockResolvedValue(pageOf([]))
  summaryMock.mockResolvedValue(summaryOf())
})

describe('useEvolutionAlerts', () => {
  it('默认按待办视野取数，并暴露列表与总数', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert(1)], 1))
    const { scope, items, total, loading, load } = runComposable()

    const loading1 = load()
    expect(loading.value).toBe(true)
    await loading1

    expect(listMock).toHaveBeenCalledWith(
      { page: 1, pageSize: EVOLUTION_PAGE_SIZE, open: true },
      expect.any(AbortSignal),
    )
    expect(items.value.map(item => item.id)).toEqual([1])
    expect(total.value).toBe(1)
    expect(loading.value).toBe(false)
    scope.stop()
  })

  it('处理记录页签合并已处理与已忽略两路，按裁决时间倒序', async () => {
    listMock.mockImplementation(async (query = {}) => {
      if (query.status === 'resolved')
        return pageOf([
          makeAlert(1, { status: 'resolved', resolved_at: '2026-11-12T10:00:00+08:00' }),
          makeAlert(2, { status: 'resolved', resolved_at: '2026-11-12T12:00:00+08:00' }),
        ])
      return pageOf([
        makeAlert(3, { status: 'dismissed', resolved_at: '2026-11-12T11:00:00+08:00' }),
      ])
    })
    const { scope, tab, items, total, load } = runComposable()

    tab.value = 'records'
    await nextTick()
    await load()

    expect(listMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'resolved' }),
      expect.any(AbortSignal),
    )
    expect(listMock).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'dismissed' }),
      expect.any(AbortSignal),
    )
    expect(items.value.map(item => item.id)).toEqual([2, 3, 1])
    expect(total.value).toBe(3)
    scope.stop()
  })

  it('切换类型筛选回到第一页并带上 type', async () => {
    const { scope, type, page, load } = runComposable()
    await load()
    page.value = 3

    type.value = 'conflict'
    await nextTick()
    await nextTick()

    expect(page.value).toBe(1)
    expect(listMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ type: 'conflict', page: 1 }),
      expect.any(AbortSignal),
    )
    scope.stop()
  })

  it('指定状态时不再传待办视野，避免后端用 open 覆盖状态', async () => {
    const { scope, status, load } = runComposable()
    await load()

    status.value = 'deferred'
    await nextTick()
    await nextTick()

    const lastQuery = listMock.mock.calls.at(-1)?.[0]
    expect(lastQuery).toMatchObject({ status: 'deferred' })
    expect(lastQuery?.open).toBeUndefined()
    scope.stop()
  })

  it('裁决成功后重拉列表与摘要，并把最新计数写回', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert(1)], 1))
    summaryMock.mockResolvedValue(summaryOf({ pending: 1 }))
    resolveMock.mockResolvedValue(
      makeAlert(1, { status: 'resolved', resolved_action: 'keep_both' }),
    )
    const { scope, summary, load, loadSummary, resolveAlert } = runComposable()

    await load()
    await loadSummary()
    expect(summary.value?.pending).toBe(1)

    listMock.mockResolvedValue(pageOf([]))
    summaryMock.mockResolvedValue(summaryOf({ pending: 0 }))
    const ok = await resolveAlert(makeAlert(1), 'keep_both', '  两条都保留  ')

    expect(ok).toBe(true)
    expect(resolveMock).toHaveBeenCalledWith(
      1,
      { action: 'keep_both', note: '两条都保留', baseStatus: 'pending' },
      expect.any(AbortSignal),
    )
    expect(listMock).toHaveBeenCalledTimes(2)
    expect(summary.value?.pending).toBe(0)
    scope.stop()
  })

  it('裁决失败时保留服务端文案且不重拉列表', async () => {
    listMock.mockResolvedValue(pageOf([makeAlert(1)], 1))
    resolveMock.mockRejectedValue(new Error('该裁决动作尚未启用'))
    const { scope, resolveError, load, resolveAlert } = runComposable()

    await load()
    const ok = await resolveAlert(makeAlert(1), 'accept_revision')

    expect(ok).toBe(false)
    expect(resolveError.value).toBe('该裁决动作尚未启用')
    expect(listMock).toHaveBeenCalledTimes(1)
    scope.stop()
  })

  it('请求失败时暴露错误文案并清空列表', async () => {
    listMock.mockRejectedValue(new Error('演化提醒请求失败'))
    const { scope, items, total, error, load } = runComposable()

    await load()

    expect(items.value).toEqual([])
    expect(total.value).toBe(0)
    expect(error.value).toBe('演化提醒请求失败')
    scope.stop()
  })

  it('scope 销毁时取消未完成的列表请求', async () => {
    const pending = deferred<EvolutionAlertPage>()
    listMock.mockImplementation(() => pending.promise)
    const { scope, load } = runComposable()

    const promise = load()
    const signal = listMock.mock.calls[0]?.[1]
    expect(signal?.aborted).toBe(false)

    scope.stop()

    expect(signal?.aborted).toBe(true)
    pending.resolve(pageOf([makeAlert(1)]))
    await promise
  })
})
