<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppButton from '../components/ui/AppButton.vue'
import { createKnowledge, getKnowledge, updateKnowledge } from '../services/apis/knowledge'
import { isKnowledgeConflict, knowledgeError } from '../utils/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'
const route = useRoute()
const router = useRouter()
// 与后端 service.ManualContentMaxRunes 对齐。
const CONTENT_MAX_RUNES = 2_000_000
const title = ref('')
const content = ref('')
const error = ref('')
const loading = ref(false)
const saving = ref(false)
// 乐观锁基线：编辑模式下为加载时的 version，提交时回传，冲突时后端返回 409。
const baseVersion = ref(0)
let controller: AbortController | null = null
const editing = computed(() => route.params.id !== undefined)
const knowledgeId = computed(() => Number(route.params.id))
const contentRunes = computed(() => [...content.value].length)
const previewHtml = computed(() => renderKnowledgeContent(content.value, 'markdown'))
const canSave = computed(
  () =>
    !saving.value &&
    !loading.value &&
    title.value.trim() !== '' &&
    [...title.value.trim()].length <= 200 &&
    content.value.trim() !== '' &&
    contentRunes.value <= CONTENT_MAX_RUNES,
)
async function load() {
  controller?.abort()
  if (!editing.value) return
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  try {
    const item = await getKnowledge(knowledgeId.value, current.signal)
    if (current.signal.aborted) return
    title.value = item.title
    content.value = item.content
    baseVersion.value = item.version
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
async function save() {
  if (!canSave.value) return
  saving.value = true
  error.value = ''
  controller = new AbortController()
  try {
    const item = editing.value
      ? await updateKnowledge(
          knowledgeId.value,
          title.value.trim(),
          content.value,
          baseVersion.value,
          controller.signal,
        )
      : await createKnowledge(title.value.trim(), content.value, controller.signal)
    if (controller.signal.aborted) return
    await router.replace(`/home/knowledge/${item.id}`)
  } catch (e) {
    if (controller.signal.aborted) return
    error.value = isKnowledgeConflict(e)
      ? '内容已在其他页面被修改，请刷新后重新编辑。'
      : knowledgeError(e)
  } finally {
    saving.value = false
  }
}
onScopeDispose(() => controller?.abort())
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <!-- 原型 ④「表单」：容器 panel（896）、页头带 64px。
         面包屑保留（三层以上页面，共 3 处之一），压在带子里的标题上方。 -->
    <header class="shell-band-panel">
      <div class="min-w-0 flex-1">
        <div data-test="breadcrumb" class="flex gap-2 text-body text-label-secondary">
          <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink
          ><span>/</span><span>{{ editing ? '编辑知识' : '新建知识' }}</span>
        </div>
        <h1 class="text-title">{{ editing ? '编辑知识' : '新建知识' }}</h1>
      </div>
    </header>

    <main class="shell-page">
      <div class="shell-panel">
        <p class="mb-6 text-body text-label-secondary">
          {{
            editing
              ? '修改标题或正文，保存后自动重新分块并向量化，旧内容会保留为历史版本。'
              : '直接写下想法或笔记，保存后自动完成分块与向量化，可参与检索与问答。'
          }}
        </p>
        <p v-if="error" role="alert" class="mb-5 bg-warning-tint p-4 text-body text-warning-strong">
          {{ error }}
        </p>
        <p v-if="loading" class="py-12 text-center" role="status">正在读取知识正文…</p>
        <form v-else @submit.prevent="save">
          <!-- 字段间距 24px（DESIGN.md:269「表单字段 24 / 分节 48」）。 -->
          <div>
            <label for="knowledge-title" class="atom-field-label">标题</label
            ><input
              id="knowledge-title"
              v-model="title"
              maxlength="200"
              required
              class="atom-field atom-tone-secondary w-full"
              placeholder="给这份知识起个标题"
            />
          </div>
          <!-- 正文与预览用上下两段而不是左右双列：panel 896 切成两列后每列 424，
               预览里的正文（16/28）只剩 26 个汉字一行，比 measure 的 40 字差得远。
               上下排布时两段都能吃到完整 640px 的阅读宽度。 -->
          <div class="mt-6 border-t border-hairline pt-6">
            <label for="knowledge-content" class="atom-field-label flex justify-between gap-3"
              ><span>正文（Markdown）</span
              ><span class="font-type"
                >{{ contentRunes }} / {{ CONTENT_MAX_RUNES }} 字符</span
              ></label
            ><textarea
              id="knowledge-content"
              v-model="content"
              rows="18"
              required
              class="atom-field atom-field-multiline atom-tone-secondary"
              placeholder="粘贴或输入知识正文，支持 Markdown 语法。"
            />
          </div>
          <section class="mt-6 border-t border-hairline pt-6" aria-label="正文预览">
            <p class="mb-3 text-note text-label-secondary">预览</p>
            <!-- Parser disables HTML, unsafe protocols, and all image requests. -->
            <div v-if="content.trim()" class="knowledge-markdown" v-html="previewHtml" />
            <p v-else class="text-body leading-7 text-label-secondary">正文预览会显示在这里。</p>
          </section>
          <div class="mt-8 flex flex-wrap items-center gap-3 border-t border-hairline pt-6">
            <AppButton type="submit" tone="primary" size="lg" :disabled="!canSave">{{
              saving ? '保存中…' : editing ? '保存修改' : '保存知识'
            }}</AppButton>
            <RouterLink to="/home/knowledge" class="shell-band-action">取消</RouterLink>
            <span class="text-note text-label-secondary">
              保存后自动分块并向量化，可在知识库中查看进度。
            </span>
          </div>
        </form>
      </div>
    </main>
  </div>
</template>
