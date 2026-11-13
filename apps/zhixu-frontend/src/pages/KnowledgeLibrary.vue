<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, watch } from 'vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import AppFilterBar from '../components/ui/AppFilterBar.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import {
  archiveKnowledge,
  deleteKnowledge,
  favoriteKnowledge,
  listKnowledgeRelations,
  retryKnowledgeVectorize,
  unarchiveKnowledge,
  type KnowledgeItem,
  type KnowledgeRelation,
  type KnowledgeSort,
  type KnowledgeStatus,
  type KnowledgeVectorState,
} from '../services/apis/knowledge'
import {
  KNOWLEDGE_FORMATS,
  hasKnowledgeVector,
  knowledgeDescription,
  knowledgeError,
  knowledgeSourceLabel,
} from '../utils/knowledge'
import AppIcon from '../components/ui/AppIcon.vue'
import AppButton from '../components/ui/AppButton.vue'
import { useRouter } from 'vue-router'
import { useKnowledgeLibrary } from '../composables/useKnowledgeLibrary'
import { useKnowledgeImport } from '../composables/useKnowledgeImport'
import { useToast } from '../composables/useToast'
import { knowledgeDate } from '../utils/knowledge'
import { RELATION_TYPE_LABELS } from '../utils/relation'
import KnowledgeActionMenu from '../components/knowledge/KnowledgeActionMenu.vue'
import KnowledgeBadge from '../components/knowledge/KnowledgeBadge.vue'
import KnowledgeTableView from '../components/knowledge/KnowledgeTableView.vue'
import KnowledgeVectorBadge from '../components/knowledge/KnowledgeVectorBadge.vue'
import KnowledgeTaskPanel from '../components/knowledge/KnowledgeTaskPanel.vue'
import WebImportDialog from '../components/knowledge/WebImportDialog.vue'
import FilePagination from '../components/file-management/FilePagination.vue'
const {
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
} = useKnowledgeLibrary()
const { displayToast } = useToast()
const { item, panelOpen, panelError, busy, track, retry, poll } = useKnowledgeImport(displayToast)
const router = useRouter()
const showWebImport = ref(false)
function onWebImported(created: KnowledgeItem) {
  showWebImport.value = false
  displayToast('已从网页创建知识，正在自动分块向量化', 'success')
  void router.push(`/home/knowledge/${created.id}`)
}
const statusOptions = [
  { key: '', label: '全部状态' },
  { key: 'ready', label: '已加入' },
  { key: 'pending', label: '排队中' },
  { key: 'running', label: '处理中' },
  { key: 'failed', label: '加入失败' },
]
const vectorOptions = [
  { key: '', label: '全部向量状态' },
  { key: 'ready', label: '已向量化' },
  { key: 'running', label: '向量化中' },
  { key: 'partial', label: '部分失败' },
  { key: 'pending', label: '待向量化' },
]
const sourceOptions = [
  { key: '', label: '全部来源' },
  { key: 'manual', label: '手动创建' },
  { key: 'web', label: '网页导入' },
  ...KNOWLEDGE_FORMATS.map(format => ({ key: format.type, label: format.label })),
]
const tagOptions = computed(() => [
  { key: '', label: '全部标签' },
  ...tags.value.map(tag => ({ key: String(tag.id), label: tag.name })),
])
const favoriteOptions = [
  { key: '', label: '全部收藏' },
  { key: 'true', label: '仅收藏' },
]
const archivedOptions = [
  { key: '', label: '默认列表' },
  { key: 'true', label: '仅已归档' },
]
const sortOptions = [
  { key: 'created', label: '最近创建' },
  { key: 'updated', label: '最近更新' },
  { key: 'title', label: '标题排序' },
]
const hasFilters = computed(() =>
  Boolean(
    search.value ||
    status.value ||
    vector.value ||
    sourceType.value ||
    tagId.value ||
    favorite.value ||
    archived.value,
  ),
)
/**
 * 窄屏收起按钮上的角标数：只数**下拉筛选**，不数搜索框。
 * 搜索框在窄屏仍然常驻（它是最高频的操作，藏进面板要多点两下），
 * 而角标说的是「还有几个筛选条件在生效」—— 把搜索算进去会让用户
 * 在清空搜索后仍看到「筛选 1」，找不到那 1 是什么。
 */
