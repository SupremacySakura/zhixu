<script setup lang="ts">
import { computed, ref } from 'vue'
import AppButton from '../ui/AppButton.vue'
import AppIcon from '../ui/AppIcon.vue'
import type { BreadcrumbItem } from '../../types/file'

const props = withDefaults(
  defineProps<{
    breadcrumbs: readonly BreadcrumbItem[]
    currentFolderName: string
    errorMessage?: string | null
    uploadTaskCount?: number
    completedUploadCount?: number
    isUploading?: boolean
    overallProgress?: number
    isUploadPanelOpen?: boolean
  }>(),
  {
    errorMessage: null,
    uploadTaskCount: 0,
    completedUploadCount: 0,
    isUploading: false,
    overallProgress: 0,
    isUploadPanelOpen: false,
  },
)

const emit = defineEmits<{
  navigate: [index: number]
  'create-folder': []
  'files-selected': [files: File[]]
  'folder-selected': [files: File[]]
  'toggle-upload-panel': []
}>()

const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)

const nestedBreadcrumbs = computed(() => props.breadcrumbs.slice(1))
const normalizedProgress = computed(() => Math.min(100, Math.max(0, props.overallProgress)))
const uploadStatusIcon = computed(() =>
  props.isUploading ? 'material-symbols:progress-activity' : 'material-symbols:check-circle',
)

const navigateToBreadcrumb = (index: number) => emit('navigate', index)
const isLastBreadcrumb = (nestedIndex: number) => nestedIndex === nestedBreadcrumbs.value.length - 1
const openFileDialog = () => fileInputRef.value?.click()
const openFolderDialog = () => folderInputRef.value?.click()

const takeFilesFromInput = (event: Event): File[] => {
  const input = event.currentTarget
  if (!(input instanceof HTMLInputElement) || !input.files?.length) return []
  const files = Array.from(input.files)
  input.value = ''
  return files
}

const handleFileInput = (event: Event) => {
  const files = takeFilesFromInput(event)
  if (files.length > 0) emit('files-selected', files)
}

const handleFolderInput = (event: Event) => {
  const files = takeFilesFromInput(event)
  if (files.length > 0) emit('folder-selected', files)
}

defineExpose({ openFileDialog, openFolderDialog })
</script>

