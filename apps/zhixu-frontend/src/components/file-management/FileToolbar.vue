<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
import type { FileFilterKey, FileSortKey, FileViewMode, SortDirection } from '../../types/file'
import FilterMenu from '../ui/FilterMenu.vue'

export type FileFilterOption = {
  key: FileFilterKey
  label: string
}

const props = defineProps<{
  searchQuery: string
  viewMode: FileViewMode
  filterOptions: readonly FileFilterOption[]
  activeFilter: FileFilterKey
  selectedCount: number
  sortKey: FileSortKey
  sortDirection: SortDirection
}>()

const emit = defineEmits<{
  'update:searchQuery': [value: string]
  'update:viewMode': [value: FileViewMode]
  'clear-search': []
  'select-filter': [key: FileFilterKey]
  'select-sort': [key: FileSortKey]
  'select-current-page': []
  'clear-selection': []
}>()

const rootRef = ref<HTMLElement | null>(null)
const isSortOpen = ref(false)

const sortLabel = computed(() => {
  if (props.sortKey === 'name') return '名称'
  if (props.sortKey === 'size') return '大小'
  return '最近修改'
})

const closeMenus = () => {
  isSortOpen.value = false
}

const handleDocumentClick = (event: MouseEvent) => {
  const target = event.target
  if (target instanceof Node && rootRef.value && !rootRef.value.contains(target)) closeMenus()
}

const handleSearchInput = (event: Event) => {
  const input = event.currentTarget
  if (input instanceof HTMLInputElement) emit('update:searchQuery', input.value)
}

const clearSearch = () => {
  emit('update:searchQuery', '')
  emit('clear-search')
}

