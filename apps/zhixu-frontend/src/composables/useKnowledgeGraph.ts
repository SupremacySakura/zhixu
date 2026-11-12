import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getKnowledgeGraph,
  listKnowledge,
  type KnowledgeGraph,
  type KnowledgeGraphEdge,
  type KnowledgeItem,
  type RelationType,
} from '../services/apis/knowledge'
import { listTags, type Tag } from '../services/apis/tag'
import { knowledgeError } from '../utils/knowledge'

export interface OneHopRelation {
  edge: KnowledgeGraphEdge
  counterpartId: number
  counterpartTitle: string
}

/**
 * 知识图谱的数据编排：负责请求、取消、搜索防抖与卸载释放。
 * 页面只消费状态与动作，避免自持 AbortController（见 docs/frontend-structure.md:69）。
 */
export function useKnowledgeGraph() {
  const route = useRoute()
  const router = useRouter()

  const graph = ref<KnowledgeGraph>({ nodes: [], edges: [], truncated: false })
  const loading = ref(true)
  const error = ref('')
  const selectedId = ref<number | null>(null)
  const searchTerm = ref('')
  const searchResults = ref<KnowledgeItem[]>([])
  const searchOpen = ref(false)
  const searching = ref(false)
  const searchHint = ref('')
  const tags = ref<Tag[]>([])
  const tagId = ref(0)
  const sourceType = ref('')
  const showSuggested = ref(false)
  /** 关系类型筛选只在客户端裁剪边，不改后端查询（图例与筛选因此始终同源）。 */
  const relationType = ref<RelationType | ''>('')

  /** 标签/来源会裁剪节点，空图时要给出“无匹配”而不是“还没有知识”。 */
  const nodeFiltersActive = computed(() => tagId.value > 0 || sourceType.value !== '')
  /** 建议边开关与关系类型筛选不裁剪节点，但也算筛选状态，用于显示“清空筛选”。 */
  const filtersActive = computed(
    () => nodeFiltersActive.value || showSuggested.value || relationType.value !== '',
  )

  /** 按关系类型裁剪后的边；未选类型时原样透出后端结果。 */
  const visibleEdges = computed<KnowledgeGraphEdge[]>(() =>
    relationType.value === ''
      ? graph.value.edges
      : graph.value.edges.filter(edge => edge.relation_type === relationType.value),
  )

  const selectedNode = computed(
    () => graph.value.nodes.find(node => node.id === selectedId.value) ?? null,
  )

  /** 抽屉里只展示与中心节点直接相连的一跳关系，对端标题从当前子图节点取。 */
  const oneHopRelations = computed<OneHopRelation[]>(() => {
    const center = selectedNode.value
    if (!center) return []
    const rows: OneHopRelation[] = []
    for (const edge of visibleEdges.value) {
      const counterpartId =
        edge.source_id === center.id
          ? edge.target_id
          : edge.target_id === center.id
            ? edge.source_id
            : null
      if (counterpartId == null) continue
      const counterpart = graph.value.nodes.find(node => node.id === counterpartId)
      if (!counterpart) continue
      rows.push({ edge, counterpartId, counterpartTitle: counterpart.title })
    }
    return rows
  })

  let controller: AbortController | null = null
  let searchController: AbortController | null = null
  let tagsController: AbortController | null = null
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  function parseCenter(raw: unknown): number | undefined {
    const value = Array.isArray(raw) ? raw[0] : raw
    const parsed = Number(value)
    return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
  }

  async function loadGraph(centerId: number | undefined) {
    const current = new AbortController()
    controller?.abort()
    controller = current
    loading.value = true
    error.value = ''
    try {
      const next = await getKnowledgeGraph(
        {
          centerId,
          includeSuggested: showSuggested.value,
          tagId: tagId.value || undefined,
          sourceType: sourceType.value || undefined,
        },
        current.signal,
      )
      if (controller !== current) return
      graph.value = next
      if (selectedId.value && !next.nodes.some(node => node.id === selectedId.value)) {
        selectedId.value = null
      }
    } catch (err) {
      if (current.signal.aborted) return
      graph.value = { nodes: [], edges: [], truncated: false }
      error.value = knowledgeError(err)
    } finally {
      if (controller === current) loading.value = false
    }
  }

  watch(
    () => route.query.center,
    value => {
      selectedId.value = null
      void loadGraph(parseCenter(value))
    },
    { immediate: true },
  )

  /** 筛选只改变图的数据范围，中心仍以 URL 为准；旧图保留到新结果返回。 */
  watch([tagId, sourceType, showSuggested], () => {
    void loadGraph(parseCenter(route.query.center))
  })

  function clearFilters() {
    tagId.value = 0
    sourceType.value = ''
    showSuggested.value = false
    relationType.value = ''
  }

  async function loadTags() {
    const current = new AbortController()
    tagsController?.abort()
    tagsController = current
    try {
      const page = await listTags(current.signal)
      if (!current.signal.aborted) tags.value = page.items
    } catch {
      // 标签列表失败不阻断图谱；下拉仍保留“全部标签”。
      if (!current.signal.aborted) tags.value = []
    }
  }

  function scheduleSearch() {
    searchOpen.value = true
    searchHint.value = ''
    if (searchTimer) clearTimeout(searchTimer)
    const term = searchTerm.value.trim()
    if (!term) {
      searchResults.value = []
      searching.value = false
      return
    }
    searchTimer = setTimeout(() => void runSearch(term), 250)
  }

  async function runSearch(term: string) {
    searchController?.abort()
    const current = new AbortController()
    searchController = current
    searching.value = true
    try {
      const page = await listKnowledge(1, term, '', '', {}, current.signal)
      if (current.signal.aborted || term !== searchTerm.value.trim()) return
      searchResults.value = page.items.slice(0, 8)
      searchHint.value = page.items.length ? '' : '没有找到匹配的知识'
    } catch (err) {
      if (!current.signal.aborted) searchHint.value = knowledgeError(err)
    } finally {
      if (!current.signal.aborted) searching.value = false
    }
  }

  async function goCenter(id: number) {
    if (parseCenter(route.query.center) === id) {
      await loadGraph(id)
      return
    }
    await router.replace({ path: '/home/knowledge/graph', query: { center: String(id) } })
  }

  async function chooseCenter(item: KnowledgeItem) {
    searchTerm.value = item.title
    searchOpen.value = false
    searchResults.value = []
    await goCenter(item.id)
  }

  async function backToRecent() {
    searchTerm.value = ''
    searchOpen.value = false
    searchResults.value = []
    selectedId.value = null
    if (route.query.center) {
      await router.replace({ path: '/home/knowledge/graph' })
      return
    }
    await loadGraph(undefined)
  }

  async function recenter() {
    if (!selectedNode.value) return
    const id = selectedNode.value.id
    selectedId.value = null
    await goCenter(id)
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key !== 'Escape') return
    selectedId.value = null
    searchOpen.value = false
  }

  onMounted(() => {
    window.addEventListener('keydown', onKeydown)
    void loadTags()
  })

  onBeforeUnmount(() => {
    window.removeEventListener('keydown', onKeydown)
    controller?.abort()
    searchController?.abort()
    tagsController?.abort()
    if (searchTimer) clearTimeout(searchTimer)
  })

  return {
    graph,
    loading,
    error,
    selectedId,
    searchTerm,
    searchResults,
    searchOpen,
    searching,
    searchHint,
    tags,
    tagId,
    sourceType,
    showSuggested,
    relationType,
    visibleEdges,
    nodeFiltersActive,
    filtersActive,
    selectedNode,
    oneHopRelations,
    loadGraph,
    loadTags,
    scheduleSearch,
    clearFilters,
    goCenter,
    chooseCenter,
    backToRecent,
    recenter,
  }
}
