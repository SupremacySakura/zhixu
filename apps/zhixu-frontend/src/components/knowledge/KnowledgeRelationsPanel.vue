<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Icon } from '@iconify/vue'
import FilterMenu from '../ui/FilterMenu.vue'
import {
  confirmKnowledgeRelation,
  createKnowledgeRelation,
  deleteKnowledgeRelation,
  listKnowledge,
  listKnowledgeRelations,
  regenerateKnowledgeRelations,
  rejectKnowledgeRelation,
  updateKnowledgeRelationType,
  type KnowledgeItem,
  type KnowledgeRelation,
  type RelationTask,
  type RelationType,
} from '../../services/apis/knowledge'
import { knowledgeError } from '../../utils/knowledge'
import { RELATION_TYPE_OPTIONS, relationTypeLabel } from '../../utils/relation'

const props = defineProps<{ item: KnowledgeItem }>()

/** 任务 pending/running 时的轮询间隔。 */
const POLL_INTERVAL = 3_000

const confirmed = ref<KnowledgeRelation[]>([])
const suggested = ref<KnowledgeRelation[]>([])
const task = ref<RelationTask | null>(null)
const loading = ref(false)
const error = ref('')
const busy = ref('')
const expandedId = ref<number | null>(null)
const query = ref('')
const searching = ref(false)
const results = ref<KnowledgeItem[]>([])
const searchHint = ref('')
const target = ref<KnowledgeItem | null>(null)
const relationType = ref<RelationType>('related')
// 删除是物理删除：先点删除只进入待确认，再点确认才真正调用接口。
const pendingDelete = ref<number | null>(null)
let controller: AbortController | null = null
let searchController: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined

const total = computed(() => confirmed.value.length + suggested.value.length)
const running = computed(() => task.value?.status === 'pending' || task.value?.status === 'running')
const hasSuggestions = computed(() => suggested.value.length > 0)
/** 关系类型下拉复用项目内的 FilterMenu，选项转成它要求的字符串键。 */
const relationTypeOptions = RELATION_TYPE_OPTIONS.map(option => ({
  key: option.value,
  label: option.label,
}))
/** 手动添加的关系类型：只接受关系类型表里的值。 */
function setRelationType(value: string) {
  if (relationTypeOptions.some(option => option.key === value)) {
    relationType.value = value as RelationType
  }
}

interface RelationEvidence {
  reason: string
  source_quote: string
  target_quote: string
}

/** 解析 #3 落库的证据 JSON；历史行可能不是 JSON，退化为纯理由文本。 */
function evidenceOf(relation: KnowledgeRelation): RelationEvidence | null {
  const raw = relation.evidence?.trim()
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<RelationEvidence>
    if (!parsed || typeof parsed !== 'object')
      return { reason: raw, source_quote: '', target_quote: '' }
    return {
      reason: parsed.reason ?? '',
      source_quote: parsed.source_quote ?? '',
      target_quote: parsed.target_quote ?? '',
    }
  } catch {
    return { reason: raw, source_quote: '', target_quote: '' }
  }
}

function confidencePercent(relation: KnowledgeRelation) {
  return `${Math.round(relation.confidence * 100)}%`
}

function toggleEvidence(id: number) {
  expandedId.value = expandedId.value === id ? null : id
}

function stopPolling() {
  if (timer !== undefined) {
    clearTimeout(timer)
    timer = undefined
  }
}

function schedulePoll() {
  stopPolling()
  if (!running.value) return
  timer = setTimeout(() => {
    timer = undefined
    void load(true)
  }, POLL_INTERVAL)
}

async function load(background = false) {
  controller?.abort()
  const current = new AbortController()
  controller = current
  if (!background) loading.value = true
  error.value = ''
  try {
    const data = await listKnowledgeRelations(props.item.id, current.signal)
    if (current.signal.aborted) return
    confirmed.value = data.confirmed
    suggested.value = data.suggested
    task.value = data.task
  } catch (issue) {
    if (!current.signal.aborted) error.value = knowledgeError(issue)
  } finally {
    if (controller === current) {
      controller = null
      if (!background) loading.value = false
      schedulePoll()
    }
  }
}

