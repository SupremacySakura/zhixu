<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import AppButton from '../ui/AppButton.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../../utils/motion'
import type { FileListItem } from '../../services/types/file'
import type { BreadcrumbItem, FileDisplayItem } from '../../types/file'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    target: FileDisplayItem | null
    breadcrumbs: readonly BreadcrumbItem[]
    folders: readonly FileListItem[]
    browserLoading: boolean
    targetFolderId: number
    moving?: boolean
  }>(),
  {
    moving: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  cancel: []
  navigate: [index: number]
  'open-folder': [folder: FileListItem]
  'select-current': []
  confirm: []
}>()

const dialogRef = ref<HTMLElement | null>(null)
const titleId = `move-file-dialog-title-${useId()}`

const handleCancel = () => {
  if (props.moving) return
  emit('update:modelValue', false)
  emit('cancel')
}
const isLastBreadcrumb = (index: number) => index === props.breadcrumbs.length - 1
const isOwnFolder = (folder: FileListItem) =>
  props.target?.type === 'folder' && props.target.key === `folder:${folder.id}`

watch(
  () => props.modelValue,
  async visible => {
    if (!visible) return
    await nextTick()
    dialogRef.value?.focus()
  },
)
</script>

<template>
  <Teleport to="body">
    <AnimatePresence>
      <Motion
        v-if="modelValue"
        key="move-file-overlay"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @click.self="handleCancel"
        @keydown.esc="handleCancel"
      >
        <Motion
          :initial="{ opacity: 0, scale: 0.96 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.98 }"
          :transition="springSnappy"
          class="w-full max-w-2xl bg-surface p-6 shadow-popover outline-none"
        >
          <div ref="dialogRef" tabindex="-1">
            <h3 :id="titleId" class="mb-2 text-title">移动到</h3>
            <p class="mb-4 text-note text-label-secondary">当前对象：{{ target?.name ?? '-' }}</p>

            <div class="mb-3 flex items-center justify-between gap-3">
              <nav class="flex items-center gap-1.5 overflow-x-auto text-note text-label-tertiary">
                <template
                  v-for="(breadcrumb, index) in breadcrumbs"
                  :key="`${breadcrumb.id}-${index}`"
                >
                  <button
                    v-if="!isLastBreadcrumb(index)"
                    class="whitespace-nowrap transition-colors hover:text-primary"
                    type="button"
                    @click="emit('navigate', index)"
                  >
                    {{ breadcrumb.name }}
                  </button>
                  <span v-else class="whitespace-nowrap font-semibold text-label">
                    {{ breadcrumb.name }}
                  </span>
                  <AppIcon
                    v-if="!isLastBreadcrumb(index)"
                    icon="material-symbols:chevron-right"
                    class="atom-glyph"
                    :style="{ '--atom-glyph-size': '14px' }"
                  />
                </template>
              </nav>
              <AppButton
                size="sm"
                class="flex-none"
                aria-label="选中当前目录作为目标"
                :disabled="moving"
                @click="emit('select-current')"
              >
                选中当前目录
              </AppButton>
            </div>

            <!-- 目录浏览器：48px 行，hover 底色 -->
            <div class="overflow-hidden border border-hairline">
              <div class="max-h-72 overflow-y-auto">
                <div v-if="browserLoading" class="px-4 py-3 text-note text-label-secondary">
                  正在加载目录...
                </div>
                <template v-else>
                  <template v-for="(folder, index) in folders" :key="`folder:${folder.id}`">
                    <button
                      class="flex min-h-12 w-full items-center justify-between px-4 text-left transition-colors hover:bg-surface-secondary disabled:cursor-not-allowed disabled:opacity-50"
                      type="button"
                      :disabled="isOwnFolder(folder) || moving"
                      @click="emit('open-folder', folder)"
                    >
                      <span class="flex min-w-0 items-center gap-3">
                        <span
                          class="flex h-8 w-8 flex-none items-center justify-center bg-warning-tint text-warning"
                        >
                          <AppIcon
                            class="atom-glyph"
                            icon="material-symbols:folder"
                            :style="{ '--atom-glyph-size': '16px' }"
                          />
                        </span>
                        <span class="truncate text-body text-label">{{ folder.name }}</span>
                      </span>
                      <AppIcon
                        class="atom-glyph text-label-tertiary"
                        icon="material-symbols:chevron-right"
                        :style="{ '--atom-glyph-size': '16px' }"
                      />
                    </button>
                    <div v-if="index < folders.length - 1" class="ml-15 h-px bg-hairline" />
                  </template>
                  <div v-if="folders.length === 0" class="px-4 py-3 text-note text-label-secondary">
                    当前目录下没有子文件夹
                  </div>
                </template>
              </div>
            </div>

            <p class="mt-3 text-note text-label-tertiary">
              已选目标目录 ID：
              <span class="font-semibold text-label">{{ targetFolderId }}</span>
            </p>

            <div class="mt-5 flex justify-end gap-3">
              <AppButton :disabled="moving" @click="handleCancel">取消</AppButton>
              <AppButton
                tone="primary"
                :disabled="moving"
                :loading="moving"
                @click="emit('confirm')"
              >
                {{ moving ? '移动中...' : '确认移动' }}
              </AppButton>
            </div>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
