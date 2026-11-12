import { beforeEach, describe, expect, it, vi } from 'vitest'
import request from '../request'
import { getEvolutionSummary, listEvolutionAlerts, resolveEvolutionAlert } from './evolution'
import type { EvolutionAlert, EvolutionAlertPage, EvolutionSummary } from './evolution'

vi.mock('../request', () => ({
  default: { get: vi.fn(), post: vi.fn() },
}))

const mockedRequest = vi.mocked(request)

const page: EvolutionAlertPage = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
}

const summary: EvolutionSummary = {
  pending: 3,
  deferred: 1,
  by_type: { duplicate: 2, outdated: 1, conflict: 1 },
}

const alert: EvolutionAlert = {
  id: 7,
  alert_type: 'conflict',
  status: 'resolved',
  severity: 'strong',
  subjects: [],
  payload: { type: 'conflict', reason: '', evidence: [], suggested_action: 'create_relation' },
  resolved_at: '2026-11-12T10:00:00+08:00',
  resolved_action: 'keep_both',
  resolution_note: '两条都保留',
  created_at: '2026-11-12T09:00:00+08:00',
  updated_at: '2026-11-12T10:00:00+08:00',
}

describe('evolution api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('默认只带分页参数，空筛选不进入查询串', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await expect(listEvolutionAlerts()).resolves.toEqual(page)
    expect(mockedRequest.get).toHaveBeenCalledWith('/api/evolution/alerts', {
      params: { page: 1, page_size: 20 },
      signal: undefined,
    })
  })

  it('类型、状态与待办视野按后端字段名透传', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await listEvolutionAlerts({ page: 2, type: 'duplicate', open: true })
    expect(mockedRequest.get).toHaveBeenCalledWith('/api/evolution/alerts', {
      params: { page: 2, page_size: 20, type: 'duplicate', open: 'true' },
      signal: undefined,
    })

    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: page } })
    await listEvolutionAlerts({ status: 'dismissed', pageSize: 50 })
    expect(mockedRequest.get).toHaveBeenLastCalledWith('/api/evolution/alerts', {
      params: { page: 1, page_size: 50, status: 'dismissed' },
      signal: undefined,
    })
  })

  it('摘要接口只取一个对象', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: summary } })
    await expect(getEvolutionSummary()).resolves.toEqual(summary)
    expect(mockedRequest.get).toHaveBeenCalledWith('/api/evolution/alerts/summary', {
      signal: undefined,
    })
  })

  it('裁决提交动作与备注，未填备注时不带 note 字段', async () => {
    mockedRequest.post.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: alert } })
    await expect(
      resolveEvolutionAlert(7, { action: 'keep_both', note: '两条都保留', baseStatus: 'pending' }),
    ).resolves.toEqual(alert)
    expect(mockedRequest.post).toHaveBeenCalledWith(
      '/api/evolution/alerts/7/resolve',
      { action: 'keep_both', note: '两条都保留', base_status: 'pending' },
      { signal: undefined },
    )

    mockedRequest.post.mockResolvedValueOnce({ data: { code: 0, msg: 'ok', data: alert } })
    await resolveEvolutionAlert(7, { action: 'defer' })
    expect(mockedRequest.post).toHaveBeenLastCalledWith(
      '/api/evolution/alerts/7/resolve',
      { action: 'defer' },
      { signal: undefined },
    )
  })

  it('信封报错时抛出服务端文案', async () => {
    mockedRequest.get.mockResolvedValueOnce({ data: { code: 1001, msg: '未登录', data: null } })
    await expect(listEvolutionAlerts()).rejects.toThrow('未登录')
  })
})
