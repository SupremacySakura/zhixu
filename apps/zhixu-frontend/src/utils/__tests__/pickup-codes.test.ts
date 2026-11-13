import { describe, expect, it } from 'vitest'
import type { PickupCodeItem } from '../../services/types/file'
import { pickupCodeStats } from '../pickup-codes'

const code = (overrides: Partial<PickupCodeItem>): PickupCodeItem => ({
  id: 1,
  code: 'ABC123',
  file_id: 1,
  folder_id: null,
  name: 'alpha.txt',
  type: 'file',
  download: 0,
  max_download: 10,
  expire_time: '2026-01-01T00:00:00Z',
  created_at: '2025-12-01T00:00:00Z',
  status: 'Active',
  ...overrides,
})

describe('pickup code stats', () => {
  // 这一条钉的是 #45 的 R22：票面统计卡标题写「活跃取件码」「总下载量」，
  // 数值却只统计当前页 10 条 —— 翻到第 2 页会从 10 掉到 1，是假口径。
  // 总数必须来自后端 count 接口（totalCount），与当前页有多少条无关。
  it('reports the account-wide total from the count endpoint, not the page size', () => {
    const page = [code({ id: 1 }), code({ id: 2 })]

    expect(pickupCodeStats(page, 37).total).toBe(37)
  })

  it('keeps page-scoped figures scoped to the page they were measured on', () => {
    const page = [
      code({ id: 1, download: 3 }),
      code({ id: 2, download: 4 }),
      code({ id: 3, download: 5 }),
    ]

    // 后端只有 /code/count 一个聚合接口，没有总下载量、没有即将过期数，
    // 所以这两个数只能诚实地报「本页」——分母写 10 也是谎。
    expect(pickupCodeStats(page, 37).pageDownloads).toBe(12)
    expect(pickupCodeStats(page, 37).pageCount).toBe(3)
  })

  it('counts only codes expiring inside the next seven days', () => {
    const now = new Date('2026-01-01T00:00:00Z')
    const page = [
      code({ id: 1, expire_time: '2026-01-03T00:00:00Z' }), // 2 天后
      code({ id: 2, expire_time: '2026-01-07T00:00:00Z' }), // 6 天后
      code({ id: 3, expire_time: '2026-02-01T00:00:00Z' }), // 31 天后
      code({ id: 4, expire_time: '2025-12-25T00:00:00Z' }), // 已过期
    ]

    expect(pickupCodeStats(page, 4, now).pageExpiringSoon).toBe(2)
  })
})