const activeFilterCount = computed(
  () =>
    [
      status.value,
      vector.value,
      sourceType.value,
      tagId.value,
      favorite.value,
      archived.value,
    ].filter(Boolean).length,
)

function clearFilters() {
  search.value = ''
  status.value = ''
  vector.value = ''
  sourceType.value = ''
  tagId.value = 0
  favorite.value = ''
  archived.value = ''
}
/** 卡片左上的分类号（DESIGN.md §Layout 招牌动作「左上分类号 KM-0142」）。
 *  从 id 生成而不是另存字段：id 是唯一稳定的序号，重启、重建索引都不变；
 *  另外造一个显示序号反而会在删除后出现空洞。 */
const knowledgeSerial = (id: number) => `KM-${String(id).padStart(4, '0')}`

const deleteTarget = ref<KnowledgeItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')
let deleteController: AbortController | null = null
function selectDelete(entry: KnowledgeItem) {
  deleteTarget.value = entry
  deleteError.value = ''
}
async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  deleteController = new AbortController()
  try {
    await deleteKnowledge(deleteTarget.value.id, deleteController.signal)
    if (deleteController.signal.aborted) return
    if (item.value?.id === deleteTarget.value.id) {
      panelOpen.value = false
      item.value = null
    }
    deleteTarget.value = null
    displayToast('知识已删除，原文件保持不变', 'success')
    await load()
  } catch (e) {
    if (!deleteController.signal.aborted) deleteError.value = knowledgeError(e)
  } finally {
    deleting.value = false
  }
}
onScopeDispose(() => deleteController?.abort())
const favoriteBusy = new Set<number>()
async function toggleFavorite(entry: KnowledgeItem) {
  if (favoriteBusy.has(entry.id)) return
  favoriteBusy.add(entry.id)
  try {
    const state = await favoriteKnowledge(entry.id, !entry.is_favorite)
    items.value = items.value.map(current =>
      current.id === entry.id ? { ...current, is_favorite: state.is_favorite } : current,
    )
  } catch (e) {
    displayToast(knowledgeError(e), 'error')
  } finally {
    favoriteBusy.delete(entry.id)
  }
}
const archiving = ref(false)
async function archiveEntry(entry: KnowledgeItem) {
  if (archiving.value) return
  archiving.value = true
  try {
    await archiveKnowledge(entry.id)
    displayToast('知识已归档，默认列表不再显示', 'success')
    await load()
  } catch (e) {
    displayToast(knowledgeError(e), 'error')
  } finally {
    archiving.value = false
  }
}
async function unarchiveEntry(entry: KnowledgeItem) {
  if (archiving.value) return
  archiving.value = true
  try {
    await unarchiveKnowledge(entry.id)
    displayToast('已恢复归档', 'success')
    await load()
  } catch (e) {
    displayToast(knowledgeError(e), 'error')
  } finally {
    archiving.value = false
  }
}
const vectorRetryControllers = new Map<number, AbortController>()
onScopeDispose(() => {
  vectorRetryControllers.forEach(controller => controller.abort())
})
async function retryVector(entry: KnowledgeItem) {
  if (vectorRetryControllers.has(entry.id)) return
  const controller = new AbortController()
  vectorRetryControllers.set(entry.id, controller)
  try {
    const updated = await retryKnowledgeVectorize(entry.id, controller.signal)
    if (controller.signal.aborted) return
    items.value = items.value.map(current => (current.id === entry.id ? updated : current))
    if (item.value?.id === entry.id) item.value = updated
    displayToast('失败分块已重新排队，向量化稍后自动继续', 'success')
    await load()
  } catch (e) {
    if (!controller.signal.aborted) displayToast(knowledgeError(e), 'error')
  } finally {
    vectorRetryControllers.delete(entry.id)
  }
}
function closePanel() {
  panelOpen.value = false
  void load()
}

/**
 * 交叉引用纸条（#40 R1）。
 *
 * 为什么是逐卡并发 `listKnowledgeRelations`：票面 AC 要求「不用点开就能看到
 * 这张卡依赖哪一条、和哪一条矛盾」。图谱接口只返回最近 50 个节点之间的边，
 * 关联对象不在那 50 条里时边根本不在响应里 —— 那会渲染出一句
 * 「暂无交叉引用」的假话。逐卡取关系才是诚实的：拿到什么就说什么。
 *
 * 一页 20 条，`Promise.all` 一次并发打完；20 个短请求远比串行快，
 * 也比「先等列表、再逐条补」少一轮闪烁。
 *
 * `AbortController` 是必须的：翻页/筛选会重新拉列表，上一页那 20 个请求
 * 回来时若还往 map 里写，就会把 A 卡的关系贴到 B 卡上。这里在每次重拉前
 * abort 整批，并在写入前再查一次 `signal.aborted`（abort 与 await 之间存在
 * 竞态窗口，光 abort 不够）。
 */
