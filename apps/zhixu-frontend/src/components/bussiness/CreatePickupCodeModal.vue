<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import { createPickupCode, getListByFolderIDAndUserID } from '../../services/apis/file'
import type { PickupCodeType } from '../../services/types/file'
import type { FileListItem } from '../../services/types/file'

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

const form = ref({
  file_id: null as number | null,
  folder_id: null as number | null,
  type: 'file' as PickupCodeType,
  max_downloads: 10,
  expire_days: 7,
})

const fileList = ref<FileListItem[]>([])
const loading = ref(false)
const submitting = ref(false)
const selectedItemId = ref<number | null>(null)
const currentFolderId = ref(0)
const pathStack = ref<{ id: number; name: string }[]>([{ id: 0, name: '根目录' }])
const toast = ref<{ type: 'success' | 'error'; text: string } | null>(null)

const minDownloads = 1
const maxDownloads = 100
const expireDayOptions = [1, 7, 14, 30]

const expireTime = computed(() => {
  const date = new Date()
  date.setDate(date.getDate() + form.value.expire_days)
  return date.toISOString()
})

const canSubmit = computed(() => {
  return (
    selectedItemId.value !== null &&
    form.value.max_downloads >= minDownloads &&
    form.value.max_downloads <= maxDownloads
  )
})

const displayFileList = computed(() => {
  if (currentFolderId.value === 0) {
    return fileList.value
  }
  const parentItem: FileListItem = {
    id: -1,
    name: '..',
    type: 'folder',
    file_type: '',
    size: 0,
    updated_at: '',
  }
  return [parentItem, ...fileList.value]
})

const clearSelection = () => {
  selectedItemId.value = null
  form.value.file_id = null
  form.value.folder_id = null
}

const fetchFileList = async (folderId = 0) => {
  loading.value = true
  try {
    currentFolderId.value = folderId
    fileList.value = await getListByFolderIDAndUserID(folderId, 1, 100)
    clearSelection()
  } finally {
    loading.value = false
  }
}

const handleSelectItem = (item: FileListItem) => {
  if (item.id === -1) return
  selectedItemId.value = item.id
  if (item.type === 'folder') {
    form.value.folder_id = item.id
    form.value.file_id = null
    form.value.type = 'folder'
  } else {
    form.value.file_id = item.id
    form.value.folder_id = null
    form.value.type = 'file'
  }
}

const handleGoParent = async () => {
  if (pathStack.value.length <= 1) return
  pathStack.value = pathStack.value.slice(0, pathStack.value.length - 1)
  const parent = pathStack.value[pathStack.value.length - 1]
  await fetchFileList(parent.id)
}

const handleEnterFolder = async (item: FileListItem) => {
  if (item.id === -1) {
    await handleGoParent()
    return
  }
  if (item.type !== 'folder') return
  pathStack.value.push({ id: item.id, name: item.name })
  await fetchFileList(item.id)
}

const handleNavigatePath = async (index: number) => {
  const target = pathStack.value[index]
  if (!target) return
  pathStack.value = pathStack.value.slice(0, index + 1)
  await fetchFileList(target.id)
}

const handleSubmit = async () => {
  if (!canSubmit.value || submitting.value) return

  submitting.value = true
  try {
    const result = await createPickupCode({
      code: '',
      file_id: form.value.file_id,
      folder_id: form.value.folder_id,
      type: form.value.type,
      max_downloads: form.value.max_downloads,
      expire_time: expireTime.value,
    })

    if (result !== null) {
      toast.value = { type: 'success', text: '创建取件码成功' }
      setTimeout(() => {
        emit('success')
      }, 700)
      return
    }
    toast.value = { type: 'error', text: '创建取件码失败，请重试' }
  } catch {
    toast.value = { type: 'error', text: '创建取件码失败，请重试' }
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  fetchFileList(0)
})
</script>

