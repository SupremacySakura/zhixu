<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import AppButton from '../ui/AppButton.vue'
import AppLoading from '../ui/AppLoading.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../../utils/motion'
import type { FileDisplayItem, PreviewKind } from '../../types/file'
import { formatBytes } from '../../utils/file'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    file: FileDisplayItem | null
    loading: boolean
    error: string | null
    kind: PreviewKind
    url: string
    textContent: string
    canDownload: boolean
    publicShareLink: string
    creatingShareLink?: boolean
    deletingShareLink?: boolean
  }>(),
  {
    creatingShareLink: false,
    deletingShareLink: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  close: []
  download: []
  'create-share': []
  'delete-share': []
  'copy-share': []
}>()

const closeButtonRef = ref<HTMLButtonElement | null>(null)
const titleId = `file-preview-dialog-title-${useId()}`
const fileName = computed(() => props.file?.name ?? '文件预览')
const fileTypeLabel = computed(() => props.file?.typeLabel ?? 'File')
const fileSize = computed(() => formatBytes(props.file?.size ?? 0))
const lastModifiedText = computed(() => props.file?.lastModifiedText ?? '-')
const fileIconBg = computed(() => props.file?.iconBg ?? 'bg-primary-tint')
const fileIconFg = computed(() => props.file?.iconFg ?? 'text-primary')
const fileIcon = computed(() => props.file?.icon ?? 'material-symbols:description')

const handleClose = () => {
  emit('update:modelValue', false)
  emit('close')
}

watch(
  () => props.modelValue,
  async visible => {
    if (!visible) return
    await nextTick()
    closeButtonRef.value?.focus()
  },
)
</script>

