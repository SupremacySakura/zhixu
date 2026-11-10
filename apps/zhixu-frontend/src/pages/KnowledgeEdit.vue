<script setup lang="ts">
import { computed, onScopeDispose, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createKnowledge } from '../services/apis/knowledge'
import { knowledgeError } from '../utils/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'
const router = useRouter()
// 与后端 service.ManualContentMaxRunes 对齐。
const CONTENT_MAX_RUNES = 2_000_000
const title = ref('')
const content = ref('')
const error = ref('')
const saving = ref(false)
let controller: AbortController | null = null
const contentRunes = computed(() => [...content.value].length)
const previewHtml = computed(() => renderKnowledgeContent(content.value, 'markdown'))
const canSave = computed(
  () =>
    !saving.value &&
    title.value.trim() !== '' &&
    [...title.value.trim()].length <= 200 &&
    content.value.trim() !== '' &&
    contentRunes.value <= CONTENT_MAX_RUNES,
)
async function save() {
  if (!canSave.value) return
  saving.value = true
  error.value = ''
  controller = new AbortController()
  try {
    const item = await createKnowledge(title.value.trim(), content.value, controller.signal)
    if (controller.signal.aborted) return
    await router.replace(`/home/knowledge/${item.id}`)
  } catch (e) {
    if (!controller.signal.aborted) error.value = knowledgeError(e)
  } finally {
    saving.value = false
  }
}
onScopeDispose(() => controller?.abort())
</script>
<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <div class="mb-5 flex gap-3 text-subhead text-label-secondary">
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink
        ><span>/</span><span>新建知识</span>
      </div>
      <header class="mb-7">
        <h1 class="text-title-1">新建知识</h1>
        <p class="mt-2 text-subhead text-label-secondary">
          直接写下想法或笔记，保存后自动完成分块与向量化，可参与检索与问答。
        </p>
      </header>
      <p
        v-if="error"
        role="alert"
        class="mb-5 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
      >
        {{ error }}
      </p>
      <form class="space-y-5" @submit.prevent="save">
        <div>
          <label for="knowledge-title" class="mb-2 block text-caption text-label-secondary"
            >标题</label
          ><input
            id="knowledge-title"
            v-model="title"
            maxlength="200"
            required
            class="knowledge-input w-full"
            placeholder="给这份知识起个标题"
          />
        </div>
        <div class="grid items-start gap-5 xl:grid-cols-2">
          <div>
            <label
              for="knowledge-content"
              class="mb-2 flex items-center justify-between gap-3 text-caption text-label-secondary"
              ><span>正文（Markdown）</span
              ><span>{{ contentRunes }} / {{ CONTENT_MAX_RUNES }} 字符</span></label
            ><textarea
              id="knowledge-content"
              v-model="content"
              rows="18"
              required
              class="knowledge-textarea"
              placeholder="粘贴或输入知识正文，支持 Markdown 语法。"
            />
          </div>
          <section class="rounded-lg border border-hairline bg-surface p-6" aria-label="正文预览">
            <p class="mb-4 text-caption text-label-secondary">预览</p>
            <!-- Parser disables HTML, unsafe protocols, and all image requests. -->
            <div v-if="content.trim()" class="knowledge-markdown" v-html="previewHtml" />
            <p v-else class="text-subhead leading-7 text-label-secondary">正文预览会显示在这里。</p>
          </section>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <button type="submit" class="knowledge-button-primary" :disabled="!canSave">
            {{ saving ? '保存中…' : '保存知识' }}
          </button>
          <RouterLink to="/home/knowledge" class="knowledge-button-secondary">取消</RouterLink>
          <span class="text-caption text-label-secondary">
            保存后自动分块并向量化，可在知识库中查看进度。
          </span>
        </div>
      </form>
    </div>
  </main>
</template>
