import type { PickupCodeItem } from '../services/types/file'

/** 「即将过期」的窗口：票面写「未来 7 天」，与后端 ExpireOutdatedPickUpCodes 无关，纯展示口径。 */
const EXPIRING_WINDOW_MS = 7 * 24 * 60 * 60 * 1000

export interface PickupCodeStats {
  /** 账号下全部取件码条数，来自 /api/file/code/count，与当前页无关。 */
  total: number
  /** 当前页条数——只用来给「本页」口径做分母，不当总数用。 */
  pageCount: number
  /** 当前页下载次数之和。后端没有账号级下载量聚合，只能报本页。 */
  pageDownloads: number
  /** 当前页里未来 7 天内到期的条数。同上，只能报本页。 */
  pageExpiringSoon: number
}

/**
 * 统计卡的口径计算。
 *
 * 为什么「总数」必须外部传入 totalCount 而不是数 items.length：
 * /api/file/code/list 是分页接口，一页 10 条。拿当前页去数总数，
 * 翻到第 2 页就会从 10 掉到 1，而卡面标题写的是账号级的「活跃取件码」。
 * 后端只有 /api/file/code/count 一个聚合接口（SELECT count(*) WHERE user_id=?），
 * 没有活跃数、没有总下载量、没有即将过期数，所以其余三项只能诚实地限定在「本页」，
 * 由卡面的限定语交代清楚，而不是冒充账号级数字。
 */
export function pickupCodeStats(
  page: readonly PickupCodeItem[],
  totalCount: number,
  now: Date = new Date(),
): PickupCodeStats {
  const deadline = now.getTime() + EXPIRING_WINDOW_MS
  const nowMs = now.getTime()

  return {
    total: totalCount,
    pageCount: page.length,
    pageDownloads: page.reduce((sum, item) => sum + item.download, 0),
    pageExpiringSoon: page.filter(item => {
      const expiresAt = new Date(item.expire_time).getTime()
      if (Number.isNaN(expiresAt)) return false
      return expiresAt > nowMs && expiresAt <= deadline
    }).length,
  }
}
