import { computed, onScopeDispose, ref, watch } from 'vue'
import {
  getEvolutionSummary,
  listEvolutionAlerts,
  resolveEvolutionAlert,
} from '../services/apis/evolution'
import type {
  EvolutionAction,
  EvolutionAlert,
  EvolutionAlertStatus,
  EvolutionAlertType,
  EvolutionSummary,
} from '../services/apis/evolution'
import { knowledgeError } from '../utils/knowledge'

/** 每页条数；后端上限 100，这里固定 20 与其它列表页一致。 */
export const EVOLUTION_PAGE_SIZE = 20

/** 页签：待处理（待处理 + 暂不处理）与处理记录（已处理 + 已忽略）。 */
export type EvolutionTab = 'pending' | 'records'

/**
 * 演化中心的列表、筛选与裁决状态。
 *
 * 两个页签的语义不同：待办视野是后端的一个查询（open=true），处理记录要合并
 * 「已处理」与「已忽略」两个状态各查一次再按裁决时间倒序拼接，所以分页总数取两路之和。
 */
export function useEvolutionAlerts() {
  const tab = ref<EvolutionTab>('pending')
  const type = ref<EvolutionAlertType | ''>('')
  const status = ref<EvolutionAlertStatus | ''>('')
  const page = ref(1)

  const items = ref<EvolutionAlert[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')
  const resolveError = ref('')
  const resolving = ref(false)
  const summary = ref<EvolutionSummary | null>(null)

  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / EVOLUTION_PAGE_SIZE)))

  let listController: AbortController | null = null
  let summaryController: AbortController | null = null
  let resolveController: AbortController | null = null

  const sortByResolvedAt = (left: EvolutionAlert, right: EvolutionAlert) =>
    (right.resolved_at ? Date.parse(right.resolved_at) : 0) -
    (left.resolved_at ? Date.parse(left.resolved_at) : 0)

  const loadRecords = async (signal: AbortSignal) => {
    const query = { page: page.value, pageSize: EVOLUTION_PAGE_SIZE, type: type.value }
    // 记录页签横跨「已处理」与「已忽略」两个状态，指定状态时只查其中一路。
    const wanted: EvolutionAlertStatus[] =
      status.value === 'resolved' || status.value === 'dismissed'
        ? [status.value]
        : ['resolved', 'dismissed']
    const pages = await Promise.all(
      wanted.map(state => listEvolutionAlerts({ ...query, status: state }, signal)),
    )
    return {
      items: pages.flatMap(result => result.items).sort(sortByResolvedAt),
      total: pages.reduce((sum, result) => sum + result.total, 0),
    }
  }

  const load = async () => {
    const current = new AbortController()
    listController?.abort()
    listController = current
    loading.value = true
    error.value = ''
    try {
      const result =
        tab.value === 'records'
          ? await loadRecords(current.signal)
          : await listEvolutionAlerts(
              {
                page: page.value,
                pageSize: EVOLUTION_PAGE_SIZE,
                ...(type.value ? { type: type.value } : {}),
                // 指定具体状态时按状态查，否则按待办视野（待处理 + 暂不处理）查。
                // open 在服务端优先于 status，两者不能同时传。
                ...(status.value ? { status: status.value } : { open: true }),
              },
              current.signal,
            )
      if (current.signal.aborted) return
      items.value = result.items
      total.value = result.total
    } catch (cause) {
      if (current.signal.aborted) return
      items.value = []
      total.value = 0
      error.value = knowledgeError(cause)
    } finally {
      if (listController === current) {
        listController = null
        loading.value = false
      }
    }
  }

  const loadSummary = async () => {
    const current = new AbortController()
    summaryController?.abort()
    summaryController = current
    try {
      const data = await getEvolutionSummary(current.signal)
      if (current.signal.aborted) return
      summary.value = data
    } catch (cause) {
      if (current.signal.aborted) return
      // 摘要只服务首页卡片与页头计数，失败不阻塞主列表。
      console.error('获取演化提醒摘要失败', cause)
    }
  }

  /** 提交裁决；成功返回 true，失败把服务端文案留在 resolveError 里供页面展示。 */
  const resolveAlert = async (alert: EvolutionAlert, action: EvolutionAction, note = '') => {
    const current = new AbortController()
    resolveController?.abort()
    resolveController = current
    resolving.value = true
    resolveError.value = ''
    try {
      await resolveEvolutionAlert(
        alert.id,
        { action, note: note.trim(), baseStatus: alert.status },
        current.signal,
      )
      if (current.signal.aborted) return false
      await Promise.all([load(), loadSummary()])
      return true
    } catch (cause) {
      if (current.signal.aborted) return false
      resolveError.value = knowledgeError(cause)
      return false
    } finally {
      if (resolveController === current) {
        resolveController = null
        resolving.value = false
      }
    }
  }

  const changePage = (next: number) => {
    page.value = Math.min(Math.max(1, next), totalPages.value)
    void load()
  }

  watch([tab, type, status], ([nextTab], [prevTab]) => {
    // 换页签时旧的状态筛选不再适用（待办视野里没有「已处理」），先复位再让下一轮 watch 重拉，
    // 避免先用过期筛选发一次请求。
    if (nextTab !== prevTab && status.value) {
      status.value = ''
      return
    }
    page.value = 1
    void load()
  })

  onScopeDispose(() => {
    listController?.abort()
    summaryController?.abort()
    resolveController?.abort()
  })

  return {
    tab,
    type,
    status,
    page,
    items,
    total,
    totalPages,
    loading,
    error,
    resolveError,
    resolving,
    summary,
    load,
    loadSummary,
    resolveAlert,
    changePage,
  }
}
