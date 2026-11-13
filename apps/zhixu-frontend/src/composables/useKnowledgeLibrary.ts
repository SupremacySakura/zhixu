import { onScopeDispose, ref, watch } from 'vue'
import {
  listKnowledge,
  type KnowledgeItem,
  type KnowledgeSort,
  type KnowledgeStatus,
  type KnowledgeVectorState,
} from '../services/apis/knowledge'
import { listTags, type Tag } from '../services/apis/tag'
import { isKnowledgeActive, isKnowledgeVectorActive, knowledgeError } from '../utils/knowledge'

export function useKnowledgeLibrary() {
  const items = ref<KnowledgeItem[]>([])
  const total = ref(0)
  /**
   * 无筛选时的索引总量（#40 R2）。
   *
   * 工具栏右端要显示「命中 9 / 142」，分母是**整个知识库**有多大，
   * 而不是这次查询命中多少。分母只能在无筛选那一次响应里读到 —— 筛选后的
   * `result.total` 是命中数。所以这里缓存无筛选时的 total，而不是为拿分母
   * 再打一次 `listKnowledge`（那会让每次筛选都翻倍请求，且既有
   * `toHaveBeenCalledTimes` 断言会红）。
   *
   * 只在**无筛选**时更新：清空筛选后回到完整列表，分母自然刷新；
   * 带筛选的响应永远不会污染它。
   */
  const totalAll = ref(0)
  const page = ref(1)
  const search = ref('')
  const status = ref<KnowledgeStatus | ''>('')
  const vector = ref<KnowledgeVectorState>('')
  const sourceType = ref('')
  const tagId = ref(0)
  const favorite = ref<'' | 'true'>('')
  const archived = ref<'' | 'true'>('')
  const sort = ref<KnowledgeSort>('created')
  /**
   * 列表的呈现方式：卡片（默认）还是紧凑表格（#35 用户故事 4）。
   *
   * 放在组合式函数里而不是页面里，理由与 `sort` 一致：它是**列表这一屏的状态**，
   * 不是页面的临时 UI 开关。翻页、改筛选、改搜索都不该把它重置 —— 用户选的是
   * 「我现在想用哪种方式扫读」，换一页不该被踢回卡片视图。
   *
   * 为什么不落 localStorage：这是纯视觉偏好，刷新后回到卡片视图是安全的默认；
   * 持久化会引入「昨天切了表格，今天打开以为坏了」的状态残留。
   */
  const viewMode = ref<'card' | 'table'>('card')
  const tags = ref<Tag[]>([])
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null
  let debounce: ReturnType<typeof setTimeout> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let query = ''
  /**
   * 这一屏是否带筛选条件（决定 total 是「命中数」还是「全库数」）。
   *
   * 口径与页面上的 `hasFilters` 一致：搜索词算筛选，**排序不算** ——
   * 换排序不改变结果集大小，只是换个顺序，此时 total 仍是全库条数。
   */
  function hasActiveFilters() {
    return Boolean(
      query ||
      status.value ||
      vector.value ||
      sourceType.value ||
      tagId.value ||
      favorite.value ||
      archived.value,
    )
  }
  async function loadTags() {
    try {
      tags.value = (await listTags()).items
    } catch {
      tags.value = []
    }
  }
  async function load(background = false) {
    controller?.abort()
    clearTimeout(timer)
    const current = new AbortController()
    controller = current
    if (!background) loading.value = true
    error.value = ''
    try {
      const result = await listKnowledge(
        page.value,
        query,
        status.value,
        vector.value,
        {
          sourceType: sourceType.value || undefined,
          tagId: tagId.value || undefined,
          favorite: favorite.value === 'true' ? true : undefined,
          archived: archived.value === 'true',
          sort: sort.value,
        },
        current.signal,
      )
      if (current.signal.aborted) return
      const lastPage = Math.max(1, Math.ceil(result.total / 20))
      if (page.value > lastPage) {
        page.value = lastPage
        await load()
        return
      }
      items.value = result.items
      total.value = result.total
      // 分母只在「这一屏没有筛选条件」时采信：此时 total 就是全库条数。
      // 筛选态下 total 是命中数，写进分母会让「命中 9 / 9」自我循环。
      if (!hasActiveFilters()) totalAll.value = result.total
      if (
        items.value.some(
          item => isKnowledgeActive(item.status) || isKnowledgeVectorActive(item.vector_state),
        ) ||
        status.value === 'pending' ||
        status.value === 'running' ||
        vector.value === 'pending' ||
        vector.value === 'running'
      )
        timer = setTimeout(() => load(true), 3000)
    } catch (e) {
      if (!current.signal.aborted) error.value = knowledgeError(e)
    } finally {
      if (controller === current) loading.value = false
    }
  }
  watch(search, () => {
    controller?.abort()
    clearTimeout(debounce)
    clearTimeout(timer)
    debounce = setTimeout(() => {
      query = search.value.trim()
      page.value = 1
      void load()
    }, 300)
  })
  watch([status, vector, sourceType, tagId, favorite, archived, sort], () => {
    page.value = 1
    void load()
  })
  function changePage(value: number) {
    page.value = value
    void load()
  }
  onScopeDispose(() => {
    controller?.abort()
    clearTimeout(debounce)
    clearTimeout(timer)
  })
  return {
    items,
    total,
    totalAll,
    page,
    search,
    status,
    vector,
    sourceType,
    tagId,
    favorite,
    archived,
    sort,
    viewMode,
    tags,
    loading,
    error,
    load,
    loadTags,
    changePage,
  }
}
