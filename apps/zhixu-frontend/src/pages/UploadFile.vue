<script setup lang="ts">
import AppBadge from '../components/ui/AppBadge.vue'
import AppButton from '../components/ui/AppButton.vue'
import AppDialog from '../components/ui/AppDialog.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import AppLoading from '../components/ui/AppLoading.vue'
import { computed, reactive, ref } from 'vue'
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

/**
 * 队列状态词（R18）。
 *
 * 这里原先返回 Success / Failed / Canceled / Merging / Hashing / Pending ——
 * 「一个字未改」约束的是**既有中文串**，英文串与「中文界面」自相矛盾，
 * 所以整组换成中文。上传中是唯一带数字的状态，保留百分比。
 */
const badgeText = (item: QueueItem) => {
  if (item.status === 'success') return '成功'
  if (item.status === 'failed') return '失败'
  if (item.status === 'canceled') return '已取消'
  if (item.status === 'merging') return '合并中'
  if (item.status === 'hashing') return '校验中'
  if (item.status === 'pending') return '等待中'
  return `${clampPercent(item.percent)}%`
}

/**
 * 状态语气：徽章的颜色由 AppBadge 的 tone 决定，不再手写 text-* 类。
 * 终态用实义语气（成功=primary / 失败=danger / 取消=neutral），
 * 进行中用 info，读起来「还在动」而不是「已经定了」。
 */