type RelationSummary = { total: number; slips: { type: string; title: string }[]; failed?: boolean }
const relations = ref<Record<number, RelationSummary>>({})
let relationsController: AbortController | null = null
/** 纸条上最多印几条：卡片高度 118px，纸条 17px + 2px 间隙，三张刚好贴满右侧。 */
const SLIP_LIMIT = 3

function summarize(
  confirmed: KnowledgeRelation[],
  suggested: KnowledgeRelation[],
): RelationSummary {
  // 已确认的排前面：那是「事实」；建议只是「AI 觉得可能」，不该抢占第一条。
  const all = [...confirmed, ...suggested]
  return {
    total: all.length,
    // 类别文案全部来自 RELATION_TYPE_LABELS，本页不发明新的中文类别名；
    // 对端标题用后端算好的 counterpart_title，不再为一张纸条多打一次接口。
    slips: all.slice(0, SLIP_LIMIT).map(relation => ({
      type: RELATION_TYPE_LABELS[relation.relation_type],
      title: relation.counterpart_title,
    })),
  }
}

async function loadRelations() {
  relationsController?.abort()
  const current = new AbortController()
  relationsController = current
  const snapshot = items.value
  // 先清空：上一页的纸条不能挂在新的卡片上。
  relations.value = {}
  const results = await Promise.all(
    snapshot.map(async entry => {
      try {
        const data = await listKnowledgeRelations(entry.id, current.signal)
        if (current.signal.aborted) return null
        return [entry.id, summarize(data.confirmed, data.suggested)] as const
      } catch {
        // abort 引发的 rejection 不是「失败」，是「这批作废了」：返回 null 让
        // 整批结果一起丢弃，别把「翻页了」渲染成「读取失败」。
        if (current.signal.aborted) return null
        // 单卡失败不拖垮整页，但**也不能伪装成「没有关系」**。
        // 后端限流（60 req/min/IP）或单个 500 都会走到这里；把失败标出来，
        // 让那张卡说「读取失败」而不是「暂无交叉引用」—— 后者是假阴性，
        // 而 R1 拒绝图谱接口的全部理由就是不肯接受假阴性。
        return [
          entry.id,
          { total: 0, slips: [] as RelationSummary['slips'], failed: true },
        ] as const
      }
    }),
  )
  if (current.signal.aborted) return
  const next: Record<number, RelationSummary> = {}
  for (const result of results) if (result) next[result[0]] = result[1]
  relations.value = next
}
// 列表内容变了就重取纸条。挂在**条目 id 列表**而不是 items 数组上：
// useKnowledgeLibrary 在有条目还在处理时每 3s 轮询一次 load(true)，每次都会换掉
// items 数组的引用，但页面上的卡片还是那 20 条。挂在数组上会让轮询每 3 秒
// 重新打 20 个 relations 请求（60 req/min/IP 的后端限流会直接 429，
// 于是本该显示「依赖 X」的卡片退化成「暂无交叉引用」—— 假阴性）。
// 关系只取决于 id，所以按 id 串去重：翻页、筛选、删除会改 id 串，轮询不会。
watch(
  () => items.value.map(entry => entry.id).join(','),
  () => void loadRelations(),
  { immediate: true },
)
onScopeDispose(() => relationsController?.abort())
onMounted(() => {
  void load()
  void loadTags()
})
</script>
<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <!-- 页头带 64px：左标题 + 描述行 + 等宽计数，右 1 主 + 2 次操作（DESIGN.md 四段结构第 1 段）。
         面包屑已取消 —— 侧栏已指示位置，索引页再写一遍是浪费 24px。
         描述行「从文件中沉淀的内容，在这里继续阅读与积累。」是这一页本来就有的文案，
         迁移时被误删（497267c 无背书删除），现按审查裁定恢复。
         它不单占一行：与等宽计数同行（计数靠右），整条带仍是两行 43px
         （标题 19/26 + 描述 12/17），在 64px 里上下各留 10px。 -->
    <header class="shell-band">
      <div class="min-w-0 flex-1">
        <h1 class="truncate text-title">知识库</h1>
        <div class="flex min-w-0 items-baseline gap-3">
          <p class="min-w-0 truncate text-note text-label-secondary">
            从文件中沉淀的内容，在这里继续阅读与积累。
          </p>
          <p
            v-if="!loading"
            data-test="band-micro"
            class="ml-auto flex-none whitespace-nowrap font-type text-micro text-label-tertiary"
          >
            索引 {{ totalAll || total }}
          </p>
        </div>
      </div>
      <!-- 主次上限 1+2：新建知识是主操作，恒在；另两个是次操作。
           375px 下三个带字的按钮必然溢出视口 —— 收成图标，功能一个不少。
           文案留在 DOM 里（只做视觉隐藏），屏幕阅读器与文案断言都还能读到。 -->
      <div class="flex flex-none items-center gap-2">
        <RouterLink to="/home/files" class="shell-band-action" title="从文件加入"
          ><AppIcon icon="material-symbols:note-add-outline" /><span class="hidden sm:inline"
            >从文件加入</span
          ></RouterLink
        >
        <button
          type="button"
          class="shell-band-action"
          title="从网页导入"
          @click="showWebImport = true"
        >
          <AppIcon icon="material-symbols:language" /><span class="hidden sm:inline"
            >从网页导入</span
          >
        </button>
        <RouterLink to="/home/knowledge/new" class="shell-band-action-primary" title="新建知识"
          ><AppIcon icon="material-symbols:edit-square-outline" /><span class="hidden sm:inline"
            >新建知识</span
          ></RouterLink
        >
      </div>
    </header>

    <main class="shell-page">
      <div class="shell-wide">
        <div class="shell-toolbar mb-5">
          <div class="relative min-w-48 flex-1">
            <AppIcon
              class="absolute left-3 top-3.5 text-label-secondary"
              icon="material-symbols:search"
            /><input
              v-model="search"
              maxlength="200"
              class="atom-field min-h-9 w-full pl-10"
              placeholder="搜索知识标题"
              aria-label="搜索知识标题"
            />
          </div>
          <AppFilterBar :active-count="activeFilterCount">
            <FilterMenu
              :model-value="status"
              :options="statusOptions"
              label="筛选处理状态"
              @update:model-value="status = $event as KnowledgeStatus | ''"
            />
            <FilterMenu
              :model-value="vector"
              :options="vectorOptions"
              label="筛选向量状态"
              @update:model-value="vector = $event as KnowledgeVectorState"
            />
            <FilterMenu
              :model-value="sourceType"
              :options="sourceOptions"
              label="筛选来源类型"
              @update:model-value="sourceType = $event"
            />
            <FilterMenu
              :model-value="tagId ? String(tagId) : ''"
              :options="tagOptions"
              label="筛选标签"
              @update:model-value="tagId = Number($event) || 0"
            />
            <FilterMenu
              :model-value="favorite"
              :options="favoriteOptions"
              label="筛选收藏"
              @update:model-value="favorite = $event as '' | 'true'"
            />
            <FilterMenu
              :model-value="archived"
              :options="archivedOptions"
              label="筛选归档"
              @update:model-value="archived = $event as '' | 'true'"
            />
            <FilterMenu
              :model-value="sort"
              :options="sortOptions"
              label="排序方式"
              @update:model-value="sort = $event as KnowledgeSort"
            />
            <button v-if="hasFilters" type="button" class="atom-text-action" @click="clearFilters">
              清除筛选
            </button>
          </AppFilterBar>
          <!-- 视图切换（#35 用户故事 4）：卡片 / 表格。两枚 26px 方形图标按钮，
               相邻密度靠 pointer-coarse:gap-3 在触摸屏上撑开 44px 热区间隙。
               放在命中计数**左侧**：计数是工具栏右端的固定锚点（DESIGN.md
               §Layout「48px 吸顶工具栏右端放命中 9 / 142」），切换器贴着它左边，
               两个都是「关于这一屏的读数」，不跟左侧的搜索筛选混在一起。 -->
          <div class="ml-auto flex flex-none items-center gap-1 pointer-coarse:gap-3">
            <AppButton
              size="sm"
              icon-only
              :tone="viewMode === 'card' ? 'primary' : 'secondary'"
              icon="material-symbols:grid-view"
              aria-label="卡片"
              :aria-pressed="viewMode === 'card'"
              @click="viewMode = 'card'"
            />
            <AppButton
              size="sm"
              icon-only
              :tone="viewMode === 'table' ? 'primary' : 'secondary'"
              icon="material-symbols:list"
              aria-label="表格"
              :aria-pressed="viewMode === 'table'"
              @click="viewMode = 'table'"
            />
          </div>
          <p
            data-test="hit-count"
            class="flex-none whitespace-nowrap font-type text-micro text-label-tertiary"
          >
            <template v-if="hasFilters && totalAll && totalAll !== total">
              命中 {{ total }} / {{ totalAll }}
            </template>
            <template v-else>命中 {{ total }}</template>
          </p>
        </div>
        <div
          v-if="error"
          class="mb-5 bg-warning-tint p-4 text-body text-warning-strong"
          role="alert"
        >
          {{ error }} <button class="atom-text-action" @click="load()">重试加载</button>
        </div>
        <p v-if="loading" class="py-8 text-center text-body text-label-secondary" role="status">
          正在加载知识…
        </p>
        <div
          v-else-if="!error && !items.length"
          class="border border-hairline bg-surface px-6 py-16 text-center"
        >
          <span
            class="mx-auto mb-5 flex h-16 w-14 items-center justify-center bg-primary-tint text-lead font-bold text-primary-pressed"
            >MD</span
          >
          <h2 class="text-title">
            {{ hasFilters ? '没有匹配的知识' : '让第一份资料成为知识' }}
          </h2>
          <p class="mx-auto mt-3 max-w-sm text-body leading-7 text-label-secondary">
            {{
              hasFilters
                ? '试试其他关键词或切换筛选状态。'
                : '选择已经上传的 Markdown 文件，保留原件，同时获得清晰可读的知识正文。'
            }}
          </p>
          <RouterLink to="/home/files" class="atom-btn atom-tone-primary atom-btn-lg mt-5"
            >去文件管理选择</RouterLink
          >
        </div>
        <!-- 表格视图（#35 用户故事 4）：同一批条目、同一份关系摘要，
             只换排布，不重新请求。分页条与页脚说明两视图共用。 -->
        <KnowledgeTableView
          v-else-if="!loading && viewMode === 'table'"
          :items="items"
          :relations="relations"
        />
        <div v-else-if="!loading" class="space-y-3">
          <!-- 118px 硬卡（DESIGN.md §Layout 原型①「知识库为 118px 硬卡」）。
               卡面按物理索引卡的顺序排：左上分类号 → 标题 → 摘要（压蓝格线）
               → 右下日期章；状态与向量态退成卡角的小方章。
               右侧纸条是**这一页独占**的招牌动作（§Layout 独占条款），
               所以它写在卡片结构里，不抽成通用组件 —— 一抽出去就会被别的页复制。 -->
          <article
            v-for="entry in items"
            :key="entry.id"
            class="hard-card flex-col gap-2 p-3 sm:flex-row sm:items-center sm:gap-4 sm:px-4 sm:py-3"
          >
            <div class="flex min-w-0 flex-1 flex-col justify-center">
              <!-- 顶边一行：分类号 + 状态方章 + 标题 -->
              <div class="flex min-w-0 items-center gap-2">
                <span class="flex-none font-type text-micro text-label-tertiary">{{
                  knowledgeSerial(entry.id)
                }}</span>
                <KnowledgeBadge :status="entry.status" />
                <KnowledgeVectorBadge
                  v-if="hasKnowledgeVector(entry)"
                  :state="entry.vector_state as Exclude<KnowledgeVectorState, ''>"
                />
                <h2 class="min-w-0 flex-1 truncate text-lead text-label" :title="entry.title">
                  {{ entry.title }}
                </h2>
                <div class="flex flex-none items-center gap-1">
                  <button
                    type="button"
                    class="atom-icon-btn flex h-8 w-8 items-center justify-center border border-hairline bg-surface text-label-secondary"
                    :class="entry.is_favorite ? 'text-warning' : ''"
                    :aria-label="
                      entry.is_favorite ? `取消收藏 ${entry.title}` : `收藏 ${entry.title}`
                    "
                    :aria-pressed="entry.is_favorite"
                    @click="toggleFavorite(entry)"
                  >
                    <AppIcon
                      :icon="
                        entry.is_favorite
                          ? 'material-symbols:star-rounded'
                          : 'material-symbols:star-outline'
                      "
                      class="atom-glyph"
                      aria-hidden="true"
                      :style="{ '--atom-glyph-size': '16px' }"
                    />
                  </button>
                  <KnowledgeActionMenu
                    :item="entry"
                    @track="track"
                    @delete="selectDelete"
                    @retry-vector="retryVector"
                    @archive="archiveEntry"
                    @unarchive="unarchiveEntry"
                  />
                </div>
              </div>
              <!-- 摘要压在蓝色格线上：格线是世界的母题，不是分隔线，
                   所以用 --color-grid（30% 蓝）而不是 hairline。 -->
              <p class="card-ruled mt-1 line-clamp-1 text-body text-label-secondary">
                {{ entry.summary || knowledgeDescription(entry) }}
              </p>
              <!-- 右下日期章 + 来源 -->
              <div class="mt-1 flex min-w-0 items-center gap-3 font-type text-micro">
                <span class="flex-none text-label-tertiary">{{
                  knowledgeSourceLabel(entry.source_type)
                }}</span>
                <span v-if="entry.archived_at" class="flex-none text-warning">已归档</span>
                <time class="ml-auto flex-none text-label-tertiary">{{
                  knowledgeDate(entry.created_at)
                }}</time>
              </div>
            </div>
            <!-- 交叉引用纸条：桌面在卡右侧，窄屏落到卡内下方（DESIGN §Layout
                 「窄屏的结构性降级」）。纸条逐张 +5px 阶梯缩进，读起来是一叠
                 手写小卡，而不是一张表。 -->
            <div
              class="relation-slip-stack flex-none sm:w-40"
              :aria-label="`交叉引用 ${entry.title}`"
            >
              <template v-if="relations[entry.id]?.total">
                <span
                  v-for="(slip, index) in relations[entry.id]!.slips"
                  :key="index"
                  class="relation-slip"
                  :style="{ marginRight: `${index * 5}px` }"
                >
                  <span class="relation-slip-type">{{ slip.type }}</span>
                  <span class="truncate">{{ slip.title }}</span>
                </span>
                <span
                  v-if="relations[entry.id]!.total > SLIP_LIMIT"
                  class="flex-none font-type text-micro text-label-tertiary"
                  >另有 {{ relations[entry.id]!.total - SLIP_LIMIT }} 条</span
                >
              </template>
              <!-- 「取不到」和「确实没有」必须分开说：后者是结论，前者只是这次没读到。 -->
              <span
                v-else-if="relations[entry.id]?.failed"
                class="relation-slip text-warning-strong"
                title="交叉引用读取失败，刷新页面可重试"
                >读取失败</span
              >
              <span v-else class="relation-slip text-label-tertiary">暂无交叉引用</span>
            </div>
          </article>
        </div>
        <FilePagination
          v-if="total > 0"
          class="mt-5"
          :page="page"
          :total-pages="Math.max(1, Math.ceil(total / 20))"
          :start-index="(page - 1) * 20 + 1"
          :end-index="Math.min(page * 20, total)"
          :total-count="total"
          :loading="loading"
          @change="changePage"
        />
        <p class="mt-5 text-note leading-6 text-label-secondary">
          列表摘要来自 AI 整理结果，需在详情页确认采纳后才会展示。
        </p>
      </div>
      <KnowledgeTaskPanel
        :item="item"
        :open="panelOpen"
        :error="panelError"
        :busy="busy"
        @close="closePanel"
        @retry="retry"
        @refresh="poll"
      />
      <WebImportDialog
        v-if="showWebImport"
        @close="showWebImport = false"
        @imported="onWebImported"
      />
      <ConfirmDialog
        :model-value="Boolean(deleteTarget)"
        title="删除知识条目"
        :message="`将删除「${deleteTarget?.title || ''}」及其处理任务，原文件保持不变。此操作不可撤销。`"
        confirm-text="删除知识"
        danger
        :loading="deleting"
        @cancel="deleteTarget = null"
        @confirm="confirmDelete"
      >
        <p v-if="deleteError" role="alert" class="mb-4 text-body text-danger">
          {{ deleteError }}
        </p>
      </ConfirmDialog>
    </main>
  </div>
</template>