async function regenerate() {
  if (running.value || busy.value === 'regenerate') return
  busy.value = 'regenerate'
  error.value = ''
  try {
    const data = await regenerateKnowledgeRelations(props.item.id)
    task.value = data.task
    schedulePoll()
    // 立刻回读一次，让新任务与既有建议在面板上同步。
    await load(true)
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    if (busy.value === 'regenerate') busy.value = ''
  }
}

async function confirm(relation: KnowledgeRelation) {
  if (busy.value) return
  busy.value = `confirm:${relation.id}`
  error.value = ''
  try {
    const updated = await confirmKnowledgeRelation(relation.id)
    suggested.value = suggested.value.filter(entry => entry.id !== relation.id)
    // 确认响应不带对端信息，沿用列表里的对端字段。
    confirmed.value = [
      {
        ...relation,
        ...updated,
        counterpart_id: relation.counterpart_id,
        counterpart_title: relation.counterpart_title,
      },
      ...confirmed.value,
    ]
    if (expandedId.value === relation.id) expandedId.value = null
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

async function reject(relation: KnowledgeRelation) {
  if (busy.value) return
  busy.value = `reject:${relation.id}`
  error.value = ''
  try {
    await rejectKnowledgeRelation(relation.id)
    suggested.value = suggested.value.filter(entry => entry.id !== relation.id)
    if (expandedId.value === relation.id) expandedId.value = null
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

async function changeType(relation: KnowledgeRelation, value: string) {
  const next = value as RelationType
  if (!relationTypeOptions.some(option => option.key === next)) return
  if (next === relation.relation_type || busy.value) return
  busy.value = `type:${relation.id}`
  error.value = ''
  try {
    const updated = await updateKnowledgeRelationType(relation.id, next)
    confirmed.value = confirmed.value.map(entry =>
      entry.id === relation.id ? { ...entry, relation_type: updated.relation_type } : entry,
    )
  } catch (issue) {
    error.value = knowledgeError(issue)
    // 失败时回读，把下拉框恢复成服务端类型。
    await load(true)
  } finally {
    busy.value = ''
  }
}

async function search() {
  const keyword = query.value.trim()
  if (!keyword) return
  searchController?.abort()
  const current = new AbortController()
  searchController = current
  searching.value = true
  searchHint.value = ''
  results.value = []
  try {
    const page = await listKnowledge(1, keyword, '', '', {}, current.signal)
    if (current.signal.aborted) return
    // 只能连到同用户、未归档的其他知识；已有的对端不再重复列出。
    const known = new Set(
      [...confirmed.value, ...suggested.value].map(relation => relation.counterpart_id),
    )
    results.value = page.items.filter(
      candidate =>
        candidate.id !== props.item.id && !candidate.archived_at && !known.has(candidate.id),
    )
    if (!results.value.length) searchHint.value = '没有找到可添加的知识'
  } catch (issue) {
    if (!current.signal.aborted) searchHint.value = knowledgeError(issue)
  } finally {
    if (searchController === current) {
      searchController = null
      searching.value = false
    }
  }
}

function choose(candidate: KnowledgeItem) {
  target.value = candidate
  results.value = []
  searchHint.value = ''
}

function resetForm() {
  target.value = null
  results.value = []
  searchHint.value = ''
}

async function add() {
  if (!target.value || busy.value) return
  busy.value = 'create'
  error.value = ''
  try {
    const created = await createKnowledgeRelation(props.item.id, {
      target_id: target.value.id,
      relation_type: relationType.value,
    })
    confirmed.value = [created, ...confirmed.value]
    query.value = ''
    resetForm()
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

async function remove(relation: KnowledgeRelation) {
  busy.value = `delete:${relation.id}`
  error.value = ''
  try {
    await deleteKnowledgeRelation(relation.id)
    confirmed.value = confirmed.value.filter(entry => entry.id !== relation.id)
    pendingDelete.value = null
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

watch(
  () => props.item.id,
  () => {
    stopPolling()
    confirmed.value = []
    suggested.value = []
    task.value = null
    expandedId.value = null
    pendingDelete.value = null
    void load()
  },
  { immediate: true },
)

onScopeDispose(() => {
  stopPolling()
  controller?.abort()
  controller = null
  searchController?.abort()
  searchController = null
})
</script>

<template>
  <section class="rounded-lg border border-hairline bg-surface p-5">
    <div class="mb-3 flex items-start justify-between gap-2">
      <div class="min-w-0">
        <h2 class="text-body font-semibold">相关知识</h2>
        <p class="mt-0.5 flex items-center gap-2 text-caption text-label-tertiary">
          <span>AI 推荐与手动维护</span>
          <span
            v-if="total"
            class="rounded-full bg-surface-secondary px-2 py-0.5 text-label-secondary"
            >{{ total }} 条</span
          >
        </p>
      </div>
      <RouterLink
        :to="`/home/knowledge/graph?center=${item.id}`"
        class="knowledge-text-action min-h-8 shrink-0 text-caption"
        >在图谱中查看</RouterLink
      >
    </div>

    <div class="rounded-md border border-hairline bg-surface-secondary p-3">
      <div class="flex items-center gap-2">
        <span
          class="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary-tint text-primary"
        >
          <Icon icon="material-symbols:auto-awesome" class="text-[14px]" />
        </span>
        <p class="min-w-0 flex-1 text-caption font-semibold text-label">让 AI 找相关知识</p>
      </div>
      <p class="mt-2 text-caption leading-5 text-label-secondary">
        按标题与摘要比对知识库，只生成待确认建议；你确认后才建立关系。
      </p>
      <button
        type="button"
        class="knowledge-button-tint mt-3 min-h-9 w-full px-3 text-caption"
        :disabled="running || busy === 'regenerate'"
        @click="regenerate"
      >
        <Icon
          :icon="
            running
              ? 'material-symbols:progress-activity'
              : hasSuggestions
                ? 'material-symbols:refresh'
                : 'material-symbols:search'
          "
          class="text-[16px]"
          :class="running ? 'animate-spin' : ''"
        />
        {{
          running
            ? '正在查找…'
            : busy === 'regenerate'
              ? '正在提交…'
              : hasSuggestions
                ? '重新查找'
                : '查找相关知识'
        }}
      </button>
      <p v-if="running" class="mt-2 text-caption leading-5 text-label-tertiary" role="status">
        AI 正在比对知识库，通常需要 10–30 秒，结果会自动刷新。
      </p>
      <div
        v-if="task?.status === 'failed'"
        class="mt-2.5 rounded-sm bg-danger-tint p-2.5"
        role="alert"
      >
        <p class="break-words text-caption leading-5 text-danger">
          {{ task.error_message || '查找失败，请稍后重试。' }}
        </p>
        <button
          type="button"
          class="knowledge-text-action min-h-8 text-caption"
          :disabled="busy === 'regenerate'"
          @click="regenerate"
        >
          重试查找
        </button>
      </div>
    </div>

    <p v-if="loading" class="text-caption text-label-secondary" role="status">正在读取关系…</p>
    <template v-else>
      <section class="mt-4" aria-label="待确认建议">
        <div class="mb-2 flex items-center gap-2">
          <h3 class="text-caption font-semibold text-label">待确认建议</h3>
          <span
            v-if="suggested.length"
            class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
            >{{ suggested.length }} 条</span
          >
        </div>
        <p
          v-if="!suggested.length"
          class="rounded-md border border-dashed border-hairline px-3 py-3 text-caption leading-5 text-label-secondary"
        >
          暂无待确认建议。点上方「查找相关知识」，AI 找到的建议会出现在这里。
        </p>
        <ul v-else class="space-y-2">
          <li
            v-for="relation in suggested"
            :key="relation.id"
            class="rounded-md border border-hairline p-3"
            :data-suggestion-id="relation.id"
          >
            <div class="flex items-center justify-between gap-2">
              <span
                class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
                >{{ relationTypeLabel(relation.relation_type) }}</span
              >
              <span v-if="relation.confidence > 0" class="text-caption text-label-tertiary"
                >置信度 {{ confidencePercent(relation) }}</span
              >
            </div>
            <RouterLink
              :to="`/home/knowledge/${relation.counterpart_id}`"
              class="mt-1.5 block break-words text-subhead font-medium leading-6 text-label hover:text-primary-pressed"
              >{{ relation.counterpart_title }}</RouterLink
            >

            <template v-if="evidenceOf(relation)">
              <button
                type="button"
                class="knowledge-text-action min-h-8 text-caption"
                @click="toggleEvidence(relation.id)"
              >
                <Icon
                  :icon="
                    expandedId === relation.id
                      ? 'material-symbols:expand-less'
                      : 'material-symbols:expand-more'
                  "
                  class="text-[14px]"
                />
                {{ expandedId === relation.id ? '收起理由' : '查看理由' }}
              </button>
              <div
                v-if="expandedId === relation.id"
                class="space-y-2 rounded-sm bg-surface-secondary p-2.5"
              >
                <p v-if="evidenceOf(relation)!.reason" class="break-words text-caption leading-5">
                  {{ evidenceOf(relation)!.reason }}
                </p>
                <blockquote
                  v-if="evidenceOf(relation)!.source_quote"
                  class="line-clamp-3 break-words border-l-2 border-hairline pl-2 text-caption leading-5 text-label-secondary"
                >
                  本文：{{ evidenceOf(relation)!.source_quote }}
                </blockquote>
                <blockquote
                  v-if="evidenceOf(relation)!.target_quote"
                  class="line-clamp-3 break-words border-l-2 border-hairline pl-2 text-caption leading-5 text-label-secondary"
                >
                  对端：{{ evidenceOf(relation)!.target_quote }}
                </blockquote>
              </div>
            </template>

            <div class="mt-2 flex items-center gap-2">
              <button
                type="button"
                class="knowledge-button-primary min-h-8 flex-1 px-3 text-caption"
                :disabled="busy === `confirm:${relation.id}`"
                @click="confirm(relation)"
              >
                {{ busy === `confirm:${relation.id}` ? '确认中…' : '确认' }}
              </button>
              <button
                type="button"
                class="knowledge-button-secondary min-h-8 shrink-0 px-3 text-caption"
                :disabled="busy === `reject:${relation.id}`"
                @click="reject(relation)"
              >
                {{ busy === `reject:${relation.id}` ? '忽略中…' : '忽略' }}
              </button>
            </div>
          </li>
        </ul>
      </section>

      <section class="mt-4 border-t border-hairline pt-3" aria-label="已确认关系">
        <div class="mb-2 flex items-center gap-2">
          <h3 class="text-caption font-semibold text-label">已确认</h3>
          <span
            v-if="confirmed.length"
            class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
            >{{ confirmed.length }} 条</span
          >
        </div>
        <p
          v-if="!confirmed.length && !suggested.length"
          class="text-caption leading-6 text-label-secondary"
        >
          还没有建立关系。手动添加一条，把这条知识和相关内容连起来。
        </p>
        <p v-else-if="!confirmed.length" class="text-caption leading-6 text-label-secondary">
          还没有已确认的关系。确认上面的建议，或手动添加一条。
        </p>
        <ul v-else class="space-y-2">
          <li
            v-for="relation in confirmed"
            :key="relation.id"
            class="rounded-md border border-hairline p-2.5"
          >
            <div class="flex items-start justify-between gap-2">
              <RouterLink
                :to="`/home/knowledge/${relation.counterpart_id}`"
                class="min-w-0 break-all text-subhead font-medium leading-6 text-label hover:text-primary-pressed"
                >{{ relation.counterpart_title }}</RouterLink
              >
              <button
                v-if="pendingDelete !== relation.id"
                type="button"
                class="knowledge-icon-button -mr-1 -mt-1 size-8 shrink-0"
                :aria-label="`删除与${relation.counterpart_title}的关系`"
                @click="pendingDelete = relation.id"
              >
                <Icon icon="material-symbols:delete-outline" class="text-[16px]" />
              </button>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <FilterMenu
                :model-value="relation.relation_type"
                :options="relationTypeOptions"
                :label="`修改与${relation.counterpart_title}的关系类型`"
                :disabled="busy === `type:${relation.id}`"
                compact
                :icon="false"
                @update:model-value="changeType(relation, $event)"
              />
              <span class="text-caption text-label-tertiary">{{
                relation.source === 'manual' ? '手动添加' : 'AI 建议'
              }}</span>
            </div>
            <div
              v-if="pendingDelete === relation.id"
              class="mt-2 flex items-center gap-2 border-t border-hairline pt-2"
            >
              <button
                type="button"
                class="knowledge-text-action min-h-8 text-caption text-danger"
                :disabled="busy === `delete:${relation.id}`"
                @click="remove(relation)"
              >
                确认删除
              </button>
              <button
                type="button"
                class="knowledge-text-action min-h-8 text-caption"
                @click="pendingDelete = null"
              >
                取消
              </button>
            </div>
          </li>
        </ul>
      </section>

      <section class="mt-4 border-t border-hairline pt-3" aria-label="手动添加关系">
        <h3 class="text-caption font-semibold text-label">手动添加</h3>
        <div class="mt-2 flex gap-2">
          <div class="relative min-w-0 flex-1">
            <Icon
              class="pointer-events-none absolute left-2.5 top-3 text-label-tertiary"
              icon="material-symbols:search"
            />
            <input
              v-model="query"
              class="knowledge-input w-full pl-8 text-caption"
              placeholder="搜索知识标题"
              aria-label="搜索知识标题"
              @keydown.enter.prevent="search"
            />
          </div>
          <button
            type="button"
            class="knowledge-button-secondary shrink-0 px-3 text-caption"
            :disabled="searching"
            @click="search"
          >
            {{ searching ? '搜索中…' : '搜索' }}
          </button>
        </div>
        <div
          v-if="results.length"
          class="mt-2 overflow-hidden rounded-md border border-hairline bg-surface shadow-card"
        >
          <button
            v-for="candidate in results"
            :key="candidate.id"
            type="button"
            class="block w-full truncate px-2.5 py-2 text-left text-caption transition-colors hover:bg-surface-secondary"
            @click="choose(candidate)"
          >
            {{ candidate.title }}
          </button>
        </div>
        <p v-if="searchHint" class="mt-2 text-caption text-label-secondary">{{ searchHint }}</p>
        <div
          v-if="target"
          class="mt-2 flex items-center gap-2 rounded-md bg-surface-secondary px-2.5 py-2"
        >
          <span class="min-w-0 flex-1 truncate text-caption text-label"
            >已选：{{ target.title }}</span
          >
          <button
            type="button"
            class="text-label-secondary transition-colors hover:text-label"
            aria-label="清除已选知识"
            @click="resetForm"
          >
            <Icon icon="material-symbols:close" class="text-[14px]" />
          </button>
        </div>
        <FilterMenu
          :model-value="relationType"
          field
          compact
          :options="relationTypeOptions"
          label="关系类型"
          class="mt-2"
          @update:model-value="setRelationType"
        />
        <button
          type="button"
          class="knowledge-button-primary mt-2 w-full"
          :disabled="!target || busy === 'create'"
          @click="add"
        >
          {{ busy === 'create' ? '添加中…' : '添加关系' }}
        </button>
      </section>
    </template>
    <p v-if="error" class="mt-3 text-caption leading-5 text-danger" role="alert">{{ error }}</p>
  </section>
</template>
