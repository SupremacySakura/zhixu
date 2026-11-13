<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AddKnowledgeDialog from '../components/knowledge/AddKnowledgeDialog.vue'
import KnowledgeTaskPanel from '../components/knowledge/KnowledgeTaskPanel.vue'
import { useKnowledgeImport } from '../composables/useKnowledgeImport'
import type { KnowledgeItem } from '../services/apis/knowledge'
import type { FileDisplayItem } from '../types/file'
import AppLoading from '../components/ui/AppLoading.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import LoginRequiredPlaceholder from '../components/bussiness/LoginRequiredPlaceholder.vue'
import FileListView from '../components/file-management/FileListView.vue'
import FileManagerHeader from '../components/file-management/FileManagerHeader.vue'
import FilePagination from '../components/file-management/FilePagination.vue'
import FileToolbar from '../components/file-management/FileToolbar.vue'
import { useFileActionMenu } from '../composables/useFileActionMenu'
import { fileFilterOptions, useFileList } from '../composables/useFileList'
import { useFileOperations } from '../composables/useFileOperations'
import { useFilePreview } from '../composables/useFilePreview'
import { useToast } from '../composables/useToast'
import { useUpload } from '../composables/useUpload'
import { useUserStore } from '../stores/user'
import type { FileViewMode, UploadTask } from '../types/file'

const MoveFileDialog = defineAsyncComponent(
  () => import('../components/file-management/MoveFileDialog.vue'),
)
const FileGridView = defineAsyncComponent(
  () => import('../components/file-management/FileGridView.vue'),
)
const FileNameDialog = defineAsyncComponent(
  () => import('../components/file-management/FileNameDialog.vue'),
)
const FilePreviewDialog = defineAsyncComponent(
  () => import('../components/file-management/FilePreviewDialog.vue'),
)
const UploadTaskPanel = defineAsyncComponent(
  () => import('../components/file-management/UploadTaskPanel.vue'),
)
const FileActionMenu = defineAsyncComponent(
  () => import('../components/file-management/FileActionMenu.vue'),
)

const userStore = useUserStore()
const headerRef = ref<InstanceType<typeof FileManagerHeader> | null>(null)

const { displayToast } = useToast()
const route = useRoute()
const router = useRouter()
const {
  target: knowledgeTarget,
  title: knowledgeTitle,
  error: knowledgeError,
  busy: knowledgeBusy,
  item: knowledgeItem,
  panelOpen: knowledgePanelOpen,
  panelError: knowledgePanelError,
  revision: knowledgeRevision,
  open: openKnowledge,
  submit: submitKnowledge,
  track: trackKnowledge,
  retry: retryKnowledge,
  poll: pollKnowledge,
} = useKnowledgeImport(displayToast)
function handleKnowledge(file: FileDisplayItem, item: KnowledgeItem | null) {
  closeFileMenu()
  if (item?.status === 'ready') {
    void router.push(`/home/knowledge/${item.id}`)
    return
  }
  if (item) trackKnowledge(item)
  else openKnowledge(file)
}

const {
  viewMode,
  sortKey,
  sortDirection,
  activeFilter,
  searchQuery,
  debouncedSearchQuery,
  currentFolderId,
  breadcrumbs,
  currentFolderName,
  hasParentFolder,
  page,
  totalCount,
  totalPages,
  startIndex,
  endIndex,
  sortedFiles,
  isLoading,
  errorMessage,
  selectedIds,
  selectedCount,
  allSelected,
  fetchFolder,
  refresh,
  onSearchInput,
  clearSearch,
  setFilter,
  setSort,
  clearSelection,
  toggleAll,
  toggleOne,
  selectCurrentPage,
  goToPage,
  goToBreadcrumb,
  goToParentFolder,
  onRowClick,
} = useFileList()

