<script setup lang="ts">
import AppIcon from '../components/ui/AppIcon.vue'
import { computed, reactive, ref } from 'vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../utils/motion'
import { getListByFolderIDAndUserID, uploadFile } from '../services/apis/file'
import { detectFileType, formatBytes, iconForFile } from '../utils/file'
import { createId } from '../utils/hash'
import { useUserStore } from '../stores/user'
import LoginRequiredPlaceholder from '../components/bussiness/LoginRequiredPlaceholder.vue'

const userStore = useUserStore()

type QueueItemStatus =
  | 'pending'
  | 'hashing'
  | 'uploading'
  | 'merging'
  | 'success'
  | 'failed'
  | 'canceled'

type QueueItem = {
  id: string
  file: File
  status: QueueItemStatus
  percent: number
  message: string | null
  canceled: boolean
}

const isDragging = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const items = ref<QueueItem[]>([])
const processingCount = computed(
  () =>
    items.value.filter(i => ['pending', 'hashing', 'uploading', 'merging'].includes(i.status))
      .length,
)
const clampPercent = (value: number) => Math.min(100, Math.max(0, Math.floor(value)))

type BreadcrumbItem = { id: number; name: string }
const selectedUploadFolderId = ref(0)
const selectedUploadFolderPath = ref('root')
const isFolderPickerOpen = ref(false)
const isFolderPickerLoading = ref(false)
const folderPickerCurrentFolderId = ref(0)
const folderPickerBreadcrumbs = ref<BreadcrumbItem[]>([{ id: 0, name: 'root' }])
const folderPickerFolders = ref<{ id: number; name: string }[]>([])
const folderPickerErrorMessage = ref<string | null>(null)

const removeItem = (id: string) => {
  items.value = items.value.filter(x => x.id !== id)
}

const cancelItem = (item: QueueItem) => {
  item.canceled = true
  item.status = 'canceled'
  item.message = '已取消'
}

const retryItem = async (item: QueueItem) => {
  item.status = 'pending'
  item.percent = 0
  item.message = null
  item.canceled = false
  await startUpload(item)
}

const clearCompleted = () => {
  items.value = items.value.filter(x => x.status !== 'success')
}

const openFileDialog = () => {
  fileInputRef.value?.click()
}

const loadFolderPickerFolders = async (folderId: number) => {
  isFolderPickerLoading.value = true
  folderPickerErrorMessage.value = null
  try {
    const list = await getListByFolderIDAndUserID(folderId, 1, 100)
    folderPickerFolders.value = list
      .filter(item => item.type === 'folder')
      .map(item => ({ id: item.id, name: item.name }))
  } catch (error: unknown) {
    folderPickerFolders.value = []
    folderPickerErrorMessage.value = error instanceof Error ? error.message : '加载目录失败'
  } finally {
    isFolderPickerLoading.value = false
  }
}

const openFolderPicker = async () => {
  isFolderPickerOpen.value = true
  folderPickerCurrentFolderId.value = 0
  folderPickerBreadcrumbs.value = [{ id: 0, name: 'root' }]
  await loadFolderPickerFolders(0)
}

const closeFolderPicker = () => {
  isFolderPickerOpen.value = false
}

const goToFolderPickerFolder = async (folder: { id: number; name: string }) => {
  folderPickerCurrentFolderId.value = folder.id
  folderPickerBreadcrumbs.value = [
    ...folderPickerBreadcrumbs.value,
    { id: folder.id, name: folder.name },
  ]
  await loadFolderPickerFolders(folder.id)
}

const goToFolderPickerBreadcrumb = async (index: number) => {
  const next = folderPickerBreadcrumbs.value[index]
  if (!next) return
  folderPickerCurrentFolderId.value = next.id
  folderPickerBreadcrumbs.value = folderPickerBreadcrumbs.value.slice(0, index + 1)
  await loadFolderPickerFolders(next.id)
}

const selectCurrentFolderForUpload = () => {
  selectedUploadFolderId.value = folderPickerCurrentFolderId.value
  selectedUploadFolderPath.value = folderPickerBreadcrumbs.value.map(item => item.name).join(' / ')
  closeFolderPicker()
}

const pushFiles = async (files: File[]) => {
  for (const file of files) {
    const item = reactive<QueueItem>({
      id: createId(),
      file,
      status: 'pending',
      percent: 0,
      message: null,
      canceled: false,
    })
    items.value.unshift(item)
    await startUpload(item)
  }
}