<template>
  <!-- 页头带 64px：文件管理是索引列表原型。左侧是「当前目录名 + 目录路径」，
       右侧 1 主（上传）+ 2 次（新建文件夹 / 上传进度）。
       目录路径不是面包屑 —— 它是文件浏览器的导航控件，层级由用户的目录深度决定，
       不受「三层以上页面才留面包屑」那条规则约束。 -->
  <header class="shell-band">
    <div class="min-w-0">
      <h1 class="truncate text-title text-label">{{ currentFolderName }}</h1>
      <nav
        aria-label="目录路径"
        class="flex items-center gap-1.5 overflow-x-auto font-type text-micro text-label-tertiary"
      >
        <button
          class="flex items-center transition-colors hover:text-primary"
          type="button"
          aria-label="返回根目录"
          @click="navigateToBreadcrumb(0)"
        >
          <AppIcon
            class="mr-1 atom-glyph"
            icon="material-symbols:home"
            :style="{ '--atom-glyph-size': '14px' }"
          />
          root
        </button>
        <template v-for="(breadcrumb, index) in nestedBreadcrumbs" :key="breadcrumb.id">
          <AppIcon
            class="atom-glyph"
            icon="material-symbols:chevron-right"
            :style="{ '--atom-glyph-size': '14px' }"
          />
          <button
            v-if="!isLastBreadcrumb(index)"
            class="transition-colors hover:text-primary"
            type="button"
            :aria-label="`导航到 ${breadcrumb.name} 文件夹`"
            @click="navigateToBreadcrumb(index + 1)"
          >
            {{ breadcrumb.name }}
          </button>
          <span v-else class="font-medium text-label">{{ breadcrumb.name }}</span>
        </template>
      </nav>
      <p
        v-if="errorMessage"
        class="mt-0.5 flex items-center gap-1.5 text-note font-medium text-danger"
      >
        <AppIcon
          class="atom-glyph"
          icon="material-symbols:error-outline-rounded"
          :style="{ '--atom-glyph-size': '14px' }"
        />
        {{ errorMessage }}
      </p>
    </div>

    <div class="flex flex-none items-center gap-2 pointer-coarse:gap-3">
      <input ref="fileInputRef" type="file" class="hidden" multiple @change="handleFileInput" />
      <input
        ref="folderInputRef"
        type="file"
        class="hidden"
        webkitdirectory
        directory
        @change="handleFolderInput"
      />

      <!-- 375px 下三颗按钮 + 文字会顶破 64px 带子的右边界（实测 3 颗
           各 ~110px，加间距正好超宽）。窄屏只留图标，名字靠 aria-label
           与 title 保留；≥sm 再露出文字。 -->
      <AppButton
        icon="material-symbols:create-new-folder"
        aria-label="新建文件夹"
        title="新建文件夹"
        @click="emit('create-folder')"
      >
        <span class="hidden sm:inline">新建文件夹</span>
      </AppButton>

      <!-- 移动端：拆分为两个按钮 -->
      <div class="flex items-center gap-2 pointer-coarse:gap-3 sm:hidden">
        <AppButton
          tone="primary"
          icon="material-symbols:upload"
          aria-label="上传文件"
          title="上传文件"
          @click="openFileDialog"
        >
          <span class="hidden sm:inline">上传文件</span>
        </AppButton>
        <AppButton
          icon="material-symbols:folder"
          aria-label="上传文件夹"
          title="上传文件夹"
          @click="openFolderDialog"
        >
          <span class="hidden sm:inline">上传文件夹</span>
        </AppButton>
      </div>

      <!-- 桌面端：上传 split 按钮 + 悬停下拉（菜单从按钮 scale 0.95→1 生长） -->
      <div class="group relative hidden sm:block">
        <AppButton tone="primary" icon="material-symbols:upload" aria-label="上传文件或文件夹">
          上传
        </AppButton>
        <div
          class="invisible absolute right-0 top-full z-40 mt-1.5 w-44 bg-surface p-1.5 opacity-0 shadow-popover transition-[opacity,visibility] group-hover:visible group-hover:opacity-100"
        >
          <button
            class="flex h-9 w-full items-center gap-2.5 px-2.5 text-body text-label transition-colors hover:bg-surface-secondary"
            type="button"
            aria-label="上传文件"
            @click="openFileDialog"
          >
            <AppIcon
              class="atom-glyph text-label-secondary"
              icon="material-symbols:description"
              :style="{ '--atom-glyph-size': '16px' }"
            />
            上传文件
          </button>
          <button
            class="flex h-9 w-full items-center gap-2.5 px-2.5 text-body text-label transition-colors hover:bg-surface-secondary"
            type="button"
            aria-label="上传文件夹"
            @click="openFolderDialog"
          >
            <AppIcon
              class="atom-glyph text-label-secondary"
              icon="material-symbols:folder"
              :style="{ '--atom-glyph-size': '16px' }"
            />
            上传文件夹
          </button>
        </div>
      </div>

      <!-- 上传进度入口 -->
      <AppButton
        v-if="uploadTaskCount > 0"
        :icon="isUploading ? undefined : uploadStatusIcon"
        :loading="isUploading"
        :class="isUploadPanelOpen ? 'bg-surface-secondary' : ''"
        aria-label="查看上传进度"
        @click="emit('toggle-upload-panel')"
      >
        <span v-if="isUploading">{{ normalizedProgress }}%</span>
        <span v-else>{{ completedUploadCount }}/{{ uploadTaskCount }}</span>
      </AppButton>
    </div>
  </header>
</template>
