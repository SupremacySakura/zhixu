<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import type { FileDisplayItem, FileItemKey } from '../../types/file'
import { formatBytes } from '../../utils/file'
import { ownerInitials } from '../../utils/file-management'
import FileEmptyState from './FileEmptyState.vue'
import FileKnowledgeAction from '../knowledge/FileKnowledgeAction.vue'
import FileSelectBox from './FileSelectBox.vue'
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
const handleOpenMenu = (file: FileDisplayItem, event: MouseEvent) => {
  const anchor = event.currentTarget
  if (anchor instanceof HTMLElement) emit('open-menu', file, anchor)
}

// 列网格：选择 / 图标 / 名称 / 类型 / 大小 / 所有者 / 修改时间 / 操作
const rowGridClass =
  'grid min-h-12 items-center gap-3 px-4 ' +
  'grid-cols-[28px_40px_minmax(140px,1fr)_72px_88px_92px_140px_36px]'
</script>

<template>
  <div class="overflow-hidden bg-surface shadow-card">
    <!-- ≥lg：八列行网格。这一行在 375px 下要 760px 才装得下，只能靠横向滚动，
         而 DESIGN.md:514 明确「表格变两列卡列表，不做横向滚动」——
         所以表格只在 lg 及以上出现，窄屏换成下面的卡片列表。 -->
    <div class="hidden lg:block">
      <!-- 表头 -->
      <div :class="[rowGridClass, 'min-h-10 border-b border-hairline bg-surface-secondary']">
        <FileSelectBox
          label="选择当前页全部文件"
          :checked="allSelected"
          @change="checked => emit('toggle-all', checked)"
        />
        <span></span>
        <span class="text-micro font-normal uppercase text-label-tertiary">名称</span>
        <span class="text-micro font-normal uppercase text-label-tertiary">类型</span>
        <span class="text-micro font-normal uppercase text-label-tertiary">大小</span>
        <span class="text-micro font-normal uppercase text-label-tertiary">所有者</span>
        <span class="text-micro font-normal uppercase text-label-tertiary">最近修改</span>
        <span class="text-right text-micro font-normal uppercase text-label-tertiary">操作</span>
      </div>

      <!-- 「..」返回上一级目录行 -->
      <div
        v-if="hasParentFolder"
        :class="[rowGridClass, 'cursor-pointer transition-colors hover:bg-surface-secondary']"
        role="button"
        tabindex="0"
        @click="emit('go-parent')"
        @keyup.enter.self="emit('go-parent')"
      >
        <span></span>
        <span class="flex h-10 w-10 items-center justify-center bg-warning-tint text-warning">
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:folder"
            :style="{ '--atom-glyph-size': '20px' }"
          />
        </span>
        <span class="truncate">
          <span class="text-lead text-label">..</span>
          <span class="text-note text-label-secondary">　返回上一级目录</span>
        </span>
        <span class="text-note text-label-secondary">文件夹</span>
        <span class="text-note text-label-secondary">-</span>
        <span class="text-note text-label-secondary">-</span>
        <span class="text-note text-label-secondary">-</span>
        <span></span>
      </div>
      <div v-if="hasParentFolder" class="ml-[108px] h-px bg-hairline" />

      <!-- 文件行：分隔线自图标后内缩（16 + 28 + 12 + 40 + 12 = 108） -->
      <template v-for="(file, index) in files" :key="file.key">
        <div
          :class="[
            rowGridClass,
            'transition-colors',
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
          <FileSelectBox
            :label="`选择 ${file.name}`"
            :checked="selectedIds.has(file.key)"
            @change="checked => emit('toggle-select', file, checked)"
          />
          <span
            class="flex h-10 w-10 items-center justify-center"
            :class="`${file.iconBg} ${file.iconFg}`"
          >
            <AppIcon
              class="atom-glyph"
              :icon="file.icon"
              :style="{ '--atom-glyph-size': '20px' }"
            />
          </span>
          <div class="min-w-0 py-2">
            <p class="truncate text-lead text-label">{{ file.name }}</p>
            <FileKnowledgeAction
              :file="file"
              :revision="knowledgeRevision"
              @action="(target, item) => emit('knowledge', target, item)"
            />
          </div>
          <span class="truncate text-note text-label-secondary">{{ file.typeLabel }}</span>
          <span class="truncate text-note text-label-secondary">
            {{ file.type === 'folder' ? '-' : formatBytes(file.size) }}
          </span>
          <span class="flex min-w-0 items-center gap-2">
            <span
              class="flex h-6 w-6 flex-none items-center justify-center bg-surface-tertiary text-note font-bold tracking-[0.02em] text-label-secondary"
            >
              {{ ownerBadge }}
            </span>
            <span class="truncate text-note text-label-secondary">{{ ownerName }}</span>
          </span>
          <span class="whitespace-nowrap text-note text-label-secondary">
            {{ file.lastModifiedText }}
          </span>
          <button
            class="flex h-9 w-9 items-center justify-center justify-self-end text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
            type="button"
            :aria-label="`${file.name} 操作菜单`"
            @click.stop="handleOpenMenu(file, $event)"
          >
            <AppIcon icon="material-symbols:more-vert" aria-hidden="true" />
          </button>
        </div>
        <div v-if="index < files.length - 1" class="ml-[108px] h-px bg-hairline" />
      </template>
    </div>

    <!-- <lg：卡片列表，零横向滚动。行网格的八列在窄屏塞不下，
         折成「图标 + 名称 + 一行摘要 + 选择框/菜单」，一件事都不丢。 -->
    <div v-if="!loading && files.length > 0" class="divide-y divide-hairline lg:hidden">
      <div
        v-if="hasParentFolder"
        class="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-secondary"
        role="button"
        tabindex="0"
        @click="emit('go-parent')"
        @keyup.enter.self="emit('go-parent')"
      >
        <span
          class="flex h-10 w-10 flex-none items-center justify-center bg-warning-tint text-warning"
        >
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:folder"
            :style="{ '--atom-glyph-size': '20px' }"
          />
        </span>
        <span class="min-w-0">
          <span class="block truncate text-lead text-label">..</span>
          <span class="block truncate text-note text-label-secondary">返回上一级目录</span>
        </span>
      </div>

      <div
        v-for="file in files"
        :key="file.key"
        class="flex items-start gap-3 px-4 py-3 transition-colors"
        :class="[
          file.type === 'folder' ? 'cursor-pointer' : '',
          selectedIds.has(file.key) ? 'bg-primary-tint' : 'hover:bg-surface-secondary',
        ]"
        :role="file.type === 'folder' ? 'button' : undefined"
        :tabindex="file.type === 'folder' ? 0 : undefined"
        @click="handleItemClick(file)"
        @keyup.enter.self="handleItemClick(file)"
      >
        <span
          class="flex h-10 w-10 flex-none items-center justify-center"
          :class="`${file.iconBg} ${file.iconFg}`"
        >
          <AppIcon class="atom-glyph" :icon="file.icon" :style="{ '--atom-glyph-size': '20px' }" />
        </span>
        <div class="min-w-0 flex-1">
          <p class="truncate text-lead text-label">{{ file.name }}</p>
          <p class="mt-0.5 truncate text-note text-label-secondary">
            {{ file.typeLabel }} · {{ file.type === 'folder' ? '-' : formatBytes(file.size) }} ·
            {{ file.lastModifiedText }}
          </p>
          <FileKnowledgeAction
            :file="file"
            :revision="knowledgeRevision"
            @action="(target, item) => emit('knowledge', target, item)"
          />
        </div>
        <div class="flex flex-none items-center gap-1 pointer-coarse:gap-3">
          <FileSelectBox
            :label="`选择 ${file.name}`"
            :checked="selectedIds.has(file.key)"
            @change="checked => emit('toggle-select', file, checked)"
          />
          <button
            class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
            type="button"
            :aria-label="`${file.name} 操作菜单`"
            @click.stop="handleOpenMenu(file, $event)"
          >
            <AppIcon icon="material-symbols:more-vert" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="flex flex-col items-center justify-center gap-2.5 py-11">
      <AppLoading :size="28" class="text-primary" />
      <p class="text-note text-label-tertiary">加载中...</p>
    </div>

    <!-- 空状态 -->
    <div v-if="files.length === 0 && !loading" class="px-6 py-20">
      <FileEmptyState :query="query" @upload="emit('upload')" />
    </div>

    <slot name="footer" />
  </div>
</template>
