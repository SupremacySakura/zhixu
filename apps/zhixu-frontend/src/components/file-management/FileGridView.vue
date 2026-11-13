<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import type { FileDisplayItem, FileItemKey } from '../../types/file'
import { formatBytes } from '../../utils/file'
import { ownerInitials } from '../../utils/file-management'
import FileEmptyState from './FileEmptyState.vue'
import FileKnowledgeAction from '../knowledge/FileKnowledgeAction.vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'

const props = withDefaults(
  defineProps<{
    files: readonly FileDisplayItem[]
    loading: boolean
    hasParentFolder: boolean
    selectedIds: ReadonlySet<FileItemKey>
    query?: string
    ownerName?: string
    knowledgeRevision?: number
  }>(),
  {
    query: '',
    ownerName: 'Me',
  },
)

const emit = defineEmits<{
  'go-parent': []
  'open-item': [file: FileDisplayItem]
  'toggle-select': [file: FileDisplayItem, checked: boolean]
  'open-menu': [file: FileDisplayItem, anchor: HTMLElement]
  upload: []
  knowledge: [file: FileDisplayItem, item: KnowledgeItem | null]
}>()

const ownerBadge = computed(() => ownerInitials(props.ownerName))

const handleItemClick = (file: FileDisplayItem) => emit('open-item', file)
const handleToggleSelect = (file: FileDisplayItem, event: Event) => {
  const input = event.currentTarget
  if (input instanceof HTMLInputElement) emit('toggle-select', file, input.checked)
}
const handleOpenMenu = (file: FileDisplayItem, event: MouseEvent) => {
  const anchor = event.currentTarget
  if (anchor instanceof HTMLElement) emit('open-menu', file, anchor)
}

const isSelected = (key: FileItemKey) => props.selectedIds.has(key)

const cardClass = (key: FileItemKey) =>
  [
    'relative cursor-pointer select-none bg-surface p-4 shadow-card transition-[transform,box-shadow] hover:-translate-y-0.5 hover:shadow-popover',
    isSelected(key) ? 'ring-2 ring-primary' : '',
  ].join(' ')
</script>

<template>
  <div>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <!-- 「..」返回上一级目录卡 -->
      <div
        v-if="hasParentFolder"
        class="relative cursor-pointer select-none bg-surface p-4 shadow-card transition-[transform,box-shadow] hover:-translate-y-0.5 hover:shadow-popover"
        role="button"
        tabindex="0"
        @click="emit('go-parent')"
        @keyup.enter.self="emit('go-parent')"
      >
        <div class="flex items-start gap-3">
          <span
            class="flex h-11 w-11 flex-none items-center justify-center bg-warning-tint text-warning"
          >
            <AppIcon
              class="atom-glyph"
              icon="material-symbols:folder"
              :style="{ '--atom-glyph-size': '22px' }"
            />
          </span>
          <div class="min-w-0">
            <p class="truncate text-lead font-semibold leading-5 text-label">..</p>
            <p class="mt-0.5 text-body text-label-secondary">返回上一级目录</p>
          </div>
        </div>
      </div>

      <div
        v-for="file in files"
        :key="file.key"
        :class="cardClass(file.key)"
        :role="file.type === 'folder' ? 'button' : undefined"
        :tabindex="file.type === 'folder' ? 0 : undefined"
        @click="handleItemClick(file)"
        @keyup.enter.self="handleItemClick(file)"
      >
        <div class="flex items-start gap-3">
          <span
            class="flex h-11 w-11 flex-none items-center justify-center"
            :class="`${file.iconBg} ${file.iconFg}`"
          >
            <AppIcon
              class="atom-glyph"
              :icon="file.icon"
              :style="{ '--atom-glyph-size': '22px' }"
            />
          </span>
          <div class="min-w-0 flex-1">
            <p class="line-clamp-2 text-lead font-semibold leading-5 text-label">
              {{ file.name }}
            </p>
            <p class="mt-0.5 truncate text-body text-label-secondary">{{ file.typeLabel }}</p>
          </div>
          <label class="inline-flex flex-none cursor-pointer pt-0.5" @click.stop>
            <input
              type="checkbox"
              class="peer sr-only"
              :aria-label="`选择 ${file.name}`"
              :checked="isSelected(file.key)"
              @change="handleToggleSelect(file, $event)"
            />
            <span
              class="flex h-[18px] w-[18px] items-center justify-center border-[1.5px] border-label-tertiary bg-surface transition-[background-color,border-color] hover:border-primary peer-checked:border-primary peer-checked:bg-primary peer-focus-visible:ring-2 peer-focus-visible:ring-primary/50"
            >
              <svg
                v-if="isSelected(file.key)"
                class="h-[11px] w-[11px]"
                viewBox="0 0 24 24"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="m5 12.5 5 5L19 8"
                  stroke="#fff"
                  stroke-width="3"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </span>
          </label>
        </div>

        <div
          class="mt-3.5 flex items-center justify-between gap-2 text-caption text-label-secondary"
        >
          <span>{{ file.type === 'folder' ? '-' : formatBytes(file.size) }}</span>
          <span class="whitespace-nowrap">{{ file.lastModifiedText }}</span>
        </div>

        <FileKnowledgeAction
          class="mt-3"
          :file="file"
          :revision="knowledgeRevision"
          @action="(target, item) => emit('knowledge', target, item)"
        />
        <div class="mt-3 flex items-center justify-between gap-2">
          <div class="flex min-w-0 items-center gap-2">
            <span
              class="flex h-6 w-6 flex-none items-center justify-center bg-surface-tertiary text-note font-bold tracking-[0.02em] text-label-secondary"
            >
              {{ ownerBadge }}
            </span>
            <span class="truncate text-caption text-label-secondary">{{ ownerName }}</span>
          </div>
          <button
            class="flex h-8 w-8 flex-none items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
            type="button"
            :aria-label="`${file.name} 操作菜单`"
            @click.stop="handleOpenMenu(file, $event)"
          >
            <AppIcon
              class="atom-glyph"
              icon="material-symbols:more-vert"
              :style="{ '--atom-glyph-size': '18px' }"
            />
          </button>
        </div>
      </div>

      <div v-if="loading" class="col-span-full py-20">
        <div class="flex flex-col items-center justify-center text-center">
          <AppLoading :size="28" class="mb-3 text-primary" />
          <p class="text-subhead text-label-tertiary">加载中...</p>
        </div>
      </div>
      <div v-else-if="files.length === 0" class="col-span-full py-20">
        <FileEmptyState :query="query" @upload="emit('upload')" />
      </div>
    </div>
    <slot name="footer" />
  </div>
</template>
