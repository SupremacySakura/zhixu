<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref } from 'vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import {
  archiveKnowledge,
  deleteKnowledge,
  favoriteKnowledge,
  retryKnowledgeVectorize,
  unarchiveKnowledge,
  type KnowledgeItem,
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
import { useRouter } from 'vue-router'
import { useKnowledgeLibrary } from '../composables/useKnowledgeLibrary'
import { useKnowledgeImport } from '../composables/useKnowledgeImport'
import { useToast } from '../composables/useToast'
import { knowledgeDate } from '../utils/knowledge'
import KnowledgeActionMenu from '../components/knowledge/KnowledgeActionMenu.vue'
import KnowledgeBadge from '../components/knowledge/KnowledgeBadge.vue'
import KnowledgeVectorBadge from '../components/knowledge/KnowledgeVectorBadge.vue'
import KnowledgeTaskPanel from '../components/knowledge/KnowledgeTaskPanel.vue'
import WebImportDialog from '../components/knowledge/WebImportDialog.vue'
import FilePagination from '../components/file-management/FilePagination.vue'
const {
  items,
  total,
  page,
  search,
  status,
  vector,
  sourceType,
  tagId,
  favorite,
  archived,
  sort,
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
function clearFilters() {
  search.value = ''
  status.value = ''
  vector.value = ''
  sourceType.value = ''
  tagId.value = 0
  favorite.value = ''
  archived.value = ''
}
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
onMounted(() => {
  void load()
  void loadTags()
})
</script>
<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-wide">
      <p class="mb-4 text-subhead text-label-secondary">知识库 / 全部知识</p>
      <header class="mb-7 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="text-title-1">知识库</h1>
          <p class="mt-2 text-subhead text-label-secondary">
            从文件中沉淀的内容，在这里继续阅读与积累。
          </p>
        </div>
        <div class="flex flex-wrap gap-3">
          <RouterLink to="/home/knowledge/new" class="knowledge-button-primary"
            ><AppIcon icon="material-symbols:edit-square-outline" />新建知识</RouterLink
          >
          <RouterLink to="/home/files" class="knowledge-button-secondary"
            ><AppIcon icon="material-symbols:note-add-outline" />从文件加入</RouterLink
          >
          <button type="button" class="knowledge-button-secondary" @click="showWebImport = true">
            <AppIcon icon="material-symbols:language" />从网页导入
          </button>
        </div>
      </header>
      <div class="mb-5 flex flex-wrap gap-3">
        <div class="relative min-w-48 flex-1">
          <AppIcon
            class="absolute left-3 top-3.5 text-label-secondary"
            icon="material-symbols:search"
          /><input
            v-model="search"
            maxlength="200"
            class="knowledge-input w-full pl-10"
            placeholder="搜索知识标题"
            aria-label="搜索知识标题"
          />
        </div>
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
        <button v-if="hasFilters" type="button" class="knowledge-text-action" @click="clearFilters">
          清除筛选
        </button>
      </div>
      <div
        v-if="error"
        class="mb-5 bg-warning-tint p-4 text-subhead text-warning-strong"
        role="alert"
      >
        {{ error }} <button class="knowledge-text-action" @click="load()">重试加载</button>
      </div>
      <p v-if="loading" class="py-8 text-center text-subhead text-label-secondary" role="status">
        正在加载知识…
      </p>
      <div
        v-else-if="!error && !items.length"
        class="border border-hairline bg-surface px-6 py-16 text-center"
      >
        <span
          class="mx-auto mb-5 flex h-16 w-14 items-center justify-center bg-primary-tint text-headline font-bold text-primary-pressed"
          >MD</span
        >
        <h2 class="text-title-3">
          {{ hasFilters ? '没有匹配的知识' : '让第一份资料成为知识' }}
        </h2>
        <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
          {{
            hasFilters
              ? '试试其他关键词或切换筛选状态。'
              : '选择已经上传的 Markdown 文件，保留原件，同时获得清晰可读的知识正文。'
          }}
        </p>
        <RouterLink to="/home/files" class="knowledge-button-primary mt-5"
          >去文件管理选择</RouterLink
        >
      </div>
      <div v-else-if="!loading" class="space-y-3">
        <article
          v-for="entry in items"
          :key="entry.id"
          class="flex items-start justify-between gap-3 border border-hairline bg-surface p-5 sm:items-center sm:gap-4 sm:p-6"
        >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <KnowledgeBadge :status="entry.status" />
              <KnowledgeVectorBadge
                v-if="hasKnowledgeVector(entry)"
                :state="entry.vector_state as Exclude<KnowledgeVectorState, ''>"
              />
            </div>
            <h2 class="mb-2 mt-3 break-all text-headline">{{ entry.title }}</h2>
            <p v-if="entry.summary" class="mb-2 line-clamp-2 text-subhead leading-6 text-label">
              {{ entry.summary }}
            </p>
            <p class="text-subhead leading-6 text-label-secondary">
              {{ knowledgeDescription(entry) }}
            </p>
            <div class="mt-3 flex flex-wrap gap-3 text-caption text-label-secondary">
              <span
                >{{ knowledgeSourceLabel(entry.source_type)
                }}{{ entry.source_file_id ? '文件' : '' }}</span
              ><span v-if="entry.source_file_id" class="break-all">{{
                entry.source_file_name
              }}</span
              ><span v-if="entry.archived_at" class="text-warning">已归档</span
              ><time>{{ knowledgeDate(entry.created_at) }}</time>
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button
              type="button"
              class="knowledge-icon-button flex h-11 w-11 items-center justify-center border border-hairline bg-surface text-label-secondary"
              :class="entry.is_favorite ? 'text-warning' : ''"
              :aria-label="entry.is_favorite ? `取消收藏 ${entry.title}` : `收藏 ${entry.title}`"
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
                :style="{ '--atom-glyph-size': '20px' }"
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
      <p class="mt-5 text-caption leading-6 text-label-secondary">
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
      <p v-if="deleteError" role="alert" class="mb-4 text-subhead text-danger">{{ deleteError }}</p>
    </ConfirmDialog>
  </main>
</template>
