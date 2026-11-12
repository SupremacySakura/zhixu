<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
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
    allSelected: boolean
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
  'toggle-all': [checked: boolean]
  'toggle-select': [file: FileDisplayItem, checked: boolean]
  'open-menu': [file: FileDisplayItem, anchor: HTMLElement]
  upload: []
  knowledge: [file: FileDisplayItem, item: KnowledgeItem | null]
}>()

const ownerBadge = computed(() => ownerInitials(props.ownerName))

const handleItemClick = (file: FileDisplayItem) => emit('open-item', file)
const handleToggleAll = (event: Event) => {
  const input = event.currentTarget
  if (input instanceof HTMLInputElement) emit('toggle-all', input.checked)
}
const handleToggleSelect = (file: FileDisplayItem, event: Event) => {
  const input = event.currentTarget
  if (input instanceof HTMLInputElement) emit('toggle-select', file, input.checked)
}
const handleOpenMenu = (file: FileDisplayItem, event: MouseEvent) => {
  const anchor = event.currentTarget
  if (anchor instanceof HTMLElement) emit('open-menu', file, anchor)
}

// 列网格（FileListView.html）：选择 / 图标 / 名称 / 类型 / 大小 / 所有者 / 修改时间 / 操作
const rowGridClass =
  'grid min-h-12 items-center gap-3 px-4 ' +
  'grid-cols-[28px_40px_minmax(140px,1fr)_72px_88px_92px_140px_36px] min-w-[760px]'
</script>

