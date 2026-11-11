<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Motion } from 'motion-v'
import KnowledgeGraphChart from '../components/knowledge/KnowledgeGraphChart.vue'
import {
  getKnowledgeGraph,
  listKnowledge,
  type KnowledgeGraph,
  type KnowledgeGraphEdge,
  type KnowledgeItem,
  type RelationStatus,
} from '../services/apis/knowledge'
import { knowledgeError, knowledgeSourceLabel } from '../utils/knowledge'
import { relationTypeLabel } from '../utils/relation'
import { springSnappy } from '../utils/motion'

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

const STATUS_LABELS: Record<RelationStatus, string> = {
  suggested: '建议',
  confirmed: '已确认',
  rejected: '已拒绝',
}

const selectedNode = computed(
  () => graph.value.nodes.find(node => node.id === selectedId.value) ?? null,
)

interface OneHopRelation {
  edge: KnowledgeGraphEdge
  counterpartId: number
  counterpartTitle: string
}

/** 抽屉里只展示与中心节点直接相连的一跳关系，对端标题从当前子图节点取。 */
const oneHopRelations = computed<OneHopRelation[]>(() => {
  const center = selectedNode.value
  if (!center) return []
  const rows: OneHopRelation[] = []
  for (const edge of graph.value.edges) {
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

function parseCenter(raw: unknown): number | undefined {
  const value = Array.isArray(raw) ? raw[0] : raw
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

let controller: AbortController | null = null
let searchController: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | undefined

async function loadGraph(centerId: number | undefined) {
  const current = new AbortController()
  controller?.abort()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const next = await getKnowledgeGraph(centerId, false, current.signal)
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

onMounted(() => window.addEventListener('keydown', onKeydown))

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  controller?.abort()
  searchController?.abort()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <p class="mb-4 text-subhead text-label-secondary">知识库 / 知识图谱</p>
      <header class="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-title-1">知识图谱</h1>
          <p class="mt-2 text-subhead text-label-secondary">
            以一条知识为中心查看它的一跳关系，也可以从最近更新的知识开始探索。
          </p>
        </div>
      </header>

      <section class="mb-4 flex flex-wrap items-center gap-3">
        <div class="relative w-full max-w-sm">
          <input
            v-model="searchTerm"
            class="knowledge-input w-full"
            placeholder="搜索知识，设为图谱中心"
            type="search"
            @input="scheduleSearch"
            @focus="searchOpen = true"
          />
          <div
            v-if="searchOpen && (searchResults.length || searching || searchHint)"
            class="absolute z-20 mt-2 w-full overflow-hidden rounded-lg border border-hairline bg-surface shadow-popover"
          >
            <p v-if="searching" class="px-3 py-2 text-subhead text-label-secondary">正在搜索…</p>
            <p v-else-if="searchHint" class="px-3 py-2 text-subhead text-label-secondary">
              {{ searchHint }}
            </p>
            <button
              v-for="item in searchResults"
              :key="item.id"
              type="button"
              class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-surface-secondary"
              @click="chooseCenter(item)"
            >
              <span class="truncate text-subhead text-label">{{ item.title }}</span>
              <span class="shrink-0 text-caption text-label-secondary">
                {{ knowledgeSourceLabel(item.source_type) }}
              </span>
            </button>
          </div>
        </div>
        <button type="button" class="knowledge-button-secondary" @click="backToRecent">
          回到最近
        </button>
      </section>

      <p
        v-if="graph.truncated"
        class="mb-4 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
        role="status"
      >
        节点较多，已截断为连接度最高的 100 个节点。可以缩小范围，以某条知识为中心查看局部图谱。
      </p>
      <p
        v-if="error"
        class="mb-4 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
        role="alert"
      >
        {{ error }}
      </p>
      <p
        v-if="loading"
        class="rounded-lg border border-hairline bg-surface p-6 text-subhead text-label-secondary"
      >
        正在装载图谱…
      </p>

      <div
        v-if="!loading && !error && graph.nodes.length === 0"
        class="rounded-lg border border-hairline bg-surface px-6 py-16 text-center"
      >
        <h2 class="text-title-3">还没有可展示的知识</h2>
        <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
          先导入或创建知识，再回到这里查看它们之间的关系。
        </p>
        <RouterLink to="/home/knowledge" class="knowledge-button-primary mt-5">
          去知识库
        </RouterLink>
      </div>

      <template v-if="!loading && !error && graph.nodes.length">
        <p
          v-if="graph.edges.length === 0"
          class="mb-4 rounded-lg border border-hairline bg-surface p-4 text-subhead text-label-secondary"
        >
          这些知识之间还没有已确认关系。打开任意知识详情页，在「知识关系」面板中建立或确认关系后再回来查看。
        </p>
        <section class="rounded-lg border border-hairline bg-surface p-2 shadow-card">
          <KnowledgeGraphChart
            :nodes="graph.nodes"
            :edges="graph.edges"
            :selected-id="selectedId"
            @node-click="selectedId = $event"
          />
        </section>
      </template>
    </div>

    <div
      v-if="selectedNode"
      class="fixed inset-0 z-40"
      role="dialog"
      aria-modal="true"
      aria-label="知识节点详情"
    >
      <div class="absolute inset-0 bg-scrim" @click="selectedId = null" />
      <Motion
        :initial="{ x: 32, opacity: 0 }"
        :animate="{ x: 0, opacity: 1 }"
        :transition="springSnappy"
        class="absolute inset-y-0 right-0 flex w-full max-w-sm flex-col overflow-y-auto border-l border-hairline bg-surface p-6 shadow-floating"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="text-caption text-label-secondary">图谱节点</p>
            <h2 class="mt-1 text-title-3">{{ selectedNode.title }}</h2>
          </div>
          <button
            type="button"
            class="knowledge-icon-button shrink-0"
            aria-label="关闭"
            @click="selectedId = null"
          >
            ×
          </button>
        </div>

        <dl class="mt-5 grid grid-cols-2 gap-3">
          <div>
            <dt class="text-caption text-label-secondary">来源类型</dt>
            <dd class="mt-1 text-subhead text-label">
              {{ knowledgeSourceLabel(selectedNode.source_type) }}
            </dd>
          </div>
          <div>
            <dt class="text-caption text-label-secondary">连接度</dt>
            <dd class="mt-1 text-subhead text-label">度数 {{ selectedNode.degree }}</dd>
          </div>
        </dl>

        <p v-if="selectedNode.summary" class="mt-4 text-subhead leading-7 text-label-secondary">
          {{ selectedNode.summary }}
        </p>

        <section class="mt-6">
          <h3 class="text-headline">一跳关系</h3>
          <p v-if="oneHopRelations.length === 0" class="mt-2 text-subhead text-label-secondary">
            还没有已确认的一跳关系。
          </p>
          <ul v-else class="mt-3 space-y-2">
            <li
              v-for="relation in oneHopRelations"
              :key="relation.edge.id"
              class="rounded-md border border-hairline p-3"
            >
              <div
                class="flex items-center justify-between gap-2 text-caption text-label-secondary"
              >
                <span>{{ relationTypeLabel(relation.edge.relation_type) }}</span>
                <span>
                  {{ STATUS_LABELS[relation.edge.status] }} ·
                  {{ relation.edge.source === 'manual' ? '手动' : 'AI' }}
                </span>
              </div>
              <RouterLink
                :to="`/home/knowledge/${relation.counterpartId}`"
                class="mt-1 block truncate text-subhead font-medium text-primary hover:underline"
              >
                {{ relation.counterpartTitle }}
              </RouterLink>
            </li>
          </ul>
        </section>

        <div class="mt-6 flex flex-wrap gap-3">
          <button type="button" class="knowledge-button-primary" @click="recenter">
            以此为中心
          </button>
          <RouterLink :to="`/home/knowledge/${selectedNode.id}`" class="knowledge-button-secondary">
            查看详情
          </RouterLink>
        </div>
      </Motion>
    </div>
  </main>
</template>
