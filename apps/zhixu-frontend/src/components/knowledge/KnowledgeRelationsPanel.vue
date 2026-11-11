<script setup lang="ts">
import { onScopeDispose, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  createKnowledgeRelation,
  deleteKnowledgeRelation,
  listKnowledge,
  listKnowledgeRelations,
  type KnowledgeItem,
  type KnowledgeRelation,
  type RelationType,
} from '../../services/apis/knowledge'
import { knowledgeError } from '../../utils/knowledge'
import { RELATION_TYPE_OPTIONS, relationTypeLabel } from '../../utils/relation'

const props = defineProps<{ item: KnowledgeItem }>()

const relations = ref<KnowledgeRelation[]>([])
const loading = ref(false)
const error = ref('')
const busy = ref('')
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

async function load() {
  controller?.abort()
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const data = await listKnowledgeRelations(props.item.id, current.signal)
    if (current.signal.aborted) return
    relations.value = data.confirmed
  } catch (issue) {
    if (!current.signal.aborted) error.value = knowledgeError(issue)
  } finally {
    if (controller === current) {
      controller = null
      loading.value = false
    }
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
    const known = new Set(relations.value.map(relation => relation.counterpart_id))
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
    relations.value = [created, ...relations.value]
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
    relations.value = relations.value.filter(entry => entry.id !== relation.id)
    pendingDelete.value = null
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

watch(() => props.item.id, load, { immediate: true })
onScopeDispose(() => {
  controller?.abort()
  searchController?.abort()
})
</script>

<template>
  <section class="rounded-lg border border-hairline bg-surface p-5">
    <div class="mb-3 flex items-center justify-between gap-2">
      <h2 class="text-body font-semibold">相关知识</h2>
      <span
        v-if="relations.length"
        class="rounded-full bg-surface-secondary px-2.5 py-0.5 text-caption text-label-secondary"
        >{{ relations.length }} 条</span
      >
    </div>
    <p v-if="loading" class="text-caption text-label-secondary" role="status">正在读取关系…</p>
    <template v-else>
      <p v-if="!relations.length" class="text-caption leading-6 text-label-secondary">
        还没有建立关系。手动添加一条，把这条知识和相关内容连起来。
      </p>
      <ul v-else class="space-y-2">
        <li
          v-for="relation in relations"
          :key="relation.id"
          class="flex items-start justify-between gap-2"
        >
          <div class="min-w-0">
            <span
              class="rounded-full bg-surface-secondary px-2 py-0.5 text-caption text-label-secondary"
              >{{ relationTypeLabel(relation.relation_type) }}</span
            >
            <RouterLink
              :to="`/home/knowledge/${relation.counterpart_id}`"
              class="ml-2 break-all text-subhead text-primary-pressed hover:underline"
              >{{ relation.counterpart_title }}</RouterLink
            >
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
