<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
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

async function changeType(relation: KnowledgeRelation, event: Event) {
  const next = (event.target as HTMLSelectElement).value as RelationType
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
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <h2 class="text-body font-semibold">相关知识</h2>
      <div class="flex items-center gap-3">
        <span
          v-if="total"
          class="rounded-full bg-surface-secondary px-2.5 py-0.5 text-caption text-label-secondary"
          >{{ total }} 条</span
        >
        <RouterLink
          :to="`/home/knowledge/graph?center=${item.id}`"
          class="knowledge-text-action text-caption"
          >在图谱中查看</RouterLink
        >
      </div>
    </div>

    <div class="mb-3 flex flex-wrap items-center gap-2">
      <button
        type="button"
        class="knowledge-button-secondary min-h-9 px-3 text-caption"
        :disabled="running || busy === 'regenerate'"
        @click="regenerate"
      >
        {{ running ? '查找相关知识…' : '查找相关知识' }}
      </button>
      <span v-if="running" class="text-caption text-label-secondary" role="status"
        >AI 正在查找相关知识，请稍候…</span
      >
    </div>

    <div
      v-if="task?.status === 'failed'"
      class="mb-3 rounded-md border border-hairline bg-surface-secondary p-3"
      role="alert"
    >
      <p class="break-words text-caption leading-5 text-danger">
        查找失败：{{ task.error_message || '请稍后重试' }}
      </p>
      <button
        type="button"
        class="knowledge-text-action mt-1 text-caption"
        :disabled="busy === 'regenerate'"
        @click="regenerate"
      >
        重试
      </button>
    </div>

    <p v-if="loading" class="text-caption text-label-secondary" role="status">正在读取关系…</p>
    <template v-else>
      <section class="mb-4" aria-label="待确认建议">
        <div class="mb-2 flex items-center gap-2">
          <h3 class="text-caption font-semibold">待确认建议</h3>
          <span
            v-if="suggested.length"
            class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
            >{{ suggested.length }} 条</span
          >
        </div>
        <p v-if="!suggested.length" class="text-caption leading-6 text-label-secondary">
          暂无待确认建议。点「查找相关知识」，让 AI 找找可能相关的知识。
        </p>
        <ul v-else class="space-y-3">
          <li
            v-for="relation in suggested"
            :key="relation.id"
            class="rounded-md border border-hairline p-3"
            :data-suggestion-id="relation.id"
          >
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
                >{{ relationTypeLabel(relation.relation_type) }}</span
              >
              <span v-if="relation.confidence > 0" class="text-caption text-label-secondary"
                >置信度 {{ confidencePercent(relation) }}</span
              >
              <RouterLink
                :to="`/home/knowledge/${relation.counterpart_id}`"
                class="min-w-0 break-words text-subhead text-primary-pressed hover:underline"
                >{{ relation.counterpart_title }}</RouterLink
              >
            </div>

            <div v-if="evidenceOf(relation)" class="mt-2">
              <button
                type="button"
                class="knowledge-text-action text-caption"
                @click="toggleEvidence(relation.id)"
              >
                {{ expandedId === relation.id ? '收起理由' : '查看理由' }}
              </button>
              <div
                v-if="expandedId === relation.id"
                class="mt-2 space-y-2 rounded-md bg-surface-secondary p-2"
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
            </div>

            <div class="mt-3 flex gap-2">
              <button
                type="button"
                class="knowledge-button-primary min-h-8 px-3 text-caption"
                :disabled="busy === `confirm:${relation.id}`"
                @click="confirm(relation)"
              >
                {{ busy === `confirm:${relation.id}` ? '确认中…' : '确认' }}
              </button>
              <button
                type="button"
                class="knowledge-button-secondary min-h-8 px-3 text-caption"
                :disabled="busy === `reject:${relation.id}`"
                @click="reject(relation)"
              >
                {{ busy === `reject:${relation.id}` ? '忽略中…' : '忽略' }}
              </button>
            </div>
          </li>
        </ul>
      </section>

      <section class="border-t border-hairline pt-3" aria-label="已确认关系">
        <div class="mb-2 flex items-center gap-2">
          <h3 class="text-caption font-semibold">已确认</h3>
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
            class="flex flex-wrap items-start justify-between gap-2"
          >
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <RouterLink
                :to="`/home/knowledge/${relation.counterpart_id}`"
                class="min-w-0 break-all text-subhead text-primary-pressed hover:underline"
                >{{ relation.counterpart_title }}</RouterLink
              >
              <select
                class="knowledge-input w-auto py-1 text-caption"
                :value="relation.relation_type"
                :aria-label="`修改与${relation.counterpart_title}的关系类型`"
                :disabled="busy === `type:${relation.id}`"
                @change="changeType(relation, $event)"
              >
                <option
                  v-for="option in RELATION_TYPE_OPTIONS"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <template v-if="pendingDelete === relation.id">
                <button
                  type="button"
                  class="knowledge-text-action text-caption text-danger"
                  :disabled="busy === `delete:${relation.id}`"
                  @click="remove(relation)"
                >
                  确认删除
                </button>
                <button
                  type="button"
                  class="knowledge-text-action text-caption"
                  @click="pendingDelete = null"
                >
                  取消
                </button>
              </template>
              <button
                v-else
                type="button"
                class="knowledge-text-action text-caption"
                :aria-label="`删除与${relation.counterpart_title}的关系`"
                @click="pendingDelete = relation.id"
              >
                删除
              </button>
            </div>
          </li>
        </ul>
      </section>

      <div class="mt-4 border-t border-hairline pt-4">
        <p class="mb-2 text-caption text-label-secondary">手动添加</p>
        <div class="flex gap-2">
          <input
            v-model="query"
            class="knowledge-input min-w-0 flex-1"
            placeholder="搜索知识标题"
            @keydown.enter.prevent="search"
          />
          <button
            type="button"
            class="knowledge-button-secondary min-h-9 shrink-0 px-3 text-caption"
            :disabled="searching"
            @click="search"
          >
            {{ searching ? '搜索中…' : '搜索' }}
          </button>
        </div>
        <div v-if="results.length" class="mt-2 space-y-1">
          <button
            v-for="candidate in results"
            :key="candidate.id"
            type="button"
            class="block w-full truncate rounded-md px-2 py-1 text-left text-caption transition-colors hover:bg-surface-secondary"
            @click="choose(candidate)"
          >
            {{ candidate.title }}
          </button>
        </div>
        <p v-if="searchHint" class="mt-2 text-caption text-label-secondary">{{ searchHint }}</p>
        <div v-if="target" class="mt-3 flex items-center gap-2">
          <span class="min-w-0 truncate text-caption text-label-secondary"
            >已选：{{ target.title }}</span
          >
          <button type="button" class="knowledge-text-action text-caption" @click="resetForm">
            清除
          </button>
        </div>
        <select v-model="relationType" class="knowledge-input mt-3 w-full" aria-label="关系类型">
          <option v-for="option in RELATION_TYPE_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <button
          type="button"
          class="knowledge-button-primary mt-3 min-h-9 px-4 text-caption"
          :disabled="!target || busy === 'create'"
          @click="add"
        >
          {{ busy === 'create' ? '添加中…' : '添加关系' }}
        </button>
      </div>
    </template>
    <p v-if="error" class="mt-3 text-caption leading-5 text-danger" role="alert">{{ error }}</p>
  </section>
</template>
