<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getKnowledge, type KnowledgeItem } from '../services/apis/knowledge'
import {
  isKnowledgeActive,
  isKnowledgeVectorActive,
  knowledgeDate,
  knowledgeError,
} from '../utils/knowledge'
import { renderMarkdown } from '../utils/markdown'
import KnowledgeBadge from '../components/knowledge/KnowledgeBadge.vue'
import KnowledgeVectorPanel from '../components/knowledge/KnowledgeVectorPanel.vue'
const route = useRoute()
const item = ref<KnowledgeItem | null>(null)
const error = ref('')
const loading = ref(false)
let controller: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const html = computed(() => renderMarkdown(item.value?.content || ''))
async function load(background = false) {
  controller?.abort()
  clearTimeout(timer)
  const current = new AbortController()
  controller = current
  if (!background) {
    item.value = null
    loading.value = true
  }
  error.value = ''
  try {
    const data = await getKnowledge(Number(route.params.id), current.signal)
    if (current.signal.aborted) return
    item.value = data
    if (isKnowledgeActive(data.status) || isKnowledgeVectorActive(data.vector_state))
      timer = setTimeout(() => load(true), 3000)
  } catch (e) {
    if (!current.signal.aborted) error.value = knowledgeError(e)
  } finally {
    if (controller === current) loading.value = false
  }
}
watch(
  () => route.params.id,
  () => load(),
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
      <div class="mb-5 flex gap-3 text-subhead text-label-secondary">
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink
        ><span>/</span><span>知识详情</span>
      </div>
      <p v-if="loading" class="py-12 text-center" role="status">正在读取知识正文…</p>
      <div
        v-if="error"
        class="mb-5 rounded-lg bg-warning-tint p-4 text-warning-strong"
        role="alert"
      >
        {{ error }} <button class="knowledge-text-action" @click="load()">重新加载</button>
      </div>
      <template v-if="item">
        <header class="mb-7 flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0">
            <h1 class="break-all text-title-1">{{ item.title }}</h1>
            <p class="mt-2 text-subhead text-label-secondary">来源于 Markdown 文件 · 原始正文</p>
          </div>
          <RouterLink
            :to="{
              path: '/home/files',
              query: { folder: item.source_folder_id, folder_name: item.source_folder_name },
            }"
            class="knowledge-button-secondary"
            >查看来源目录 →</RouterLink
          >
        </header>
        <div class="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_15rem]">
          <article
            class="min-w-0 rounded-lg border border-hairline bg-surface p-6 sm:p-9"
            aria-label="知识正文"
          >
            <div
              class="mb-7 flex flex-wrap items-center justify-between gap-3 border-b border-hairline pb-4"
            >
              <span class="text-caption text-label-secondary">正文 · Markdown</span
              ><KnowledgeBadge :status="item.status" />
            </div>
            <!-- Parser disables HTML, unsafe protocols, and all image requests. -->
            <div v-if="item.status === 'ready'" class="knowledge-markdown" v-html="html" />
            <p v-else class="text-subhead leading-7 text-label-secondary" role="status">
              {{
                item.status === 'failed' ? item.error_message : '正文仍在处理，完成后会自动展示。'
              }}
            </p>
            <RouterLink
              v-if="item.status === 'failed'"
              to="/home/knowledge"
              class="knowledge-text-action mt-4"
              >返回知识库重试</RouterLink
            >
          </article>
          <div class="space-y-5">
            <KnowledgeVectorPanel :item="item" />
            <aside class="rounded-lg border border-hairline bg-surface p-5">
              <h2 class="mb-5 text-body font-semibold">来源信息</h2>
              <dl class="space-y-5">
                <div>
                  <dt class="text-caption text-label-secondary">原文件</dt>
                  <dd class="mt-1.5 break-all text-subhead">{{ item.source_file_name }}</dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">来源目录（加入时）</dt>
                  <dd class="mt-1.5 text-subhead">{{ item.source_folder_name }}</dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">来源类型</dt>
                  <dd class="mt-1.5 text-subhead">Markdown 文件</dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">加入时间</dt>
                  <dd class="mt-1.5 text-subhead">{{ knowledgeDate(item.imported_at) }}</dd>
                </div>
              </dl>
              <p
                class="mt-6 border-t border-hairline pt-4 text-caption leading-6 text-label-secondary"
              >
                来源信息为加入时的快照。原文件改名、移动或删除不影响已保存正文。
              </p>
            </aside>
          </div>
        </div>
      </template>
    </div>
  </main>
</template>