const onFileInputChange = async (e: Event) => {
  const el = e.target as HTMLInputElement
  const fileList = el.files
  if (!fileList?.length) return
  const files = Array.from(fileList)
  el.value = ''
  await pushFiles(files)
}

const onDragOver = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = true
}

const onDragLeave = () => {
  isDragging.value = false
}

const onDrop = async (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = false
  const fileList = e.dataTransfer?.files
  if (!fileList?.length) return
  await pushFiles(Array.from(fileList))
}

const startUpload = async (item: QueueItem) => {
  try {
    item.status = 'uploading'
    item.percent = 0
    item.message = '上传中...'
    const fileType = detectFileType(item.file)
    await uploadFile(
      item.file,
      {
        file_type: fileType,
        folder_id: selectedUploadFolderId.value,
      },
      progress => {
        if (item.canceled) return
        item.percent = clampPercent(progress)
        item.message = progress >= 100 ? '上传完成' : '上传中...'
      },
    )

    if (item.canceled) return
    item.status = 'success'
    item.percent = 100
    item.message = '上传完成'
  } catch (error: unknown) {
    if (item.canceled) return
    item.status = 'failed'
    item.message = error instanceof Error ? error.message : '上传失败'
  }
}

const badgeText = (item: QueueItem) => {
  if (item.status === 'success') return 'Success'
  if (item.status === 'failed') return 'Failed'
  if (item.status === 'canceled') return 'Canceled'
  if (item.status === 'merging') return 'Merging'
  if (item.status === 'hashing') return 'Hashing'
  if (item.status === 'pending') return 'Pending'
  return `${clampPercent(item.percent)}%`
}