const {
  isCreateFolderModalOpen,
  newFolderName,
  isCreatingFolder,
  createFolderError,
  openCreateFolderModal,
  closeCreateFolderModal,
  handleCreateFolder,
  isRenameModalOpen,
  renameName,
  isRenaming,
  renamingTarget,
  renameError,
  openRenameModal,
  closeRenameModal,
  handleRename,
  isMoveModalOpen,
  isMoving,
  movingTarget,
  moveTargetFolderId,
  moveBrowserBreadcrumbs,
  moveBrowserFolders,
  isMoveBrowserLoading,
  openMoveModal,
  closeMoveModal,
  goToMoveBrowserFolder,
  goToMoveBrowserBreadcrumb,
  selectMoveTargetCurrent,
  handleMove,
  isDeleteConfirmModalOpen,
  deleteRelatedKnowledge,
  deleteConfirmTarget,
  deletingMenuTargetId,
  handleDeleteFromMenu,
  closeDeleteConfirmModal,
  confirmDeleteFromModal,
  downloadingMenuTargetId,
  handleDownloadFromMenu,
} = useFileOperations({
  currentFolderId,
  breadcrumbs,
  refresh: fetchFolder,
  notify: displayToast,
})

const {
  isPreviewModalOpen,
  previewLoading,
  previewError,
  previewingFile,
  previewBlob,
  previewUrl,
  previewTextContent,
  previewKind,
  publicShareLink,
  isLoadingShareLink,
  isCreatingShareLink,
  isDeletingShareLink,
  openPreviewModal,
  closePreviewModal,
  triggerPreviewDownload,
  generatePublicShareLink,
  removePublicShareLink,
  copyPublicShareLink,
} = useFilePreview({ notify: displayToast })

const {
  uploadTasks,
  isUploadPanelOpen,
  isUploading,
  overallProgress,
  handleFileInput,
  handleFolderInput,
  cancelTask,
  retryTask,
  removeTask,
  clearCompleted,
  clearAllTasks,
  toggleUploadPanel,
} = useUpload({ refresh, notify: displayToast })

const { isMenuOpen, menuTargetFile, menuPosition, openFileMenu, closeFileMenu } =
  useFileActionMenu()

const completedUploadCount = computed(
  () => uploadTasks.value.filter(task => task.status === 'success').length,
)
const pageErrorMessage = computed(() => errorMessage.value)
const emptyStateQuery = computed(() => {
  if (debouncedSearchQuery.value) return debouncedSearchQuery.value
  return activeFilter.value === 'all' ? '' : '当前筛选条件'
})
const renameDescription = computed(
  () => renameError.value ?? `当前对象：${renamingTarget.value?.name ?? '-'}`,
)
const deleteConfirmMessage = computed(
  () =>
    `将删除${deleteConfirmTarget.value?.type === 'folder' ? '文件夹' : '文件'}：「${deleteConfirmTarget.value?.name ?? '-'}」。此操作不可撤销。`,
)
const canDownloadPreview = computed(() => Boolean(previewBlob.value))
const isShareLinkBusy = computed(() => isLoadingShareLink.value || isCreatingShareLink.value)

const updateViewMode = (mode: FileViewMode) => {
  viewMode.value = mode
}

const openFileDialog = () => headerRef.value?.openFileDialog()

const startFileUploads = (files: File[]) => {
  handleFileInput(files, currentFolderId.value)
}

const startFolderUploads = (files: File[]) => {
  handleFolderInput(files, currentFolderId.value)
}

const takeMenuTarget = () => {
  const target = menuTargetFile.value
  closeFileMenu()
  return target
}

const previewMenuTarget = () => {
  const target = takeMenuTarget()
  if (target) void openPreviewModal(target)
}

const downloadMenuTarget = () => {
  const target = takeMenuTarget()
  if (target) void handleDownloadFromMenu(target)
}

const renameMenuTarget = () => {
  const target = takeMenuTarget()
  if (target) openRenameModal(target)
}

const moveMenuTarget = () => {
  const target = takeMenuTarget()
  if (target) void openMoveModal(target)
}

const deleteMenuTarget = () => {
  const target = takeMenuTarget()
  if (target) handleDeleteFromMenu(target)
}

const retryUploadTask = (task: UploadTask) => {
  void retryTask(task)
}

const closeAndClearUploadPanel = () => {
  clearAllTasks()
}

