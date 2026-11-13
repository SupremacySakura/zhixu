<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { AnimatePresence, Motion } from 'motion-v'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import {
  getKnowledgeVersion,
  listKnowledgeVersions,
  restoreKnowledgeVersion,
  type KnowledgeItem,
  type KnowledgeVersionDetail,
  type KnowledgeVersionSummary,
} from '../../services/apis/knowledge'
import { knowledgeDate, knowledgeError } from '../../utils/knowledge'
import { renderKnowledgeContent } from '../../utils/markdown'
import { fadeTransition, springSnappy } from '../../utils/motion'
const props = defineProps<{ knowledgeId: number }>()
const emit = defineEmits<{ close: []; restored: [item: KnowledgeItem] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const id = useId()
const versions = ref<KnowledgeVersionSummary[]>([])
const selected = ref(0)
const detail = ref<KnowledgeVersionDetail | null>(null)
const loading = ref(true)
const detailLoading = ref(false)
const busy = ref(false)
const error = ref('')
let previous: HTMLElement | null = null
let controller: AbortController | null = null
const html = computed(() => renderKnowledgeContent(detail.value?.content || '', 'markdown'))
const selectedVersion = computed(() =>
  versions.value.find(v => v.version_number === selected.value),
)
const changeSourceLabel = (source: string) => (source === 'ai' ? 'AI 整理' : '手动编辑')
async function loadVersions() {
  loading.value = true
  error.value = ''
  controller?.abort()
  const current = new AbortController()
  controller = current
  try {
    const data = await listKnowledgeVersions(props.knowledgeId, current.signal)
    if (current.signal.aborted) return
    versions.value = data.items
    const target =
      data.items.find(v => v.current)?.version_number ?? data.items[0]?.version_number ?? 0
    if (target > 0) await selectVersion(target)
  } catch (e) {
    if (!current.signal.aborted) error.value = knowledgeError(e)
  } finally {
    loading.value = false
  }
}
async function selectVersion(version: number) {
  selected.value = version
  detailLoading.value = true
  error.value = ''
  try {
    detail.value = await getKnowledgeVersion(props.knowledgeId, version)
  } catch (e) {
    detail.value = null
    error.value = knowledgeError(e)
  } finally {
    detailLoading.value = false
  }
}
async function restore() {
  if (busy.value || !selectedVersion.value || selectedVersion.value.current) return
  busy.value = true
  error.value = ''
  try {
    const item = await restoreKnowledgeVersion(props.knowledgeId, selected.value)
    emit('restored', item)
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    busy.value = false
  }
}
function close() {
  if (!busy.value) emit('close')
}
onMounted(async () => {
  previous = document.activeElement as HTMLElement | null
  dialog.value?.showModal()
  await loadVersions()
})
onBeforeUnmount(() => {
  controller?.abort()
  dialog.value?.close()
  previous?.focus()
})
</script>
<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="knowledge-dialog"
      :aria-labelledby="id"
      @cancel.prevent="close"
      @click.self="close"
    >
      <AnimatePresence>
        <Motion :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :transition="fadeTransition">
          <Motion
            :initial="{ opacity: 0, scale: 0.96 }"
            :animate="{ opacity: 1, scale: 1 }"
            :transition="springSnappy"
            class="w-[min(56rem,calc(100vw-2rem))] bg-surface shadow-popover"
          >
            <div class="flex items-center justify-between gap-3 border-b border-hairline p-6">
              <div>
                <h2 :id="id" class="text-title-3">版本历史</h2>
                <p class="mt-1 text-caption text-label-secondary">
                  恢复历史版本会生成一个新版本，已有版本不会被删除。
                </p>
              </div>
              <button
                type="button"
                class="knowledge-icon-button"
                :disabled="busy"
                aria-label="关闭弹窗"
                @click="close"
              >
                <AppIcon icon="material-symbols:close-rounded" />
              </button>
            </div>
            <p
              v-if="error"
              role="alert"
              class="mx-6 mt-4 bg-warning-tint p-3 text-subhead text-warning-strong"
            >
              {{ error }}
            </p>
            <p v-if="loading" class="p-10 text-center text-subhead" role="status">
              正在读取版本历史…
            </p>
            <div
              v-else-if="versions.length"
              class="grid gap-5 p-6 sm:grid-cols-[13rem_minmax(0,1fr)]"
            >
              <div class="max-h-80 space-y-2 overflow-y-auto pr-1" aria-label="版本列表">
                <button
                  v-for="version in versions"
                  :key="version.version_number"
                  type="button"
                  class="w-full border p-3 text-left transition"
                  :class="
                    version.version_number === selected
                      ? 'border-primary bg-primary-tint'
                      : 'border-hairline hover:bg-surface-tertiary'
                  "
                  @click="selectVersion(version.version_number)"
                >
                  <span class="flex items-center justify-between gap-2 text-subhead font-semibold">
                    第 {{ version.version_number }} 版
                    <span
                      v-if="version.current"
                      class="bg-primary-tint px-1.5 py-0.5 text-caption text-primary-pressed"
                      >当前</span
                    >
                  </span>
                  <span class="mt-1 block text-caption text-label-secondary"
                    >{{ knowledgeDate(version.created_at) }} ·
                    {{ changeSourceLabel(version.change_source) }}</span
                  >
                </button>
              </div>
              <div class="min-w-0">
                <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <p class="text-subhead font-semibold">
                    第 {{ selectedVersion?.version_number }} 版预览
                    <span v-if="selectedVersion?.current" class="text-caption text-label-secondary"
                      >（当前版本）</span
                    >
                  </p>
                  <span class="text-caption text-label-secondary">{{ detail?.title }}</span>
                </div>
                <p
                  v-if="detailLoading"
                  class="border border-hairline p-6 text-center text-subhead"
                  role="status"
                >
                  正在读取版本正文…
                </p>
                <div
                  v-else
                  class="knowledge-markdown max-h-80 overflow-y-auto border border-hairline p-5"
                  aria-label="版本正文预览"
                  v-html="html"
                />
              </div>
            </div>
            <p v-else class="p-10 text-center text-subhead">暂无历史版本。</p>
            <div class="flex justify-end gap-2 border-t border-hairline p-5">
              <button
                type="button"
                class="knowledge-button-secondary"
                :disabled="busy"
                @click="close"
              >
                关闭
              </button>
              <button
                type="button"
                class="knowledge-button-primary"
                :disabled="busy || loading || !selectedVersion || selectedVersion.current"
                @click="restore"
              >
                <AppLoading v-if="busy" size="sm" />{{ busy ? '恢复中…' : '恢复此版本' }}
              </button>
            </div>
          </Motion>
        </Motion>
      </AnimatePresence>
    </dialog>
  </Teleport>
</template>
