<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import {
  acceptKnowledgeOrganize,
  getKnowledgeOrganize,
  regenerateKnowledgeOrganize,
  type KnowledgeItem,
  type KnowledgeOrganizeDraft,
  type KnowledgeOrganizeTask,
} from '../../services/apis/knowledge'
import { knowledgeDate, knowledgeError } from '../../utils/knowledge'

const props = defineProps<{ item: KnowledgeItem }>()
const emit = defineEmits<{ accepted: [item: KnowledgeItem] }>()

const POLL_INTERVAL = 3_000
const SUMMARY_MAX = 200
const KEYWORD_MAX = 10
// 与后端 OrganizeKeywordMaxRunes 保持一致：英文技术术语（如 React Server Components）超过 20 字符。
const KEYWORD_MAX_RUNES = 40

const task = ref<KnowledgeOrganizeTask | null>(null)
const draft = ref<KnowledgeOrganizeDraft | null>(null)
const loading = ref(false)
const busy = ref('')
const error = ref('')

// 可编辑的预览字段：草稿到达时填入，之后以表单为准，采纳时整份提交。
const summary = ref('')
const keywords = ref<string[]>([])
const keywordInput = ref('')
const selectedTags = ref<string[]>([])

let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | null = null

const visible = computed(() => props.item.status === 'ready')
const running = computed(() => task.value?.status === 'pending' || task.value?.status === 'running')
const statusLabel = computed(() => {
  switch (task.value?.status) {
    case 'pending':
      return '排队中'
    case 'running':
      return '整理中'
    case 'succeeded':
      return draft.value?.accepted ? '已采纳' : '待确认'
    case 'failed':
      return '整理失败'
    default:
      return '未整理'
  }
})
const summaryCount = computed(() => [...summary.value].length)
const keywordsFull = computed(() => keywords.value.length >= KEYWORD_MAX)
const canAccept = computed(
  () => Boolean(draft.value) && !running.value && summary.value.trim().length > 0,
)