onMounted(() => {
  if (userStore.isLoggedIn) {
    const folder = Number(route?.query.folder || 0)
    if (Number.isSafeInteger(folder) && folder > 0) {
      currentFolderId.value = folder
      breadcrumbs.value = [
        { id: 0, name: 'root' },
        { id: folder, name: String(route?.query.folder_name || '来源目录') },
      ]
    }
    void fetchFolder(currentFolderId.value)
  }
})
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col">
    <LoginRequiredPlaceholder v-if="!userStore.isLoggedIn" />
    <template v-else>
      <!-- id 从 main-content 改成 main-content-page：#main-content 是外壳 skip-link
           的目标，由 HomePage.vue 的 router-view 独占。两个元素抢同一个 id，
           跳到主内容时浏览器只会去第一个，文件管理页永远接不到焦点。 -->
      <main id="main-content-page" class="flex min-w-0 flex-1 flex-col overflow-hidden">
        <FileManagerHeader
          ref="headerRef"
          :breadcrumbs="breadcrumbs"
          :current-folder-name="currentFolderName"
          :error-message="pageErrorMessage"
          :upload-task-count="uploadTasks.length"
          :completed-upload-count="completedUploadCount"
          :is-uploading="isUploading"
          :overall-progress="overallProgress"
          :is-upload-panel-open="isUploadPanelOpen"
          @navigate="goToBreadcrumb"
          @create-folder="openCreateFolderModal"
          @files-selected="startFileUploads"
          @folder-selected="startFolderUploads"
          @toggle-upload-panel="toggleUploadPanel"
        />

        <div class="shell-page">
          <div class="shell-wide">
            <FileToolbar
              :search-query="searchQuery"
              :view-mode="viewMode"
              :filter-options="fileFilterOptions"
              :active-filter="activeFilter"
              :selected-count="selectedCount"
              :sort-key="sortKey"
              :sort-direction="sortDirection"
              @update:search-query="onSearchInput"
              @update:view-mode="updateViewMode"
              @clear-search="clearSearch"
              @select-filter="setFilter"
              @select-sort="setSort"
              @select-current-page="selectCurrentPage"
              @clear-selection="clearSelection"
            />
            <div
              class="mb-5 flex flex-wrap items-center justify-between gap-3 bg-primary-tint px-4 py-3 text-body text-primary-pressed"
            >
              <p>支持 .md / .markdown 加入知识库，原文件保持不变。</p>
              <RouterLink to="/home/knowledge" class="knowledge-text-action"
                >打开知识库 →</RouterLink
              >
            </div>

            <FileListView
              v-if="viewMode === 'list'"
              :files="sortedFiles"
              :loading="isLoading"
              :has-parent-folder="hasParentFolder"
              :selected-ids="selectedIds"
              :all-selected="allSelected"
              :query="emptyStateQuery"
              :knowledge-revision="knowledgeRevision"
              @knowledge="handleKnowledge"
              @go-parent="goToParentFolder"
              @open-item="onRowClick"
              @toggle-all="toggleAll"
              @toggle-select="toggleOne"
              @open-menu="openFileMenu"
              @upload="openFileDialog"
            >
              <template #footer>
                <FilePagination
                  :page="page"
                  :total-pages="totalPages"
                  :start-index="startIndex"
                  :end-index="endIndex"
                  :total-count="totalCount"
                  :loading="isLoading"
                  @change="goToPage"
                />
              </template>
            </FileListView>

            <FileGridView
              v-else
              :files="sortedFiles"
              :loading="isLoading"
              :has-parent-folder="hasParentFolder"
              :selected-ids="selectedIds"
              :query="emptyStateQuery"
              :knowledge-revision="knowledgeRevision"
              @knowledge="handleKnowledge"
              @go-parent="goToParentFolder"
              @open-item="onRowClick"
              @toggle-select="toggleOne"
              @open-menu="openFileMenu"
              @upload="openFileDialog"
            >
              <template #footer>
                <FilePagination
                  v-if="sortedFiles.length > 0"
                  class="mt-6 overflow-hidden bg-surface shadow-card"
                  :page="page"
                  :total-pages="totalPages"
                  :start-index="startIndex"
                  :end-index="endIndex"
                  :total-count="totalCount"
                  :loading="isLoading"
                  @change="goToPage"
                />
              </template>
            </FileGridView>
          </div>

          <AddKnowledgeDialog
            v-if="knowledgeTarget"
            :file="knowledgeTarget"
            v-model:title="knowledgeTitle"
            :busy="knowledgeBusy"
            :error="knowledgeError"
            @close="knowledgeTarget = null"
            @submit="submitKnowledge"
          />
          <KnowledgeTaskPanel
            :item="knowledgeItem"
            :open="knowledgePanelOpen"
            :error="knowledgePanelError"
            :busy="knowledgeBusy"
            @close="knowledgePanelOpen = false"
            @retry="retryKnowledge"
            @refresh="pollKnowledge"
          />

          <FileNameDialog
            v-if="isCreateFolderModalOpen"
            v-model="isCreateFolderModalOpen"
            v-model:value="newFolderName"
            title="新建文件夹"
            label="文件夹名称"
            placeholder="请输入文件夹名称"
            :description="createFolderError"
            confirm-text="创建"
            loading-text="创建中..."
            :loading="isCreatingFolder"
            @cancel="closeCreateFolderModal"
            @confirm="handleCreateFolder"
          />

          <FileNameDialog
            v-if="isRenameModalOpen"
            v-model="isRenameModalOpen"
            v-model:value="renameName"
            title="重命名"
            label="新名称"
            placeholder="请输入新名称"
            :description="renameDescription"
            confirm-text="保存"
            loading-text="保存中..."
            :loading="isRenaming"
            @cancel="closeRenameModal"
            @confirm="handleRename"
          />

          <MoveFileDialog
            v-if="isMoveModalOpen"
            v-model="isMoveModalOpen"
            :target="movingTarget"
            :breadcrumbs="moveBrowserBreadcrumbs"
            :folders="moveBrowserFolders"
            :browser-loading="isMoveBrowserLoading"
            :target-folder-id="moveTargetFolderId"
            :moving="isMoving"
            @cancel="closeMoveModal"
            @navigate="goToMoveBrowserBreadcrumb"
            @open-folder="goToMoveBrowserFolder"
            @select-current="selectMoveTargetCurrent"
            @confirm="handleMove"
          />

          <ConfirmDialog
            v-model="isDeleteConfirmModalOpen"
            title="确认删除"
            :message="deleteConfirmMessage"
            confirm-text="确认删除"
            cancel-text="取消"
            :loading="Boolean(deletingMenuTargetId)"
            :danger="true"
            @cancel="closeDeleteConfirmModal"
            @confirm="confirmDeleteFromModal"
          >
            <div class="mb-6 bg-surface-secondary p-4 text-body">
              <label class="flex items-start gap-3">
                <input
                  v-model="deleteRelatedKnowledge"
                  type="checkbox"
                  class="mt-1 accent-primary"
                  :disabled="Boolean(deletingMenuTargetId)"
                />
                <span>同时删除关联知识条目</span>
              </label>
              <p class="mt-2 text-note leading-6 text-label-secondary">
                若已加入知识库，不勾选则保留知识正文；勾选会删除关联知识和处理任务。删除文件夹时包含其子目录中的文件。
              </p>
            </div>
            <template #confirm-icon>
              <AppLoading v-if="deletingMenuTargetId" size="sm" />
            </template>
          </ConfirmDialog>

          <FilePreviewDialog
            v-if="isPreviewModalOpen"
            v-model="isPreviewModalOpen"
            :file="previewingFile"
            :loading="previewLoading"
            :error="previewError"
            :kind="previewKind"
            :url="previewUrl"
            :text-content="previewTextContent"
            :can-download="canDownloadPreview"
            :public-share-link="publicShareLink"
            :creating-share-link="isShareLinkBusy"
            :deleting-share-link="isDeletingShareLink"
            @close="closePreviewModal"
            @download="triggerPreviewDownload"
            @create-share="generatePublicShareLink"
            @delete-share="removePublicShareLink"
            @copy-share="copyPublicShareLink"
          />

          <UploadTaskPanel
            v-if="uploadTasks.length > 0 && isUploadPanelOpen"
            v-model="isUploadPanelOpen"
            :tasks="uploadTasks"
            :is-uploading="isUploading"
            :overall-progress="overallProgress"
            @close="closeAndClearUploadPanel"
            @clear-completed="clearCompleted"
            @retry="retryUploadTask"
            @cancel="cancelTask"
            @remove="removeTask"
          />

          <FileActionMenu
            v-if="isMenuOpen"
            :visible="isMenuOpen"
            :target="menuTargetFile"
            :position="menuPosition"
            :downloading="Boolean(downloadingMenuTargetId)"
            :deleting="Boolean(deletingMenuTargetId)"
            @preview="previewMenuTarget"
            @download="downloadMenuTarget"
            @rename="renameMenuTarget"
            @move="moveMenuTarget"
            @delete="deleteMenuTarget"
            @knowledge="handleKnowledge"
            @close="closeFileMenu"
          />
        </div>
      </main>
    </template>
  </div>
</template>
