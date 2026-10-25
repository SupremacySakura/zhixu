<script setup lang="ts">
import { onMounted, onScopeDispose, ref } from 'vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import {
  deleteKnowledge,
  type KnowledgeItem,
  type KnowledgeStatus,
  type KnowledgeVectorState,
} from '../services/apis/knowledge'
import { hasKnowledgeVector, knowledgeDescription, knowledgeError } from '../utils/knowledge'
import { Icon } from '@iconify/vue'
import { useKnowledgeLibrary } from '../composables/useKnowledgeLibrary'
import { useKnowledgeImport } from '../composables/useKnowledgeImport'
import { useToast } from '../composables/useToast'
import { knowledgeDate } from '../utils/knowledge'
import KnowledgeActionMenu from '../components/knowledge/KnowledgeActionMenu.vue'
import KnowledgeBadge from '../components/knowledge/KnowledgeBadge.vue'
import KnowledgeVectorBadge from '../components/knowledge/KnowledgeVectorBadge.vue'
import KnowledgeTaskPanel from '../components/knowledge/KnowledgeTaskPanel.vue'
import FilePagination from '../components/file-management/FilePagination.vue'
import ToastNotice from '../components/file-management/ToastNotice.vue'
const { items, total, page, search, status, vector, loading, error, load, changePage } =
  useKnowledgeLibrary()
