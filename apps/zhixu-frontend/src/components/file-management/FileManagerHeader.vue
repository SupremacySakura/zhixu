<script setup lang="ts">
import { computed, ref } from 'vue'
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

const secondaryButtonClass =
  'flex h-10 items-center gap-1.5 rounded-full bg-surface px-4 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary hover:text-label active:scale-[0.97]'
</script>

<template>
  <div class="mb-6 flex flex-col gap-4 lg:mb-8 lg:flex-row lg:items-center lg:justify-between">
    <div>
      <nav
        class="mb-1.5 flex items-center gap-1.5 overflow-x-auto text-subhead text-label-tertiary"
      >
        <button
          class="flex items-center rounded-sm transition-colors duration-150 hover:text-primary"
          type="button"
          aria-label="返回根目录"
          @click="navigateToBreadcrumb(0)"
        >
          <AppIcon class="mr-1 text-[14px]" icon="material-symbols:home" />
          root
        </button>
        <template v-for="(breadcrumb, index) in nestedBreadcrumbs" :key="breadcrumb.id">
          <AppIcon class="text-[14px]" icon="material-symbols:chevron-right" />
          <button
            v-if="!isLastBreadcrumb(index)"
            class="rounded-sm transition-colors duration-150 hover:text-primary"
            type="button"
            :aria-label="`导航到 ${breadcrumb.name} 文件夹`"
            @click="navigateToBreadcrumb(index + 1)"
          >
            {{ breadcrumb.name }}
          </button>
          <span v-else class="font-medium text-label">{{ breadcrumb.name }}</span>
        </template>
      </nav>
      <h2 class="text-title-2 text-label">{{ currentFolderName }}</h2>
      <p
        v-if="errorMessage"
        class="mt-2 flex items-center gap-1.5 text-[13px] font-medium text-danger"
      >
        <AppIcon class="text-[14px]" icon="material-symbols:error-outline-rounded" />
        {{ errorMessage }}
      </p>
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <input ref="fileInputRef" type="file" class="hidden" multiple @change="handleFileInput" />
      <input
        ref="folderInputRef"
        type="file"
        class="hidden"
        webkitdirectory
        directory
        @change="handleFolderInput"
      />

      <button
        :class="secondaryButtonClass"
        type="button"
        aria-label="新建文件夹"
        @click="emit('create-folder')"
      >
        <AppIcon class="text-[18px]" icon="material-symbols:create-new-folder" />
        新建文件夹
      </button>

      <!-- 移动端：拆分为两个按钮 -->
      <div class="flex flex-wrap items-center gap-3 sm:hidden">
        <button
          class="flex h-10 items-center gap-1.5 rounded-full bg-primary px-4 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
          type="button"
          aria-label="上传文件"
          @click="openFileDialog"
        >
          <AppIcon class="text-[18px]" icon="material-symbols:upload" />
          上传文件
        </button>
        <button
          :class="secondaryButtonClass"
          type="button"
          aria-label="上传文件夹"
          @click="openFolderDialog"
        >
          <AppIcon class="text-[18px]" icon="material-symbols:folder" />
          上传文件夹
        </button>
      </div>

      <!-- 桌面端：上传 split 按钮 + 悬停下拉（菜单从按钮 scale 0.95→1 生长） -->
      <div class="group relative hidden sm:block">
        <button
          class="flex h-10 items-center gap-1.5 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
          type="button"
          aria-label="上传文件或文件夹"
        >
          <AppIcon class="text-[18px]" icon="material-symbols:upload" />
          上传
        </button>
        <div
          class="invisible absolute right-0 top-full z-40 mt-1.5 w-44 origin-top-right scale-[0.95] rounded-md bg-surface p-1.5 opacity-0 shadow-popover transition-all duration-150 ease-out group-hover:visible group-hover:scale-100 group-hover:opacity-100"
        >
          <button
            class="flex h-9 w-full items-center gap-2.5 rounded-sm px-2.5 text-sm text-label transition-colors duration-150 hover:bg-surface-secondary"
            type="button"
            aria-label="上传文件"
            @click="openFileDialog"
          >
            <AppIcon class="text-[16px] text-label-secondary" icon="material-symbols:description" />
            上传文件
          </button>
          <button
            class="flex h-9 w-full items-center gap-2.5 rounded-sm px-2.5 text-sm text-label transition-colors duration-150 hover:bg-surface-secondary"
            type="button"
            aria-label="上传文件夹"
            @click="openFolderDialog"
          >
            <AppIcon class="text-[16px] text-label-secondary" icon="material-symbols:folder" />
            上传文件夹
          </button>
        </div>
      </div>

      <!-- 上传进度入口 -->
      <button
        v-if="uploadTaskCount > 0"
        :class="[secondaryButtonClass, isUploadPanelOpen ? 'bg-surface-secondary' : '']"
        type="button"
        aria-label="查看上传进度"
        @click="emit('toggle-upload-panel')"
      >
        <AppIcon
          class="text-[18px]"
          :icon="uploadStatusIcon"
          :class="isUploading ? 'animate-spin text-primary' : 'text-primary'"
        />
        <span v-if="isUploading">{{ normalizedProgress }}%</span>
        <span v-else>{{ completedUploadCount }}/{{ uploadTaskCount }}</span>
      </button>
    </div>
  </div>
</template>
