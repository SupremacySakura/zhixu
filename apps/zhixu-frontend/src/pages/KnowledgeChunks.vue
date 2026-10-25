<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getKnowledge, type KnowledgeItem } from '../services/apis/knowledge'
import { useKnowledgeChunks } from '../composables/useKnowledgeChunks'
import { isKnowledgeVectorActive, knowledgeError } from '../utils/knowledge'
import KnowledgeVectorPanel from '../components/knowledge/KnowledgeVectorPanel.vue'

const route = useRoute()
const id = computed(() => Number(route.params.id))
const { items, loading, error, load } = useKnowledgeChunks(id, false)
const item = ref<KnowledgeItem | null>(null)
const detailError = ref('')
const failedOnly = ref(false)
const visible = computed(() =>
  items.value.filter(chunk => !failedOnly.value || chunk.state === 'failed'),
)
const sections = computed(() => {
  const groups: { key: number; title: string; level: number; first: number; last: number }[] = []
  for (const chunk of visible.value) {
    const previous = groups.at(-1)
    if (previous?.title === chunk.section) previous.last = chunk.seq
    else
      groups.push({
        key: chunk.seq,
        title: chunk.section,
        level: chunk.section_level,
        first: chunk.seq,
        last: chunk.seq,
      })
  }
  return groups
})
function leading(content: string, overlap: number) {
  return Array.from(content).slice(0, overlap).join('')
}
function remainder(content: string, overlap: number) {
  return Array.from(content).slice(overlap).join('')
}
function locate(seq: number) {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  document
    .getElementById(`chunk-${seq}`)
    ?.scrollIntoView({ behavior: reduced ? 'auto' : 'smooth', block: 'start' })
}
let controller: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined
async function refresh() {
  controller?.abort()
  clearTimeout(timer)
  const current = new AbortController()
  controller = current
  detailError.value = ''
  try {
    const data = await getKnowledge(id.value, current.signal)
    if (current.signal.aborted) return
    item.value = data
    await load()
    if (current.signal.aborted) return
    if (isKnowledgeVectorActive(data.vector_state)) timer = setTimeout(refresh, 3000)
  } catch (e) {
    if (!current.signal.aborted) detailError.value = knowledgeError(e)
  }
}
watch(
  id,
  () => {
    item.value = null
    failedOnly.value = false
    void refresh()
  },
  { immediate: true },
)
onScopeDispose(() => {
  controller?.abort()
  clearTimeout(timer)
})
</script>

<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <nav aria-label="面包屑" class="mb-5 flex flex-wrap gap-3 text-subhead text-label-secondary">
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink>
        <span>/</span>
        <RouterLink :to="`/home/knowledge/${id}`" class="text-primary-pressed">知识详情</RouterLink>
        <span>/ 分块结构</span>
      </nav>
      <header class="mb-7 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="break-all text-title-1">{{ item?.title || '分块结构' }}</h1>
          <p class="mt-2 text-subhead leading-6 text-label-secondary">
            按写入顺序查看文档切分。每块最多 512 字符，同章节相邻块最多重叠 64 字符。
          </p>
        </div>
        <label class="flex cursor-pointer items-center gap-2 text-subhead">
          <input v-model="failedOnly" type="checkbox" class="accent-primary" />
          只看失败
        </label>
      </header>
      <p
        v-if="error || detailError"
        role="alert"
        class="mb-5 rounded-lg bg-warning-tint p-4 text-warning-strong"
      >
        {{ error || detailError }}
        <button class="knowledge-text-action" type="button" @click="refresh">重新加载</button>
      </p>
      <div class="grid items-start gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
        <aside class="space-y-5 lg:sticky lg:top-4">
          <section class="rounded-lg border border-hairline bg-surface p-5">
            <h2 class="text-body font-semibold">章节大纲</h2>
            <p class="mt-2 text-caption text-label-secondary">
              字符范围从 0 开始，结束位置不包含在块内。
            </p>
            <nav aria-label="章节大纲" class="mt-4 space-y-1">
              <button
                v-for="section in sections"
                :key="section.key"
                type="button"
                class="w-full rounded-sm px-2 py-2 text-left text-subhead hover:bg-primary-tint focus-visible:outline-primary"
                @click="locate(section.first)"
              >
                <span class="block break-words">{{ section.title }}</span>
                <span class="text-caption text-label-secondary"
                  >#{{ section.first }} - #{{ section.last }}</span
                >
              </button>
            </nav>
          </section>
          <KnowledgeVectorPanel v-if="item" :item="item" />
        </aside>
        <section aria-label="文档分块" class="min-w-0 space-y-4">
          <p
            v-if="loading && !items.length"
            role="status"
            class="py-12 text-center text-label-secondary"
          >
            正在读取分块…
          </p>
          <p
            v-else-if="!visible.length && !error"
            class="rounded-lg border border-hairline bg-surface p-8 text-subhead text-label-secondary"
          >
            {{ failedOnly ? '没有失败分块。' : '尚未生成分块，任务开始后会自动展示。' }}
          </p>
          <article
            v-for="chunk in visible"
            :id="`chunk-${chunk.seq}`"
            :key="chunk.id"
            class="scroll-mt-4 rounded-lg border bg-surface p-5 sm:p-6"
            :class="chunk.state === 'failed' ? 'border-danger' : 'border-hairline'"
          >
            <header class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="break-words text-headline">
                  #{{ String(chunk.seq).padStart(2, '0') }} · {{ chunk.section }}
                </h2>
                <p class="mt-1 text-caption text-label-secondary">
                  {{ chunk.chars }} 字符 · [{{ chunk.start_char }}, {{ chunk.end_char }})
                </p>
              </div>
              <span
                class="rounded-full px-3 py-1 text-caption"
                :class="
                  chunk.state === 'failed'
                    ? 'bg-danger-tint text-danger'
                    : chunk.state === 'ready'
                      ? 'bg-primary-tint text-primary-pressed'
                      : 'bg-surface-secondary text-label-secondary'
                "
              >
                {{
                  chunk.state === 'ready'
                    ? '已入向量库'
                    : chunk.state === 'failed'
                      ? '向量化失败'
                      : '待处理'
                }}
              </span>
            </header>
            <p v-if="chunk.overlap_chars" class="mb-3 text-caption text-primary-pressed">
              与上一块重叠 {{ chunk.overlap_chars }} 字符，以下高亮为重复上下文。
            </p>
            <pre
              class="whitespace-pre-wrap break-words font-sans text-subhead leading-7"
            ><mark v-if="chunk.overlap_chars" class="rounded-sm bg-primary-tint text-primary-pressed">{{ leading(chunk.content, chunk.overlap_chars) }}</mark>{{ remainder(chunk.content, chunk.overlap_chars) }}</pre>
            <p
              v-if="chunk.error_message"
              role="status"
              class="mt-4 rounded-sm bg-danger-tint p-3 text-subhead text-danger"
            >
              {{ chunk.error_message }}
            </p>
          </article>
        </section>
      </div>
    </div>
  </main>
</template>
