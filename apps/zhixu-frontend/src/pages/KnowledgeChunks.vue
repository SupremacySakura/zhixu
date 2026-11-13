<script setup lang="ts">
import { computed, nextTick, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppIcon from '../components/ui/AppIcon.vue'
import { getKnowledge, type KnowledgeItem } from '../services/apis/knowledge'
import { useKnowledgeChunks } from '../composables/useKnowledgeChunks'
import { isKnowledgeVectorActive, knowledgeError } from '../utils/knowledge'
import KnowledgeVectorPanel from '../components/knowledge/KnowledgeVectorPanel.vue'

const route = useRoute()
const id = computed(() => Number(route.params.id))
// Citation links carry ?chunk=<seq> so the matching passage is highlighted.
const highlightSeq = computed(() => {
  const raw = Number(route.query.chunk)
  return Number.isInteger(raw) && raw > 0 ? raw : 0
})
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
const failedCount = computed(() => items.value.filter(chunk => chunk.state === 'failed').length)
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
    await nextTick()
    if (highlightSeq.value) locate(highlightSeq.value)
    if (isKnowledgeVectorActive(data.vector_state)) timer = setTimeout(refresh, 3000)
  } catch (e) {
    if (!current.signal.aborted) detailError.value = knowledgeError(e)
  }
}
watch(
  () => route.query.chunk,
  async () => {
    await nextTick()
    if (highlightSeq.value) locate(highlightSeq.value)
  },
)
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
  <main class="shell-page">
    <!-- 原型 ②「长文阅读」的双列变体：**没有页头带**（DESIGN.md:244）。
         容器档是 panel（896）而不是 measure：DESIGN.md:250 明确这是唯一被授权
         突破 measure 的长文场景 —— 主列 reading measure，右列分块列表可用到 panel。
         左右顺序按规范：**主列在左、分块列表在右**（旧版是左目录右正文，反了）。 -->
    <div class="shell-panel">
      <nav
        data-test="breadcrumb"
        aria-label="面包屑"
        class="mb-5 flex flex-wrap gap-3 text-body text-label-secondary"
      >
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink>
        <span>/</span>
        <RouterLink :to="`/home/knowledge/${id}`" class="text-primary-pressed">知识详情</RouterLink>
        <span>/ 分块结构</span>
      </nav>
      <header class="mb-6">
        <!-- 计数与筛选放同一行：筛选管的是整张列表，不是右侧大纲的一部分。
             放进大纲卡会让窄屏用户必须先滚到列表底部才能摸到它（大纲在 DOM 里排第二）。 -->
        <div class="flex flex-wrap items-baseline justify-between gap-3">
          <p class="font-type text-micro text-label-tertiary">
            共 {{ items.length }} 块<template v-if="failedCount">
              · {{ failedCount }} 块失败</template
            >
          </p>
          <label class="flex cursor-pointer items-center gap-1.5 text-note text-label-secondary">
            <input v-model="failedOnly" type="checkbox" class="accent-primary" />
            只看失败
          </label>
        </div>
        <h1 class="mt-2 break-all text-display">{{ item?.title || '分块结构' }}</h1>
        <p class="mt-3 text-body leading-6 text-label-secondary">
          按写入顺序查看文档切分。每块最多 512 字符，同章节相邻块最多重叠 64 字符。
        </p>
      </header>
      <p
        v-if="error || detailError"
        role="alert"
        class="mb-5 bg-warning-tint p-4 text-warning-strong"
      >
        {{ error || detailError }}
        <button class="knowledge-text-action" type="button" @click="refresh">重新加载</button>
      </p>
      <div class="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_15rem]">
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
            class="border border-hairline bg-surface p-8 text-body text-label-secondary"
          >
            {{ failedOnly ? '没有失败分块。' : '尚未生成分块，任务开始后会自动展示。' }}
          </p>
          <!-- 分块是长文的最小单位，同样不套行框：块与块之间只靠一条规则线分节。 -->
          <article
            v-for="chunk in visible"
            :id="`chunk-${chunk.seq}`"
            :key="chunk.id"
            class="scroll-mt-4 border-t border-rule pt-4"
            :class="chunk.seq === highlightSeq ? 'border-primary ring-2 ring-primary' : ''"
          >
            <header class="mb-3 flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="break-words text-title">
                  <span class="font-type">#{{ String(chunk.seq).padStart(2, '0') }}</span>
                  · {{ chunk.section }}
                </h2>
                <p class="mt-1 text-note text-label-secondary">
                  <span class="font-type">{{ chunk.chars }}</span> 字符 ·
                  <span class="font-type">[{{ chunk.start_char }}, {{ chunk.end_char }})</span>
                </p>
              </div>
              <span
                class="px-2 py-1 text-note"
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
            <p v-if="chunk.overlap_chars" class="mb-3 text-note text-primary-pressed">
              与上一块重叠 {{ chunk.overlap_chars }} 字符，以下高亮为重复上下文。
            </p>
            <!-- text-read（16px）是长文正文唯一合法档位（R5：measure 按元素自身字号算，
                 13px 下 40em 只有 520px，一行读不到 40 个汉字）。 -->
            <pre
              class="whitespace-pre-wrap break-words font-sans text-read"
            ><mark v-if="chunk.overlap_chars" class="bg-primary-tint text-primary-pressed">{{ leading(chunk.content, chunk.overlap_chars) }}</mark>{{ remainder(chunk.content, chunk.overlap_chars) }}</pre>
            <p
              v-if="chunk.error_message"
              role="status"
              class="mt-4 bg-danger-tint p-3 text-body text-danger"
            >
              {{ chunk.error_message }}
            </p>
          </article>
        </section>
        <aside class="space-y-5 lg:sticky lg:top-4">
          <section class="border-t border-rule pt-3">
            <h2 class="text-body font-semibold">章节大纲</h2>
            <p class="mt-2 text-note leading-5 text-label-secondary">
              字符范围从 0 开始，结束位置不包含在块内。
            </p>
            <nav aria-label="章节大纲" class="mt-3 space-y-px">
              <button
                v-for="section in sections"
                :key="section.key"
                type="button"
                class="w-full px-2 py-1.5 text-left text-note leading-5 hover:bg-primary-tint"
                @click="locate(section.first)"
              >
                <span class="block break-words">{{ section.title }}</span>
                <span class="font-type text-label-tertiary"
                  >#{{ section.first }} - #{{ section.last }}</span
                >
              </button>
            </nav>
          </section>
          <KnowledgeVectorPanel v-if="item" :item="item" />
          <p class="border-t border-rule pt-3 text-note leading-5 text-label-secondary">
            <AppIcon
              icon="material-symbols:info-outline-rounded"
              class="atom-glyph mr-1"
              aria-hidden="true"
              :style="{ '--atom-glyph-size': '14px' }"
            />引用链接带 <span class="font-type">?chunk=N</span>，打开时自动定位到对应分块。
          </p>
        </aside>
      </div>
    </div>
  </main>
</template>