<template>
  <!-- 面板容器由调用方 PickupCodes.vue 的 Motion 包裹（与其余弹窗同构）；此处仅面板内容 -->
  <div
    role="dialog"
    aria-modal="true"
    aria-label="创建取件码"
    class="overflow-hidden rounded-lg bg-surface shadow-popover"
  >
    <div class="flex items-center justify-between border-b border-hairline px-6 py-5">
      <h3 class="text-title-3">创建取件码</h3>
      <button
        class="flex h-9 w-9 items-center justify-center rounded-full text-label-secondary transition-[background-color,color,scale] duration-150 hover:bg-surface-secondary hover:text-label active:scale-[0.92]"
        type="button"
        aria-label="关闭"
        @click="emit('close')"
      >
        <AppIcon icon="material-symbols:close-rounded" class="text-xl" />
      </button>
    </div>

    <div class="flex flex-col gap-6 p-6">
      <!-- 结果反馈：成功 700ms 后关闭并 emit success；失败保留弹窗可重试 -->
      <div
        v-if="toast"
        class="flex items-center gap-2 rounded-sm px-3.5 py-2.5 text-[13px] font-medium"
        :class="
          toast.type === 'success'
            ? 'bg-primary-tint text-primary-pressed'
            : 'bg-danger-tint text-danger'
        "
      >
        <AppIcon
          :icon="
            toast.type === 'success' ? 'material-symbols:check-circle' : 'material-symbols:error'
          "
          class="flex-none text-[16px]"
        />
        {{ toast.text }}
      </div>

      <!-- 选择文件或文件夹：单击选中、双击进入文件夹；id=-1 的「..」行仅返回上一级，不可选中 -->
      <div class="flex flex-col gap-1.5">
        <label class="text-sm font-semibold text-label">选择文件或文件夹</label>
        <nav class="mb-1 flex items-center gap-0.5 overflow-x-auto" aria-label="目录路径">
          <button
            v-for="(path, index) in pathStack"
            :key="`${path.id}-${index}`"
            type="button"
            class="inline-flex items-center gap-0.5 whitespace-nowrap rounded-sm px-1.5 py-1 text-[13px] text-label-secondary transition-colors duration-150 hover:bg-primary-tint hover:text-primary"
            @click="handleNavigatePath(index)"
          >
            <AppIcon
              v-if="index !== 0"
              icon="material-symbols:chevron-right-rounded"
              class="text-sm"
            />
            <span>{{ path.name }}</span>
          </button>
        </nav>
        <div class="max-h-54 overflow-y-auto rounded-md border border-hairline">
          <div v-if="loading" class="flex items-center justify-center gap-3 px-4 py-[22px]">
            <AppIcon
              icon="material-symbols:progress-activity"
              class="animate-spin text-[18px] text-primary"
            />
            <span class="text-subhead text-label-secondary">正在加载文件...</span>
          </div>
          <div
            v-else-if="displayFileList.length === 0"
            class="px-4 py-[22px] text-center text-subhead text-label-secondary"
          >
            暂无可选文件，请先上传文件。
          </div>
          <template v-else>
            <template v-for="(item, index) in displayFileList" :key="item.id">
              <button
                type="button"
                class="flex min-h-12 w-full items-center gap-3 px-4 text-left transition-colors duration-150"
                :class="
                  selectedItemId === item.id && item.id !== -1
                    ? 'bg-primary-tint shadow-[inset_2px_0_0_0_var(--color-primary)] hover:bg-primary-tint'
                    : 'hover:bg-surface-secondary'
                "
                @click="handleSelectItem(item)"
                @dblclick="handleEnterFolder(item)"
              >
                <span
                  class="flex h-8 w-8 flex-none items-center justify-center rounded-sm"
                  :class="
                    item.id === -1
                      ? 'bg-surface-tertiary text-label-secondary'
                      : item.type === 'folder'
                        ? 'bg-warning-tint text-warning'
                        : 'bg-teal-tint text-info'
                  "
                >
                  <AppIcon
                    :icon="
                      item.id === -1
                        ? 'material-symbols:drive-folder-upload-rounded'
                        : item.type === 'folder'
                          ? 'material-symbols:folder-outline-rounded'
                          : 'material-symbols:description-outline-rounded'
                    "
                    class="text-[16px]"
                  />
                </span>
                <span class="flex min-w-0 flex-1 flex-col gap-px">
                  <span class="truncate text-[15px] font-semibold text-label">
                    {{ item.name }}
                  </span>
                  <span
                    class="text-caption"
                    :class="
                      selectedItemId === item.id && item.id !== -1
                        ? 'text-primary-pressed'
                        : 'text-label-tertiary'
                    "
                  >
                    {{ item.id === -1 ? '返回上一级' : item.type === 'folder' ? '文件夹' : '文件' }}
                  </span>
                </span>
                <AppIcon
                  v-if="selectedItemId === item.id && item.id !== -1"
                  icon="material-symbols:check-circle"
                  class="flex-none text-[18px] text-primary"
                />
              </button>
              <div v-if="index < displayFileList.length - 1" class="ml-15 h-px bg-hairline" />
            </template>
          </template>
        </div>
      </div>

      <!-- 最大下载次数：范围 1 - 100，越界则「生成取件码」禁用 -->
      <div class="flex flex-col gap-1.5">
        <label class="text-sm font-semibold text-label">最大下载次数</label>
        <input
          type="number"
          v-model.number="form.max_downloads"
          :min="minDownloads"
          :max="maxDownloads"
          class="h-11 w-full rounded-sm border border-hairline bg-surface-secondary px-3.5 text-[15px] text-label transition-[border-color,background-color,box-shadow] duration-150 placeholder:text-label-tertiary focus:border-primary focus:bg-surface focus:outline-none focus:ring-[3px] focus:ring-primary-tint"
        />
        <p class="text-caption text-label-tertiary">
          范围：{{ minDownloads }} - {{ maxDownloads }}
        </p>
      </div>

      <!-- 有效期：分段控件（surface-tertiary 轨道 + 选中白 pill）；到期时间按今天 + 天数实时计算 -->
      <div class="flex flex-col gap-1.5">
        <label class="text-sm font-semibold text-label">有效期</label>
        <div class="inline-flex gap-0.5 self-start rounded-full bg-surface-tertiary p-0.5">
          <button
            v-for="days in expireDayOptions"
            :key="days"
            type="button"
            class="h-7 rounded-full px-3.5 text-[13px] font-semibold transition-[background-color,color,box-shadow] duration-150"
            :class="
              form.expire_days === days
                ? 'bg-surface text-label shadow-sm'
                : 'text-label-secondary hover:text-label'
            "
            @click="form.expire_days = days"
          >
            {{ days }} 天
          </button>
        </div>
        <p class="text-caption text-label-tertiary">
          到期时间：{{ new Date(expireTime).toLocaleDateString() }}
        </p>
      </div>
    </div>

    <div class="flex justify-end gap-3 border-t border-hairline px-6 py-4">
      <button
        type="button"
        class="h-10 rounded-full bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97]"
        @click="emit('close')"
      >
        取消
      </button>
      <button
        type="button"
        class="flex h-10 items-center gap-2 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed disabled:cursor-not-allowed disabled:opacity-60"
        :disabled="!canSubmit || submitting"
        @click="handleSubmit"
      >
        <AppIcon v-if="submitting" icon="material-symbols:progress-activity" class="animate-spin" />
        {{ submitting ? '创建中...' : '生成取件码' }}
      </button>
    </div>
  </div>
</template>