<template>
  <Teleport to="body">
    <AnimatePresence>
      <Motion
        v-if="modelValue"
        key="file-preview-overlay"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4 sm:p-6 md:p-10"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @click.self="handleClose"
        @keydown.esc="handleClose"
      >
        <Motion
          :initial="{ opacity: 0, scale: 0.96 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.98 }"
          :transition="springSnappy"
          class="relative flex h-full max-h-[850px] w-full max-w-wide flex-col overflow-hidden bg-surface shadow-popover"
        >
          <!-- 头部：类型图标 + 名称 + 元数据 + 圆形关闭 -->
          <div
            class="flex flex-none items-center justify-between border-b border-hairline px-6 py-4"
          >
            <div class="flex min-w-0 items-center gap-3">
              <div
                class="flex h-10 w-10 shrink-0 items-center justify-center"
                :class="[fileIconBg, fileIconFg]"
              >
                <AppIcon
                  class="atom-glyph"
                  :icon="fileIcon"
                  :style="{ '--atom-glyph-size': '22px' }"
                />
              </div>
              <div class="min-w-0">
                <h2
                  :id="titleId"
                  class="max-w-[60vw] truncate text-title font-semibold tracking-[-0.01em] text-label"
                >
                  {{ fileName }}
                </h2>
                <p class="text-note text-label-tertiary">{{ fileTypeLabel }} • {{ fileSize }}</p>
              </div>
            </div>
            <button
              ref="closeButtonRef"
              class="flex h-9 w-9 shrink-0 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-secondary hover:text-label"
              type="button"
              aria-label="关闭预览"
              @click="handleClose"
            >
              <AppIcon icon="material-symbols:close" />
            </button>
          </div>

          <div class="flex min-h-0 flex-1 flex-col overflow-hidden md:flex-row">
            <!-- 预览主区 -->
            <div class="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-surface-secondary">
              <div class="flex-1 overflow-y-auto p-6 pb-24 md:p-8 md:pb-24">
                <div
                  v-if="loading"
                  class="flex min-h-full flex-col items-center justify-center gap-3 text-label-secondary"
                >
                  <AppLoading :size="30" class="text-primary" />
                  <span class="text-body">正在加载预览...</span>
                </div>

                <div
                  v-else-if="error"
                  class="flex min-h-full flex-col items-center justify-center px-6 text-center text-danger"
                >
                  <AppIcon
                    icon="material-symbols:error"
                    class="mb-2 atom-glyph"
                    :style="{ '--atom-glyph-size': '36px' }"
                  />
                  <p class="text-lead font-semibold">{{ error }}</p>
                </div>

                <div
                  v-else-if="kind === 'image' && url"
                  class="flex min-h-full w-full items-start justify-center"
                >
                  <img
                    :src="url"
                    :alt="fileName"
                    class="h-auto max-w-full border border-hairline bg-surface object-contain shadow-card"
                  />
                </div>

                <iframe
                  v-else-if="kind === 'pdf' && url"
                  :src="url"
                  class="h-[calc(100vh-300px)] min-h-[520px] w-full border border-hairline bg-surface"
                  title="PDF 预览"
                ></iframe>

                <div
                  v-else-if="kind === 'video' && url"
                  class="flex min-h-full w-full items-start justify-center"
                >
                  <video controls :src="url" class="max-w-full bg-black"></video>
                </div>

                <div
                  v-else-if="kind === 'audio' && url"
                  class="flex min-h-full items-center justify-center"
                >
                  <audio controls :src="url" class="w-full max-w-2xl"></audio>
                </div>

                <pre
                  v-else-if="kind === 'text'"
                  class="h-full w-full overflow-auto whitespace-pre-wrap break-words border border-hairline bg-surface p-4 text-body text-label"
                  >{{ textContent || '文件为空' }}</pre
                >

                <div
                  v-else
                  class="flex min-h-full flex-col items-center justify-center px-6 text-center text-label-secondary"
                >
                  <slot name="unsupported" :file="file">
                    <AppIcon
                      icon="material-symbols:description"
                      class="mb-2 atom-glyph"
                      :style="{ '--atom-glyph-size': '36px' }"
                    />
                    <p class="text-lead font-semibold">当前文件类型暂不支持在线预览</p>
                    <p class="mt-1 text-note">你可以使用下方按钮下载后查看</p>
                  </slot>
                </div>
              </div>

              <!-- 浮动下载条：浮在预览画面上的一条窄工具条 -->
              <div
                class="absolute bottom-6 left-1/2 z-toolbar flex -translate-x-1/2 items-center gap-1 border border-hairline bg-surface p-1.5 shadow-floating"
              >
                <AppButton
                  tone="primary"
                  icon="material-symbols:download"
                  :disabled="!canDownload"
                  aria-label="下载文件"
                  @click="emit('download')"
                >
                  下载
                </AppButton>
              </div>
            </div>

            <!-- 详情栏 -->
            <aside
              class="flex max-h-72 w-full shrink-0 flex-col overflow-y-auto border-t border-hairline bg-surface p-6 md:max-h-none md:w-80 md:border-l md:border-t-0"
            >
              <h3
                class="mb-6 text-note font-semibold uppercase tracking-[0.08em] text-label-tertiary"
              >
                文件详情
              </h3>
              <div class="space-y-6">
                <div>
                  <span class="mb-1 block text-note text-label-tertiary">文件名</span>
                  <p class="break-all text-body font-semibold text-label">{{ fileName }}</p>
                </div>
                <div>
                  <span class="mb-1 block text-note text-label-tertiary">文件大小</span>
                  <p class="text-body font-semibold text-label">{{ fileSize }}</p>
                </div>
                <div>
                  <span class="mb-1 block text-note text-label-tertiary">文件类型</span>
                  <p class="text-body font-semibold text-label">{{ fileTypeLabel }}</p>
                </div>
                <div>
                  <span class="mb-1 block text-note text-label-tertiary">最后修改</span>
                  <p class="text-body font-semibold text-label">{{ lastModifiedText }}</p>
                </div>
                <div class="border-t border-hairline pt-5">
                  <span class="mb-2 block text-note text-label-tertiary">
                    公网分享链接（免鉴权访问）
                  </span>
                  <div v-if="publicShareLink" class="space-y-2">
                    <input
                      :value="publicShareLink"
                      readonly
                      aria-label="公网分享链接"
                      class="w-full border border-hairline bg-surface-secondary px-3 py-2 text-body text-label"
                    />
                    <div class="grid grid-cols-2 gap-2">
                      <AppButton
                        tone="primary"
                        icon="material-symbols:content-copy-outline"
                        class="justify-center"
                        aria-label="复制分享链接"
                        @click="emit('copy-share')"
                      >
                        复制
                      </AppButton>
                      <AppButton
                        tone="danger"
                        :icon="deletingShareLink ? undefined : 'material-symbols:delete-outline'"
                        :loading="deletingShareLink"
                        class="justify-center"
                        aria-label="删除分享链接"
                        :disabled="deletingShareLink"
                        @click="emit('delete-share')"
                      >
                        {{ deletingShareLink ? '删除中' : '删除' }}
                      </AppButton>
                    </div>
                  </div>
                  <AppButton
                    v-else
                    tone="primary"
                    :icon="creatingShareLink ? undefined : 'material-symbols:share'"
                    :loading="creatingShareLink"
                    class="w-full justify-center"
                    aria-label="生成公网链接"
                    :disabled="creatingShareLink || !file"
                    @click="emit('create-share')"
                  >
                    {{ creatingShareLink ? '生成中' : '生成公网链接' }}
                  </AppButton>
                </div>
              </div>
            </aside>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