function stopPolling() {
  if (timer) {
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

function applyDraft(source: KnowledgeOrganizeDraft) {
  summary.value = source.summary
  keywords.value = [...source.keywords]
  keywordInput.value = ''
  selectedTags.value = source.suggested_tags.map(tag => tag.name)
}

function resetForm() {
  summary.value = ''
  keywords.value = []
  keywordInput.value = ''
  selectedTags.value = []
}

async function load(background = false) {
  controller?.abort()
  const current = new AbortController()
  controller = current
  if (!background) loading.value = true
  try {
    const state = await getKnowledgeOrganize(props.item.id, current.signal)
    if (current.signal.aborted) return
    task.value = state.task
    draft.value = state.draft
    error.value = ''
    // 后台轮询只在任务未完成时发生，此时不会有草稿，因此不会覆盖用户正在编辑的内容。
    if (state.draft) applyDraft(state.draft)
    else resetForm()
  } catch (issue) {
    if (!current.signal.aborted) error.value = knowledgeError(issue)
  } finally {
    if (controller === current) {
      controller = null
      loading.value = false
      schedulePoll()
    }
  }
}

function addKeyword() {
  const clean = keywordInput.value.trim()
  keywordInput.value = ''
  if (!clean || [...clean].length > KEYWORD_MAX_RUNES) return
  if (keywords.value.length >= KEYWORD_MAX) return
  if (keywords.value.some(item => item.toLowerCase() === clean.toLowerCase())) return
  keywords.value = [...keywords.value, clean]
}

function removeKeyword(word: string) {
  keywords.value = keywords.value.filter(item => item !== word)
}

function toggleTag(name: string) {
  selectedTags.value = selectedTags.value.includes(name)
    ? selectedTags.value.filter(item => item !== name)
    : [...selectedTags.value, name]
}

async function accept() {
  if (!draft.value || busy.value) return
  busy.value = 'accept'
  error.value = ''
  try {
    const suggested = draft.value.suggested_tags.filter(tag =>
      selectedTags.value.includes(tag.name),
    )
    const result = await acceptKnowledgeOrganize(props.item.id, {
      summary: summary.value.trim(),
      keywords: keywords.value,
      tag_ids: suggested.filter(tag => tag.tag_id > 0).map(tag => tag.tag_id),
      tag_names: suggested.filter(tag => tag.tag_id === 0).map(tag => tag.name),
    })
    if (result.draft) {
      draft.value = result.draft
      applyDraft(result.draft)
    }
    emit('accepted', result.item)
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

async function regenerate() {
  if (busy.value) return
  busy.value = 'regenerate'
  error.value = ''
  try {
    const state = await regenerateKnowledgeOrganize(props.item.id)
    task.value = state.task
    draft.value = state.draft
    if (state.draft) applyDraft(state.draft)
    else resetForm()
    schedulePoll()
  } catch (issue) {
    error.value = knowledgeError(issue)
  } finally {
    busy.value = ''
  }
}

watch(
  () => props.item.id,
  () => {
    task.value = null
    draft.value = null
    error.value = ''
    resetForm()
    void load()
  },
  { immediate: true },
)
watch(
  () => props.item.status,
  (status, previous) => {
    // 后台向量化完成、正文重新就绪后自动补一次，不必等用户刷新页面。
    if (status === 'ready' && previous !== 'ready') void load(true)
  },
)

onScopeDispose(() => {
  stopPolling()
  controller?.abort()
  controller = null
})
</script>

<template>
  <section v-if="visible" class="border border-hairline bg-surface p-5">
    <div class="mb-3 flex items-center justify-between gap-2">
      <h2 class="text-body font-semibold">AI 整理</h2>
      <span class="bg-surface-secondary px-2.5 py-0.5 text-note text-label-secondary">
        {{ statusLabel }}
      </span>
    </div>

    <p v-if="loading" class="text-note text-label-secondary" role="status">正在读取整理结果…</p>

    <template v-else>
      <p
        v-if="running"
        class="bg-surface-secondary px-3 py-2.5 text-note leading-5 text-label-secondary"
        role="status"
      >
        {{
          task?.status === 'pending'
            ? '整理任务已排队，通常十几秒内完成。'
            : '模型正在生成摘要与标签，请稍候。'
        }}
      </p>
      <p v-else-if="!task" class="text-note leading-6 text-label-secondary">
        还没有整理结果。生成后先在这里预览与修改，确认后再采纳；未采纳前不会改动正文与正式字段。
      </p>
      <p
        v-else-if="task.status === 'failed'"
        class="bg-danger-tint px-3 py-2.5 text-note leading-5 text-danger"
        role="alert"
      >
        {{ task.error_message || 'AI 整理失败，请重试。' }}
      </p>

      <template v-if="draft && !running">
        <p
          v-if="draft.stale"
          class="mt-3 bg-warning-tint px-3 py-2.5 text-note leading-5 text-warning-strong"
          role="status"
        >
          正文在草稿生成后已更新，建议重新生成后再采纳。
        </p>

        <div class="mt-4">
          <div class="flex items-baseline justify-between gap-2">
            <label for="organize-summary" class="text-note text-label-secondary">摘要</label>
            <span class="text-note text-label-secondary">{{ summaryCount }}/{{ SUMMARY_MAX }}</span>
          </div>
          <textarea
            id="organize-summary"
            v-model="summary"
            rows="3"
            :maxlength="SUMMARY_MAX"
            class="atom-field min-h-9 mt-1.5 w-full resize-none py-2.5"
            placeholder="采纳后会显示在知识列表"
          />
        </div>

        <div class="mt-4">
          <p class="text-note text-label-secondary">关键词（最多 {{ KEYWORD_MAX }} 个）</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <span
              v-for="word in keywords"
              :key="word"
              class="inline-flex items-center gap-1.5 bg-surface-secondary px-3 py-1 text-note"
            >
              {{ word }}
              <button
                type="button"
                class="leading-none text-label-secondary transition-colors hover:text-label"
                :aria-label="'移除关键词 ' + word"
                @click="removeKeyword(word)"
              >
                ×
              </button>
            </span>
            <span v-if="!keywords.length" class="text-note text-label-secondary">还没有关键词</span>
          </div>
          <input
            v-model="keywordInput"
            class="atom-field min-h-9 mt-2 w-full"
            :maxlength="KEYWORD_MAX_RUNES"
            :disabled="keywordsFull"
            placeholder="输入关键词后回车添加"
            @keydown.enter.prevent="addKeyword"
          />
        </div>

        <div v-if="draft.suggested_tags.length" class="mt-4">
          <p class="text-note text-label-secondary">
            推荐标签（点击切换；采纳时追加到已有标签，不会删除已有关联）
          </p>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-for="tag in draft.suggested_tags"
              :key="tag.name"
              type="button"
              class="border px-3 py-1 text-note transition-colors"
              :class="
                selectedTags.includes(tag.name)
                  ? 'border-primary bg-primary-tint text-primary-pressed'
                  : 'border-hairline text-label-secondary hover:border-primary hover:text-primary'
              "
              :aria-pressed="selectedTags.includes(tag.name)"
              @click="toggleTag(tag.name)"
            >
              {{ tag.name }}
              <span class="ml-1 opacity-60">{{ tag.existing ? '已有' : '新建' }}</span>
            </button>
          </div>
        </div>

        <p v-if="draft.accepted" class="mt-3 text-note leading-5 text-label-secondary">
          已采纳{{
            draft.accepted_at ? '于 ' + knowledgeDate(draft.accepted_at) : ''
          }}，可继续修改后再次采纳。
        </p>
      </template>

      <p v-if="error" class="mt-3 text-note leading-5 text-danger" role="alert">{{ error }}</p>

      <div class="mt-4 space-y-2">
        <button
          v-if="draft && !running"
          type="button"
          class="atom-btn atom-tone-primary atom-btn-lg w-full"
          :disabled="!canAccept || busy === 'accept'"
          @click="accept"
        >
          {{ busy === 'accept' ? '正在采纳…' : draft.accepted ? '更新采纳结果' : '采纳整理结果' }}
        </button>
        <button
          type="button"
          class="atom-btn atom-tone-tint atom-btn-wide w-full"
          :disabled="running || Boolean(busy)"
          @click="regenerate"
        >
          {{
            busy === 'regenerate'
              ? '正在重新排队…'
              : running
                ? '正在整理…'
                : draft || task
                  ? '重新生成'
                  : '生成整理结果'
          }}
        </button>
      </div>
    </template>
  </section>
</template>