const badgeTone = (item: QueueItem): 'primary' | 'danger' | 'neutral' | 'info' => {
  if (item.status === 'failed') return 'danger'
  if (item.status === 'success') return 'primary'
  if (item.status === 'canceled') return 'neutral'
  if (item.status === 'pending') return 'neutral'
  return 'info'
}
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col bg-canvas text-label">
    <LoginRequiredPlaceholder v-if="!userStore.isLoggedIn" />
    <template v-else>
      <!-- 页头带 64px：上传是表单原型（DESIGN 原型 ④），容器 panel（896）。
           标题原先是页面内 section，现在提到带里，与其他八页对齐。 -->
      <header class="shell-band-panel">
        <div class="min-w-0 flex-1">
          <h1 class="text-title">上传文件</h1>
          <p class="font-type text-micro text-label-tertiary">拖拽或选择文件，保留原件</p>
        </div>
      </header>

      <main class="shell-page">
        <!-- 分节 48px（DESIGN §Layout ④表单：字段 24 / 分节 48） -->
        <div class="shell-panel flex flex-col gap-12">
          <!-- 上传目录信息条 -->
          <section class="cabinet-card flex items-center justify-between gap-4 px-[18px] py-3.5">
            <div class="min-w-0">
              <p class="text-caption text-label-secondary">上传目录</p>
              <p class="text-headline truncate text-label">{{ selectedUploadFolderPath }}</p>
            </div>
            <AppButton
              tone="secondary"
              size="md"
              icon="material-symbols:folder-open-outline"
              aria-label="选择上传目录"
              @click="openFolderPicker"
            >
              选择目录
            </AppButton>
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
            class="group flex cursor-pointer flex-col items-center justify-center border-2 border-dashed border-label-tertiary bg-surface-secondary px-4 py-16 text-center transition-[transform,border-color,background-color,box-shadow] hover:border-primary"
            :class="isDragging ? 'scale-[1.01] border-primary bg-primary-tint shadow-card' : ''"
            @click="openFileDialog"
            @dragover="onDragOver"
            @dragleave="onDragLeave"
            @drop="onDrop"
          >
            <div
              class="mb-4 flex size-16 items-center justify-center bg-surface text-primary shadow-card transition-transform"
              :class="isDragging ? 'scale-110' : ''"
            >
              <AppIcon
                class="atom-glyph"
                icon="material-symbols:upload-file"
                :style="{ '--atom-glyph-size': '28px' }"
              />
            </div>
            <h3 class="text-title-3 mb-1.5">拖拽文件到此处上传</h3>
            <p class="text-subhead mb-5 max-w-sm text-label-secondary">
              支持图片、视频、文档与压缩包等常见格式。
            </p>
            <AppButton tone="secondary" size="lg" aria-label="选择文件">选择文件</AppButton>
          </section>

          <!-- 选择上传目录弹窗 -->
          <AppDialog
            v-model="isFolderPickerOpen"
            title="选择上传目录"
            description="默认上传到 root，可切换到任意已有文件夹。"
            size="md"
            @close="closeFolderPicker"
          >
            <div class="mb-3 flex items-center justify-between gap-3">
              <nav
                class="flex items-center gap-1.5 overflow-x-auto text-caption text-label-secondary"
              >
                <template v-for="(bc, idx) in folderPickerBreadcrumbs" :key="`${bc.id}-${idx}`">
                  <button
                    v-if="idx < folderPickerBreadcrumbs.length - 1"
                    class="whitespace-nowrap transition-colors hover:text-primary"
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
                    class="text-body text-label-tertiary"
                  />
                </template>
              </nav>
              <AppButton size="sm" aria-label="选择当前目录" @click="selectCurrentFolderForUpload">
                选择当前目录
              </AppButton>
            </div>

            <!-- 目录浏览器：48px 行，hover 底色，分隔线自图标后内缩 -->
            <div class="overflow-hidden border border-hairline">
              <div class="max-h-72 overflow-y-auto">
                <!-- 加载态：三页统一用 AppLoading（不传 label，见 R13） -->
                <div
                  v-if="isFolderPickerLoading"
                  class="flex items-center gap-2.5 px-3.5 py-3 text-subhead text-label-secondary"
                >
                  <AppLoading size="sm" />正在加载目录...
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
                      class="flex min-h-12 w-full items-center gap-2.5 px-3.5 text-left transition-colors hover:bg-surface-secondary"
                      type="button"
                      :aria-label="`打开文件夹 ${folder.name}`"
                      @click="goToFolderPickerFolder(folder)"
                    >
                      <AppIcon
                        icon="material-symbols:folder"
                        class="flex-none atom-glyph text-primary"
                        :style="{ '--atom-glyph-size': '18px' }"
                      />
                      <span class="min-w-0 flex-1 truncate text-subhead text-label">
                        {{ folder.name }}
                      </span>
                      <AppIcon
                        icon="material-symbols:chevron-right"
                        class="flex-none atom-glyph text-label-tertiary"
                        :style="{ '--atom-glyph-size': '16px' }"
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

            <template #footer>
              <AppButton aria-label="取消选择" @click="closeFolderPicker">取消</AppButton>
              <AppButton
                tone="primary"
                aria-label="确认选择当前目录"
                @click="selectCurrentFolderForUpload"
              >
                确认
              </AppButton>
            </template>
          </AppDialog>

          <!-- 上传队列（分节交界：一条 hairline；gap-12 已给上方 48px） -->
          <section class="border-t border-hairline pt-6">
            <div class="mb-3.5 flex items-center justify-between gap-3">
              <h3 class="text-title-3">上传队列</h3>
              <div class="flex items-center gap-4">
                <span class="text-subhead text-label-secondary"
                  >{{ processingCount }} 项处理中</span
                >
                <AppButton
                  tone="quiet"
                  size="sm"
                  aria-label="清理已完成的上传任务"
                  @click="clearCompleted"
                >
                  清理已完成
                </AppButton>
              </div>
            </div>

            <AppEmptyState
              v-if="items.length === 0"
              title="暂无上传任务"
              description="拖拽文件到上方虚线区，或点击「选择文件」。"
              icon="material-symbols:upload-file"
            />

            <div v-else class="flex flex-col gap-3">
              <div
                v-for="item in items"
                :key="item.id"
                class="flex min-h-16 items-center gap-3 bg-surface-secondary/70 p-3.5 transition-colors hover:bg-surface-tertiary/60"
              >
                <span
                  class="flex h-10 w-10 flex-none items-center justify-center"
                  :class="`${iconForFile(item.file).bg} ${iconForFile(item.file).fg}`"
                >
                  <AppIcon
                    class="atom-glyph"
                    :icon="iconForFile(item.file).icon"
                    :style="{ '--atom-glyph-size': '20px' }"
                  />
                </span>

                <div class="flex min-w-0 flex-1 flex-col gap-1.5">
                  <div class="flex items-center justify-between gap-3">
                    <p
                      class="truncate text-lead font-semibold"
                      :class="item.status === 'canceled' ? 'text-label-secondary' : 'text-label'"
                    >
                      {{ item.file.name }}
                    </p>
                    <AppBadge size="sm" :tone="badgeTone(item)">{{ badgeText(item) }}</AppBadge>
                  </div>

                  <div class="flex min-w-0 items-center gap-3">
                    <div class="h-1.5 min-w-0 flex-1 overflow-hidden bg-surface-tertiary">
                      <div
                        class="h-full transition-[width] duration-standard"
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

                <div class="flex flex-none items-center gap-1.5 pointer-coarse:gap-3">
                  <AppButton
                    v-if="item.status === 'failed'"
                    size="sm"
                    icon="material-symbols:replay"
                    aria-label="重试上传"
                    @click="retryItem(item)"
                  >
                    重试
                  </AppButton>

                  <div
                    v-if="item.status === 'success'"
                    class="p-1.5 text-primary"
                    aria-label="上传成功"
                  >
                    <AppIcon
                      class="atom-glyph"
                      icon="material-symbols:check-circle"
                      :style="{ '--atom-glyph-size': '22px' }"
                    />
                  </div>

                  <AppButton
                    v-if="
                      item.status === 'uploading' ||
                      item.status === 'hashing' ||
                      item.status === 'merging'
                    "
                    tone="quiet"
                    size="sm"
                    icon-only
                    icon="material-symbols:close"
                    aria-label="取消上传"
                    @click="cancelItem(item)"
                  />

                  <AppButton
                    v-else
                    tone="quiet"
                    size="sm"
                    icon-only
                    icon="material-symbols:close"
                    aria-label="移除任务"
                    @click="removeItem(item.id)"
                  />
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