<template>
  <div class="overflow-hidden rounded-md bg-surface shadow-card">
    <div class="overflow-x-auto">
      <!-- 表头 -->
      <div :class="[rowGridClass, 'min-h-10 border-b border-hairline bg-surface-secondary']">
        <label class="inline-flex cursor-pointer" @click.stop>
          <input
            type="checkbox"
            class="peer sr-only"
            aria-label="选择当前页全部文件"
            :checked="allSelected"
            @change="handleToggleAll"
          />
          <span
            class="flex h-[18px] w-[18px] items-center justify-center rounded-md border-[1.5px] border-label-tertiary bg-surface transition-all duration-100 hover:border-primary peer-checked:border-primary peer-checked:bg-primary peer-focus-visible:ring-2 peer-focus-visible:ring-primary/50 active:scale-[0.88]"
          >
            <svg
              v-if="allSelected"
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
        <span></span>
        <span class="text-caption font-semibold tracking-[0.06em] text-label-secondary">名称</span>
        <span class="text-caption font-semibold tracking-[0.06em] text-label-secondary">类型</span>
        <span class="text-caption font-semibold tracking-[0.06em] text-label-secondary">大小</span>
        <span class="text-caption font-semibold tracking-[0.06em] text-label-secondary"
          >所有者</span
        >
        <span class="text-caption font-semibold tracking-[0.06em] text-label-secondary"
          >最近修改</span
        >
        <span class="text-caption text-right font-semibold tracking-[0.06em] text-label-secondary">
          操作
        </span>
      </div>

      <!-- 「..」返回上一级目录行 -->
      <div
        v-if="hasParentFolder"
        :class="[
          rowGridClass,
          'cursor-pointer transition-colors duration-150 hover:bg-surface-secondary',
        ]"
        role="button"
        tabindex="0"
        @click="emit('go-parent')"
        @keyup.enter.self="emit('go-parent')"
      >
        <span></span>
        <span
          class="flex h-10 w-10 items-center justify-center rounded-sm bg-warning-tint text-warning"
        >
          <AppIcon class="text-[20px]" icon="material-symbols:folder" />
        </span>
        <span class="truncate">
          <span class="text-[15px] font-semibold text-label">..</span>
          <span class="text-subhead text-label-secondary">　返回上一级目录</span>
        </span>
        <span class="text-subhead text-label-secondary">文件夹</span>
        <span class="text-subhead text-label-secondary">-</span>
        <span class="text-subhead text-label-secondary">-</span>
        <span class="text-subhead text-label-secondary">-</span>
        <span></span>
      </div>
      <div v-if="hasParentFolder" class="ml-[108px] h-px bg-hairline" />

      <!-- 文件行：分隔线自图标后内缩（16 + 28 + 12 + 40 + 12 = 108） -->
      <template v-for="(file, index) in files" :key="file.key">
        <div
          :class="[
            rowGridClass,
            'transition-colors duration-150',
            file.type === 'folder' ? 'cursor-pointer' : '',
            selectedIds.has(file.key)
              ? 'bg-primary-tint hover:bg-primary-tint'
              : 'hover:bg-surface-secondary',
          ]"
          :role="file.type === 'folder' ? 'button' : undefined"
          :tabindex="file.type === 'folder' ? 0 : undefined"
          @click="handleItemClick(file)"
          @keyup.enter.self="handleItemClick(file)"
        >
          <label class="inline-flex cursor-pointer" @click.stop>
            <input
              type="checkbox"
              class="peer sr-only"
              :aria-label="`选择 ${file.name}`"
              :checked="selectedIds.has(file.key)"
              @change="handleToggleSelect(file, $event)"
            />
            <span
              class="flex h-[18px] w-[18px] items-center justify-center rounded-md border-[1.5px] border-label-tertiary bg-surface transition-all duration-100 hover:border-primary peer-checked:border-primary peer-checked:bg-primary peer-focus-visible:ring-2 peer-focus-visible:ring-primary/50 active:scale-[0.88]"
            >
              <svg
                v-if="selectedIds.has(file.key)"
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
          <span
            class="flex h-10 w-10 items-center justify-center rounded-sm"
            :class="`${file.iconBg} ${file.iconFg}`"
          >
            <AppIcon class="text-[20px]" :icon="file.icon" />
          </span>
          <div class="min-w-0 py-2">
            <p class="truncate text-[15px] font-semibold text-label">{{ file.name }}</p>
            <FileKnowledgeAction
              :file="file"
              :revision="knowledgeRevision"
              @action="(target, item) => emit('knowledge', target, item)"
            />
          </div>
          <span class="truncate text-subhead text-label-secondary">{{ file.typeLabel }}</span>
          <span class="truncate text-subhead text-label-secondary">
            {{ file.type === 'folder' ? '-' : formatBytes(file.size) }}
          </span>
          <span class="flex min-w-0 items-center gap-2">
            <span
              class="flex h-6 w-6 flex-none items-center justify-center rounded-full bg-surface-tertiary text-[10px] font-bold tracking-[0.02em] text-label-secondary"
            >
              {{ ownerBadge }}
            </span>
            <span class="truncate text-subhead text-label-secondary">{{ ownerName }}</span>
          </span>
          <span class="whitespace-nowrap text-subhead text-label-secondary">
            {{ file.lastModifiedText }}
          </span>
          <button
            class="flex h-9 w-9 items-center justify-center justify-self-end rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-label"
            type="button"
            :aria-label="`${file.name} 操作菜单`"
            @click.stop="handleOpenMenu(file, $event)"
          >
            <AppIcon icon="material-symbols:more-vert" aria-hidden="true" />
          </button>
        </div>
        <div v-if="index < files.length - 1" class="ml-[108px] h-px bg-hairline" />
      </template>

      <!-- 加载中 -->
      <div v-if="loading" class="flex flex-col items-center justify-center gap-2.5 py-11">
        <AppIcon
          icon="material-symbols:progress-activity"
          class="animate-spin text-[28px] text-primary"
        />
        <p class="text-subhead text-label-tertiary">加载中...</p>
      </div>

      <!-- 空状态 -->
      <div v-if="files.length === 0 && !loading" class="px-6 py-20">
        <FileEmptyState :query="query" @upload="emit('upload')" />
      </div>
    </div>
    <slot name="footer" />
  </div>
</template>
