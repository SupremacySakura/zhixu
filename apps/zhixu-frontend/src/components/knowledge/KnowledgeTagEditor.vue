<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { getKnowledgeTags, listTags, replaceKnowledgeTags, type Tag } from '../../services/apis/tag'
import { knowledgeError } from '../../utils/knowledge'
const props = defineProps<{ knowledgeId: number }>()
const emit = defineEmits<{ saved: [tags: Tag[]] }>()
const tags = ref<Tag[]>([])
const allTags = ref<Tag[]>([])
const draft = ref<Tag[]>([])
const input = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const signature = (list: Tag[]) =>
  list
    .map(tag => tag.name.toLowerCase())
    .sort()
    .join('\u0000')
const dirty = computed(() => signature(draft.value) !== signature(tags.value))
const suggestions = computed(() => {
  const query = input.value.trim().toLowerCase()
  const chosen = new Set(draft.value.map(tag => tag.name.toLowerCase()))
  return allTags.value
    .filter(
      tag =>
        !chosen.has(tag.name.toLowerCase()) && (!query || tag.name.toLowerCase().includes(query)),
    )
    .slice(0, 6)
})
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [current, all] = await Promise.all([getKnowledgeTags(props.knowledgeId), listTags()])
    tags.value = current.items
    draft.value = current.items.map(tag => ({ ...tag }))
    allTags.value = all.items
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    loading.value = false
  }
}
function addTag(name: string) {
  const clean = name.trim()
  if (!clean || clean.length > 30) return
  const lower = clean.toLowerCase()
  input.value = ''
  if (draft.value.some(tag => tag.name.toLowerCase() === lower)) return
  const existing = allTags.value.find(tag => tag.name.toLowerCase() === lower)
  draft.value.push(existing ? { ...existing } : { id: 0, name: clean, usage_count: 0 })
}
function removeTag(name: string) {
  draft.value = draft.value.filter(tag => tag.name !== name)
}
function reset() {
  draft.value = tags.value.map(tag => ({ ...tag }))
  input.value = ''
  error.value = ''
}
async function save() {
  saving.value = true
  error.value = ''
  try {
    const result = await replaceKnowledgeTags(
      props.knowledgeId,
      draft.value.map(tag => tag.name),
    )
    tags.value = result.items
    draft.value = result.items.map(tag => ({ ...tag }))
    emit('saved', result.items)
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    saving.value = false
  }
}
watch(() => props.knowledgeId, load)
onMounted(load)
</script>
<template>
  <aside class="rounded-lg border border-hairline bg-surface p-5">
    <div class="mb-3 flex items-center justify-between gap-2">
      <h2 class="text-body font-semibold">标签</h2>
      <button
        v-if="dirty && !saving"
        type="button"
        class="knowledge-text-action text-caption"
        @click="reset"
      >
        重置
      </button>
    </div>
    <p v-if="loading" class="text-caption text-label-secondary" role="status">正在读取标签…</p>
    <template v-else>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="tag in draft"
          :key="tag.name"
          class="inline-flex items-center gap-1.5 rounded-full bg-surface-secondary px-3 py-1 text-caption"
        >
          {{ tag.name }}
          <button
            type="button"
            class="leading-none text-label-secondary transition-colors hover:text-label"
            :aria-label="`移除标签 ${tag.name}`"
            @click="removeTag(tag.name)"
          >
            ×
          </button>
        </span>
        <span v-if="!draft.length" class="text-caption text-label-secondary">还没有标签</span>
      </div>
      <div class="mt-3">
        <input
          v-model="input"
          class="knowledge-input w-full"
          placeholder="输入标签后回车添加"
          maxlength="30"
          @keydown.enter.prevent="addTag(input)"
        />
        <div v-if="suggestions.length" class="mt-2 flex flex-wrap gap-2">
          <button
            v-for="suggestion in suggestions"
            :key="suggestion.name"
            type="button"
            class="rounded-full border border-hairline px-3 py-1 text-caption text-label-secondary transition-colors hover:border-primary hover:text-primary"
            @click="addTag(suggestion.name)"
          >
            {{ suggestion.name
            }}<span v-if="suggestion.usage_count" class="ml-1 opacity-60">{{
              suggestion.usage_count
            }}</span>
          </button>
        </div>
      </div>
      <p v-if="error" class="mt-3 text-caption text-warning-strong" role="alert">{{ error }}</p>
      <button
        v-if="dirty"
        type="button"
        class="knowledge-button-primary mt-4 min-h-9 px-4 text-caption"
        :disabled="saving"
        @click="save"
      >
        {{ saving ? '保存中…' : '保存标签' }}
      </button>
    </template>
  </aside>
</template>