const selectViewMode = (viewMode: FileViewMode) => emit('update:viewMode', viewMode)
const toggleSortMenu = () => {
  isSortOpen.value = !isSortOpen.value
}
const selectFilter = (key: FileFilterKey) => {
  emit('select-filter', key)
}
const selectSort = (key: FileSortKey) => {
  emit('select-sort', key)
  isSortOpen.value = false
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', handleDocumentClick))

const menuItemClass =
  'flex h-9 w-full items-center justify-between gap-[18px] px-2.5 text-left text-sm text-label transition-colors hover:bg-surface-secondary'
const segButtonClass =
  'flex h-7 items-center justify-center px-[11px] text-label-secondary transition-colors tap-target'
</script>

<template>
  <!-- 粘性材质工具栏：内容滚动时从底下穿过（FileToolbar.html） -->
  <div
    ref="rootRef"
    class="sticky top-3 z-toolbar mb-6 flex flex-wrap items-center justify-between gap-x-4 gap-y-2.5 border border-hairline bg-surface-secondary px-3 py-2 shadow-card"
  >
    <div class="flex w-full flex-wrap items-center gap-2.5 lg:w-auto">
      <!-- 胶囊搜索 -->
      <div class="relative w-full sm:w-[232px]">
        <AppIcon
          class="pointer-events-none absolute left-[11px] top-1/2 -translate-y-1/2 atom-glyph text-label-tertiary"
          icon="material-symbols:search"
          :style="{ '--atom-glyph-size': '16px' }"
        />
        <input
          :value="searchQuery"
          type="text"
          placeholder="搜索文件..."
          aria-label="搜索文件"
          class="h-[34px] w-full border-none bg-surface-secondary pl-[33px] pr-[30px] text-sm text-label outline-none transition-[background-color,box-shadow] placeholder:text-label-tertiary focus:bg-surface focus:shadow-[inset_0_0_0_1.5px_var(--color-primary),0_0_0_3px_var(--color-primary-tint)]"
          @input="handleSearchInput"
        />
        <button
          v-if="searchQuery"
          class="absolute right-[3px] top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center bg-surface-tertiary text-label-secondary transition-colors tap-target-anchored hover:text-label"
          type="button"
          aria-label="清空搜索"
          @click="clearSearch"
        >
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:close"
            :style="{ '--atom-glyph-size': '13px' }"
          />
        </button>
      </div>

      <span class="hidden h-5 w-px bg-hairline sm:block"></span>

      <!-- 视图分段控件 -->
      <div
        class="flex gap-0.5 bg-surface-tertiary p-0.5 pointer-coarse:gap-3"
        role="group"
        aria-label="视图切换"
      >
        <button
          :class="[
            segButtonClass,
            viewMode === 'list' ? 'bg-surface text-label' : 'hover:text-label',
          ]"
          aria-label="列表视图"
          type="button"
          @click="selectViewMode('list')"
        >
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:list"
            aria-hidden="true"
            :style="{ '--atom-glyph-size': '17px' }"
          />
        </button>
        <button
          :class="[
            segButtonClass,
            viewMode === 'grid' ? 'bg-surface text-label' : 'hover:text-label',
          ]"
          aria-label="网格视图"
          type="button"
          @click="selectViewMode('grid')"
        >
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:grid-view"
            aria-hidden="true"
            :style="{ '--atom-glyph-size': '17px' }"
          />
        </button>
      </div>

      <span class="hidden h-5 w-px bg-hairline sm:block"></span>

      <!-- 筛选 chip + 菜单（从触发源弹簧生长） -->
      <FilterMenu
        :model-value="activeFilter"
        :options="filterOptions"
        label="筛选文件"
        @update:model-value="selectFilter($event as FileFilterKey)"
      />

      <!-- 多选条 -->
      <div
        v-if="selectedCount > 0"
        class="ml-1 flex items-center gap-1.5 text-body text-label-secondary"
      >
        <span>已选择 {{ selectedCount }} 项</span>
        <button
          class="px-2 py-0.5 text-body font-semibold text-primary transition-colors hover:bg-primary-tint"
          type="button"
          @click="emit('select-current-page')"
        >
          选择当前页
        </button>
        <button
          class="px-2 py-0.5 text-body font-semibold text-primary transition-colors hover:bg-primary-tint"
          type="button"
          @click="emit('clear-selection')"
        >
          清空
        </button>
      </div>
    </div>

    <!-- 排序 -->
    <div class="relative w-full sm:w-auto">
      <div class="flex items-center gap-1.5 pr-1 sm:justify-end">
        <span class="text-caption text-label-tertiary">排序方式</span>
        <button
          class="flex h-8 items-center gap-1 px-2 text-body font-semibold text-label transition-colors hover:bg-primary-tint hover:text-primary"
          type="button"
          :aria-expanded="isSortOpen"
          @click="toggleSortMenu"
        >
          {{ sortLabel }}
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:expand-more"
            :style="{ '--atom-glyph-size': '14px' }"
          />
        </button>
      </div>

      <AnimatePresence>
        <Motion
          v-if="isSortOpen"
          key="sort-menu"
          :initial="{ opacity: 0, scale: 0.95 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.95 }"
          :transition="springSnappy"
          class="absolute right-0 z-40 mt-1.5 min-w-[190px] origin-top-right bg-surface p-1.5 shadow-popover"
        >
          <button :class="menuItemClass" type="button" @click="selectSort('name')">
            <span>名称</span>
            <AppIcon
              v-if="sortKey === 'name'"
              icon="material-symbols:check-rounded"
              class="atom-glyph text-primary"
              :style="{ '--atom-glyph-size': '15px' }"
            />
          </button>
          <button :class="menuItemClass" type="button" @click="selectSort('modified')">
            <span>最近修改</span>
            <AppIcon
              v-if="sortKey === 'modified'"
              icon="material-symbols:check-rounded"
              class="atom-glyph text-primary"
              :style="{ '--atom-glyph-size': '15px' }"
            />
          </button>
          <button :class="menuItemClass" type="button" @click="selectSort('size')">
            <span>大小</span>
            <AppIcon
              v-if="sortKey === 'size'"
              icon="material-symbols:check-rounded"
              class="atom-glyph text-primary"
              :style="{ '--atom-glyph-size': '15px' }"
            />
          </button>
          <div class="mx-2.5 my-1 h-px bg-hairline"></div>
          <div class="px-2.5 pb-1 pt-1.5 text-caption text-label-tertiary">
            {{ sortDirection === 'asc' ? '升序' : '降序' }}
          </div>
        </Motion>
      </AnimatePresence>
    </div>
  </div>
</template>