const { displayToast, showToast, toastMessage, toastType } = useToast()
const { item, panelOpen, panelError, busy, track, retry, poll } = useKnowledgeImport(displayToast)
const statusOptions = [
  { key: '', label: '全部状态' },
  { key: 'ready', label: '已加入' },
  { key: 'pending', label: '排队中' },
  { key: 'running', label: '处理中' },
  { key: 'failed', label: '加入失败' },
]
const vectorOptions = [
  { key: '', label: '全部向量状态' },
  { key: 'ready', label: '已向量化' },
  { key: 'running', label: '向量化中' },
  { key: 'partial', label: '部分失败' },
  { key: 'pending', label: '待向量化' },
]
const deleteTarget = ref<KnowledgeItem | null>(null)
const deleting = ref(false)
const deleteError = ref('')
let deleteController: AbortController | null = null
function selectDelete(entry: KnowledgeItem) {
  deleteTarget.value = entry
  deleteError.value = ''
}
async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  deleteController = new AbortController()
  try {
    await deleteKnowledge(deleteTarget.value.id, deleteController.signal)
    if (deleteController.signal.aborted) return
    if (item.value?.id === deleteTarget.value.id) {
      panelOpen.value = false
      item.value = null
    }
    deleteTarget.value = null
    displayToast('知识已删除，原文件保持不变', 'success')
    await load()
  } catch (e) {
    if (!deleteController.signal.aborted) deleteError.value = knowledgeError(e)
  } finally {
    deleting.value = false
  }
}
onScopeDispose(() => deleteController?.abort())
function closePanel() {
  panelOpen.value = false
  void load()
}
onMounted(() => {
  void load()
})
</script>
<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <p class="mb-4 text-subhead text-label-secondary">知识库 / 全部知识</p>
      <header class="mb-7 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="text-title-1">知识库</h1>
          <p class="mt-2 text-subhead text-label-secondary">
            从文件中沉淀的内容，在这里继续阅读与积累。
          </p>
        </div>
        <RouterLink to="/home/files" class="knowledge-button-primary"
          ><Icon icon="material-symbols:note-add-outline" />从文件加入</RouterLink
        >
      </header>
      <div class="mb-5 flex flex-wrap gap-3">
        <div class="relative min-w-48 flex-1">
          <Icon
            class="absolute left-3 top-3.5 text-label-secondary"
            icon="material-symbols:search"
          /><input
            v-model="search"
            maxlength="200"
            class="knowledge-input w-full pl-10"
            placeholder="搜索知识标题"
            aria-label="搜索知识标题"
          />
        </div>
        <FilterMenu
          :model-value="status"
          :options="statusOptions"
          label="筛选处理状态"
          @update:model-value="status = $event as KnowledgeStatus | ''"
        />
        <FilterMenu
          :model-value="vector"
          :options="vectorOptions"
          label="筛选向量状态"
          @update:model-value="vector = $event as KnowledgeVectorState"
        />
      </div>
      <div
        v-if="error"
        class="mb-5 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
        role="alert"
      >
        {{ error }} <button class="knowledge-text-action" @click="load()">重试加载</button>
      </div>
      <p v-if="loading" class="py-8 text-center text-subhead text-label-secondary" role="status">
        正在加载知识…
      </p>
      <div
        v-else-if="!error && !items.length"
        class="rounded-lg border border-hairline bg-surface px-6 py-16 text-center"
      >
        <span
          class="mx-auto mb-5 flex h-16 w-14 items-center justify-center rounded-lg bg-primary-tint text-headline font-bold text-primary-pressed"
          >MD</span
        >
        <h2 class="text-title-3">
          {{ search || status || vector ? '没有匹配的知识' : '让第一份资料成为知识' }}
        </h2>
        <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
          {{
            search || status || vector
              ? '试试其他关键词或切换筛选状态。'
              : '选择已经上传的 Markdown 文件，保留原件，同时获得清晰可读的知识正文。'
          }}
        </p>
        <RouterLink to="/home/files" class="knowledge-button-primary mt-5"
          >去文件管理选择</RouterLink
        >
      </div>
      <div v-else-if="!loading" class="space-y-3">
        <article
          v-for="entry in items"
          :key="entry.id"
          class="flex items-start justify-between gap-3 rounded-lg border border-hairline bg-surface p-5 sm:items-center sm:gap-4 sm:p-6"
        >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <KnowledgeBadge :status="entry.status" />
              <KnowledgeVectorBadge
                v-if="hasKnowledgeVector(entry)"
                :state="entry.vector_state as Exclude<KnowledgeVectorState, ''>"
              />
            </div>
            <h2 class="mb-2 mt-3 break-all text-headline">{{ entry.title }}</h2>
            <p class="text-subhead leading-6 text-label-secondary">
              {{ knowledgeDescription(entry) }}
            </p>
            <div class="mt-3 flex flex-wrap gap-3 text-caption text-label-secondary">
              <span>Markdown 文件</span><span class="break-all">{{ entry.source_file_name }}</span
              ><time>{{ knowledgeDate(entry.created_at) }}</time>
            </div>
          </div>
          <KnowledgeActionMenu :item="entry" @track="track" @delete="selectDelete" />
        </article>
      </div>
      <FilePagination
        v-if="total > 0"
        class="mt-5 rounded-lg"
        :page="page"
        :total-pages="Math.max(1, Math.ceil(total / 20))"
        :start-index="(page - 1) * 20 + 1"
        :end-index="Math.min(page * 20, total)"
        :total-count="total"
        :loading="loading"
        @change="changePage"
      />
      <p class="mt-5 text-caption leading-6 text-label-secondary">
        首版支持 Markdown 正文阅读。AI 摘要、标签与智能问答将在后续加入。
      </p>
    </div>
    <KnowledgeTaskPanel
      :item="item"
      :open="panelOpen"
      :error="panelError"
      :busy="busy"
      @close="closePanel"
      @retry="retry"
      @refresh="poll"
    />
    <ToastNotice :visible="showToast" :message="toastMessage" :type="toastType" />
    <ConfirmDialog
      :model-value="Boolean(deleteTarget)"
      title="删除知识条目"
      :message="`将删除「${deleteTarget?.title || ''}」及其处理任务，原文件保持不变。此操作不可撤销。`"
      confirm-text="删除知识"
      danger
      :loading="deleting"
      @cancel="deleteTarget = null"
      @confirm="confirmDelete"
    >
      <p v-if="deleteError" role="alert" class="mb-4 text-subhead text-danger">{{ deleteError }}</p>
    </ConfirmDialog>
  </main>
</template>