const badgeClass = (item: QueueItem) => {
  if (item.status === 'failed') return 'text-danger'
  if (item.status === 'success') return 'text-primary'
  if (item.status === 'canceled') return 'text-label-tertiary'
  return 'text-primary'
}
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col bg-canvas text-label">
    <LoginRequiredPlaceholder v-if="!userStore.isLoggedIn" />
    <template v-else>
      <main class="flex-1 overflow-y-auto">
        <div
          class="mx-auto flex w-full max-w-[960px] flex-col gap-8 px-4 py-6 sm:px-6 lg:px-8 lg:py-8"
        >
          <!-- 上传目录信息条 -->
          <section
            class="flex items-center justify-between gap-4 rounded-md bg-surface px-[18px] py-3.5 shadow-card"
          >
            <div class="min-w-0">
              <p class="text-caption text-label-secondary">上传目录</p>
              <p class="text-headline truncate text-label">{{ selectedUploadFolderPath }}</p>
            </div>
            <button
              class="inline-flex h-10 flex-none items-center gap-1.5 rounded-full bg-primary-tint px-4 text-[15px] font-semibold text-primary transition-[background-color,scale] duration-150 hover:bg-primary/[0.18] active:scale-[0.97]"
              type="button"
              aria-label="选择上传目录"
              @click="openFolderPicker"
            >
              <AppIcon icon="material-symbols:folder-open-outline" />
              选择目录
            </button>
          </section>

          <input
            ref="fileInputRef"
            type="file"
            class="hidden"
            multiple
            @change="onFileInputChange"
          />

          <!-- 大拖放区：虚线淡描边 + surface-secondary 底；悬停品牌绿描边，拖拽中 primary-tint 底 + 轻微放大 -->
          <section
            class="group flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed border-label-tertiary bg-surface-secondary px-4 py-16 text-center transition-[transform,border-color,background-color,box-shadow] duration-200 ease-out hover:border-primary"
            :class="isDragging ? 'scale-[1.01] border-primary bg-primary-tint shadow-card' : ''"
            @click="openFileDialog"
            @dragover="onDragOver"
            @dragleave="onDragLeave"
            @drop="onDrop"
          >
            <div
              class="mb-4 flex size-16 items-center justify-center rounded-full bg-surface text-primary shadow-card transition-transform duration-200 ease-out group-hover:scale-110"
              :class="isDragging ? 'scale-110' : ''"
            >
              <AppIcon class="text-[28px]" icon="material-symbols:upload-file" />
            </div>
            <h3 class="text-title-3 mb-1.5">拖拽文件到此处上传</h3>
            <p class="text-subhead mb-5 max-w-sm text-label-secondary">
              支持图片、视频、文档与压缩包等常见格式。
            </p>
            <button
              class="h-10 rounded-full bg-surface px-[18px] text-[15px] font-semibold text-label ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97]"
              type="button"
              aria-label="选择文件"
            >
              选择文件
            </button>
          </section>

          <!-- 选择上传目录弹窗：Teleport + 遮罩淡入淡出 + 面板弹簧 scale 0.96→1（motion-v） -->
          <Teleport to="body">
            <AnimatePresence>
              <Motion
                v-if="isFolderPickerOpen"
                key="folder-picker-overlay"
                :initial="{ opacity: 0 }"
                :animate="{ opacity: 1 }"
                :exit="{ opacity: 0 }"
                :transition="fadeTransition"
                class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4 backdrop-blur-sm"
                role="dialog"
                aria-modal="true"
                @click.self="closeFolderPicker"
              >
                <Motion
                  :initial="{ opacity: 0, scale: 0.96 }"
                  :animate="{ opacity: 1, scale: 1 }"
                  :exit="{ opacity: 0, scale: 0.98 }"
                  :transition="springSnappy"
                  class="flex max-h-[min(80vh,42rem)] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-surface p-6 shadow-popover"
                  @click.stop
                >
                  <h3 class="text-title-3 mb-2">选择上传目录</h3>
                  <p class="text-subhead text-label-secondary mb-4">
                    默认上传到 root，可切换到任意已有文件夹。
                  </p>

                  <div class="mb-3 flex items-center justify-between gap-3">
                    <nav
                      class="flex items-center gap-1.5 overflow-x-auto text-caption text-label-secondary"
                    >
                      <template
                        v-for="(bc, idx) in folderPickerBreadcrumbs"
                        :key="`${bc.id}-${idx}`"
                      >
                        <button
                          v-if="idx < folderPickerBreadcrumbs.length - 1"
                          class="whitespace-nowrap rounded-sm transition-colors duration-150 hover:text-primary"
                          type="button"
                          :aria-label="`导航到 ${bc.name}`"
                          @click="goToFolderPickerBreadcrumb(idx)"
                        >
                          {{ bc.name }}
                        </button>
                        <span v-else class="whitespace-nowrap font-semibold text-label">
                          {{ bc.name }}
                        </span>
                        <AppIcon
                          v-if="idx < folderPickerBreadcrumbs.length - 1"
                          icon="material-symbols:chevron-right"
                          class="text-[14px] text-label-tertiary"
                        />
                      </template>
                    </nav>
                    <button
                      class="h-8 flex-none rounded-full bg-surface px-3.5 text-[13px] font-semibold text-label ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97]"
                      type="button"
                      aria-label="选择当前目录"
                      @click="selectCurrentFolderForUpload"
                    >
                      选择当前目录
                    </button>
                  </div>

                  <!-- 目录浏览器：48px 行，hover 底色，分隔线自图标后内缩 -->
                  <div class="overflow-hidden rounded-md border border-hairline">
                    <div class="max-h-72 overflow-y-auto">
                      <div
                        v-if="isFolderPickerLoading"
                        class="px-3.5 py-3 text-subhead text-label-secondary"
                      >
                        正在加载目录...
                      </div>
                      <div
                        v-else-if="folderPickerErrorMessage"
                        class="px-3.5 py-3 text-subhead text-danger"
                      >
                        {{ folderPickerErrorMessage }}
                      </div>
                      <template v-else>
                        <template v-for="(folder, index) in folderPickerFolders" :key="folder.id">
                          <button
                            class="flex min-h-12 w-full items-center gap-2.5 px-3.5 text-left transition-colors duration-150 hover:bg-surface-secondary focus:outline-none"
                            type="button"
                            :aria-label="`打开文件夹 ${folder.name}`"
                            @click="goToFolderPickerFolder(folder)"
                          >
                            <AppIcon
                              icon="material-symbols:folder"
                              class="flex-none text-[18px] text-primary"
                            />
                            <span class="min-w-0 flex-1 truncate text-subhead text-label">
                              {{ folder.name }}
                            </span>
                            <AppIcon
                              icon="material-symbols:chevron-right"
                              class="flex-none text-[16px] text-label-tertiary"
                            />
                          </button>
                          <div
                            v-if="index < folderPickerFolders.length - 1"
                            class="ml-[52px] h-px bg-hairline"
                          />
                        </template>
                        <div
                          v-if="folderPickerFolders.length === 0"
                          class="px-3.5 py-3 text-subhead text-label-secondary"
                        >
                          当前目录下没有子文件夹
                        </div>
                      </template>
                    </div>
                  </div>

                  <div class="mt-5 flex justify-end gap-3">
                    <button
                      class="h-10 rounded-full bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97]"
                      type="button"
                      aria-label="取消选择"
                      @click="closeFolderPicker"
                    >
                      取消
                    </button>
                    <button
                      class="h-10 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
                      type="button"
                      aria-label="确认选择当前目录"
                      @click="selectCurrentFolderForUpload"
                    >
                      确认
                    </button>
                  </div>
                </Motion>
              </Motion>
            </AnimatePresence>
          </Teleport>

          <!-- 上传队列 -->
          <section>
            <div class="mb-3.5 flex items-center justify-between gap-3">
              <h3 class="text-title-3">上传队列</h3>
              <div class="flex items-center gap-4">
                <span class="text-subhead text-label-secondary"
                  >{{ processingCount }} 项处理中</span
                >
                <button
                  class="flex h-8 items-center rounded-full px-3 text-[13px] font-semibold text-primary transition-[background-color,scale] duration-150 hover:bg-primary-tint active:scale-[0.97]"
                  type="button"
                  aria-label="清理已完成的上传任务"
                  @click="clearCompleted"
                >
                  清理已完成
                </button>
              </div>
            </div>

            <div
              v-if="items.length === 0"
              class="rounded-md bg-surface p-10 text-center shadow-card"
            >
              <p class="text-body text-label-secondary">暂无上传任务</p>
            </div>

            <div v-else class="flex flex-col gap-3">
              <div
                v-for="item in items"
                :key="item.id"
                class="flex min-h-16 items-center gap-3 rounded-md bg-surface-secondary/70 p-3.5 transition-colors duration-150 hover:bg-surface-tertiary/60"
              >
                <span
                  class="flex h-10 w-10 flex-none items-center justify-center rounded-sm"
                  :class="`${iconForFile(item.file).bg} ${iconForFile(item.file).fg}`"
                >
                  <AppIcon class="text-[20px]" :icon="iconForFile(item.file).icon" />
                </span>

                <div class="flex min-w-0 flex-1 flex-col gap-1.5">
                  <div class="flex items-center justify-between gap-3">
                    <p
                      class="truncate text-[15px] font-semibold"
                      :class="item.status === 'canceled' ? 'text-label-secondary' : 'text-label'"
                    >
                      {{ item.file.name }}
                    </p>
                    <span
                      class="flex-none text-caption font-bold uppercase tracking-[0.06em]"
                      :class="badgeClass(item)"
                    >
                      {{ badgeText(item) }}
                    </span>
                  </div>

                  <div class="flex min-w-0 items-center gap-3">
                    <div
                      class="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-surface-tertiary"
                    >
                      <div
                        class="h-full rounded-full transition-[width] duration-[120ms] ease-linear"
                        :class="
                          item.status === 'failed'
                            ? 'bg-danger'
                            : item.status === 'canceled'
                              ? 'bg-label-tertiary/40'
                              : 'bg-primary'
                        "
                        :style="{ width: `${clampPercent(item.percent)}%` }"
                      ></div>
                    </div>
                    <span class="flex-none truncate text-caption text-label-secondary">
                      {{ formatBytes(item.file.size) }} • {{ item.message || '等待中' }}
                    </span>
                  </div>
                </div>

                <div class="flex flex-none items-center gap-1.5">
                  <button
                    v-if="item.status === 'failed'"
                    class="flex h-8 items-center gap-1.5 rounded-full bg-primary-tint px-3 text-[13px] font-semibold text-primary transition-[background-color,scale] duration-150 hover:bg-primary/[0.18] active:scale-[0.97]"
                    type="button"
                    aria-label="重试上传"
                    @click="retryItem(item)"
                  >
                    <AppIcon class="text-[16px]" icon="material-symbols:replay" />
                    重试
                  </button>

                  <div
                    v-if="item.status === 'success'"
                    class="p-1.5 text-primary"
                    aria-label="上传成功"
                  >
                    <AppIcon class="text-[22px]" icon="material-symbols:check-circle" />
                  </div>

                  <button
                    v-if="
                      item.status === 'uploading' ||
                      item.status === 'hashing' ||
                      item.status === 'merging'
                    "
                    class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-danger"
                    type="button"
                    aria-label="取消上传"
                    @click="cancelItem(item)"
                  >
                    <AppIcon class="text-[20px]" icon="material-symbols:close" />
                  </button>

                  <button
                    v-else
                    class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-danger"
                    type="button"
                    aria-label="移除任务"
                    @click="removeItem(item.id)"
                  >
                    <AppIcon class="text-[20px]" icon="material-symbols:close" />
                  </button>
                </div>
              </div>
            </div>
          </section>
        </div>
      </main>
    </template>
  </div>
</template>

<style lang="sass" scoped></style>
