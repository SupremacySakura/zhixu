<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import AppIcon from '../components/ui/AppIcon.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../utils/motion'
import { getPickupCodeList, getPickupCodeCount, deletePickupCode } from '../services/apis/file'
import type { PickupCodeItem, PickupCodeType } from '../services/types/file'
import CreatePickupCodeModal from '../components/bussiness/CreatePickupCodeModal.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import { sanitizeFileName } from '../utils/file'

const pickupList = ref<PickupCodeItem[]>([])
const loading = ref(false)
const showCreateModal = ref(false)
const showDetailModal = ref(false)
const selectedItem = ref<PickupCodeItem | null>(null)
const router = useRouter()
const openMenuId = ref<string | null>(null)
const menuTargetItem = ref<PickupCodeItem | null>(null)
const menuPosition = ref<{ top: number; left: number } | null>(null)
const deletingCodeId = ref<number | null>(null)
const showDeleteConfirm = ref(false)
const deleteTargetItem = ref<PickupCodeItem | null>(null)

const toastMessage = ref('')
const toastType = ref<'success' | 'error' | 'info'>('info')
const showToast = ref(false)
let toastTimer: ReturnType<typeof setTimeout> | null = null

const currentPage = ref(1)
const pageSize = ref(10)
const totalCount = ref(0)

const stats = computed(() => {
  const now = new Date()
  const weekLater = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000)

  return {
    activeCount: pickupList.value.filter(item => item.status === 'Active').length,
    totalDownloads: pickupList.value.reduce((sum, item) => sum + item.download, 0),
    expiringSoon: pickupList.value.filter(item => {
      const expireDate = new Date(item.expire_time)
      return expireDate >= now && expireDate <= weekLater
    }).length,
  }
})

const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize.value)))
const startIndex = computed(() =>
  totalCount.value === 0 ? 0 : (currentPage.value - 1) * pageSize.value + 1,
)
const endIndex = computed(() => Math.min(currentPage.value * pageSize.value, totalCount.value))

const pageNumbers = computed(() => {
  const total = totalPages.value
  if (total <= 3) return Array.from({ length: total }, (_, i) => i + 1)
  if (currentPage.value <= 1) return [1, 2, 3]
  if (currentPage.value >= total) return [total - 2, total - 1, total]
  return [currentPage.value - 1, currentPage.value, currentPage.value + 1]
})

const progressPercent = (item: PickupCodeItem) => {
  if (item.max_download <= 0) return 0
  return Math.min(100, Math.round((item.download / item.max_download) * 100))
}

const typeIcon = (type: PickupCodeType) => {
  if (type === 'folder') return 'material-symbols:folder-zip-outline-rounded'
  return 'material-symbols:description-outline-rounded'
}

const statusLabel = (status: string) => {
  return status === 'Active' ? '生效中' : '已过期'
}

const typeLabel = (type: PickupCodeType) => {
  return type === 'folder' ? '文件夹' : '文件'
}

const formatDate = (dateStr: string) => {
  const date = new Date(dateStr)
  return date.toISOString().split('T')[0]
}

const fetchData = async () => {
  loading.value = true
  try {
    const [list, count] = await Promise.all([
      getPickupCodeList(currentPage.value, pageSize.value),
      getPickupCodeCount(),
    ])
    pickupList.value = (list ?? []).map(item => ({
      ...item,
      name: sanitizeFileName(item.name),
    }))
    totalCount.value = count ?? 0
  } catch (error) {
    console.error('Failed to fetch pickup codes:', error)
    pickupList.value = []
    totalCount.value = 0
  } finally {
    loading.value = false
  }
}

const handlePageChange = (page: number) => {
  if (page < 1 || page > totalPages.value) return
  currentPage.value = page
  fetchData()
}

const openCreateModal = () => {
  showCreateModal.value = true
}

const goToFilePickup = () => {
  router.push('/pickup')
}

const closeCreateModal = () => {
  showCreateModal.value = false
}

const handleCreateSuccess = () => {
  showCreateModal.value = false
  fetchData()
}

const displayToast = (message: string, type: 'success' | 'error' | 'info' = 'info') => {
  if (toastTimer) {
    clearTimeout(toastTimer)
  }
  toastMessage.value = message
  toastType.value = type
  showToast.value = true
  toastTimer = setTimeout(() => {
    showToast.value = false
  }, 3000)
}

const closeOverlays = () => {
  openMenuId.value = null
  menuTargetItem.value = null
  menuPosition.value = null
}

const onGlobalClick = () => closeOverlays()

const onStopPropagation = (e: MouseEvent) => e.stopPropagation()

const openPickupMenu = (item: PickupCodeItem, event: MouseEvent) => {
  const menuId = String(item.id)
  if (openMenuId.value === menuId) {
    closeOverlays()
    return
  }
  const button = event.currentTarget as HTMLButtonElement
  const rect = button.getBoundingClientRect()
  const menuHeight = 180
  const menuWidth = 192
  const padding = 8
  const spaceBelow = window.innerHeight - rect.bottom
  const spaceAbove = rect.top

  let top = rect.bottom + padding
  if (spaceBelow < menuHeight && spaceAbove > spaceBelow) {
    top = rect.top - menuHeight - padding
  }

  let left = rect.right - menuWidth
  if (left < padding) left = padding

  menuPosition.value = { top, left }
  openMenuId.value = menuId
  menuTargetItem.value = item
}

const handleCopyCode = async (code: string) => {
  if (!code) {
    displayToast('复制失败，请稍后重试', 'error')
    return
  }
  try {
    await navigator.clipboard.writeText(code)
    displayToast(`取件码 ${code} 已复制`, 'success')
    closeOverlays()
  } catch {
    displayToast('复制失败，请手动复制', 'error')
  }
}

const handleViewDetail = (item: PickupCodeItem) => {
  selectedItem.value = {
    ...item,
    name: sanitizeFileName(item.name),
  }
  showDetailModal.value = true
}

const handleViewDetailFromMenu = () => {
  if (!menuTargetItem.value) return
  handleViewDetail(menuTargetItem.value)
  closeOverlays()
}

const handleDeleteFromMenu = async () => {
  if (!menuTargetItem.value) return
  deleteTargetItem.value = menuTargetItem.value
  showDeleteConfirm.value = true
  closeOverlays()
}

const closeDeleteConfirm = () => {
  if (deletingCodeId.value !== null) return
  showDeleteConfirm.value = false
  deleteTargetItem.value = null
}

const confirmDeletePickupCode = async () => {
  if (!deleteTargetItem.value) return
  const target = deleteTargetItem.value
  deletingCodeId.value = target.id
  try {
    await deletePickupCode(target.id)
    displayToast('取件码删除成功', 'success')
    showDeleteConfirm.value = false
    deleteTargetItem.value = null
    await fetchData()
  } catch (error: unknown) {
    displayToast(error instanceof Error ? error.message : '删除取件码失败', 'error')
  } finally {
    deletingCodeId.value = null
  }
}

const handleCloseDetail = () => {
  showDetailModal.value = false
  selectedItem.value = null
}

onMounted(() => {
  document.addEventListener('click', onGlobalClick)
  fetchData()
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onGlobalClick)
  if (toastTimer) {
    clearTimeout(toastTimer)
  }
})
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col bg-canvas text-label">
    <main class="flex min-w-0 flex-1 flex-col overflow-hidden">
      <div class="flex-1 space-y-6 overflow-y-auto p-4 sm:p-6 lg:space-y-8 lg:p-8">
        <!-- 页头 -->
        <div class="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
          <div class="space-y-1">
            <h2 class="text-title-1">取件码管理</h2>
            <p class="text-subhead text-label-secondary">创建、监控并撤销安全的文件提取码。</p>
          </div>
          <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row sm:items-center">
            <button
              class="flex h-10 w-full items-center justify-center gap-2 rounded-full bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97] sm:w-auto"
              type="button"
              aria-label="去取件页面"
              @click="goToFilePickup"
            >
              <AppIcon
                icon="material-symbols:download-rounded"
                class="text-[18px]"
                aria-hidden="true"
              />
              <span>去取件</span>
            </button>
            <button
              class="flex h-10 w-full items-center justify-center gap-2 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed sm:w-auto"
              type="button"
              aria-label="创建新取件码"
              @click="openCreateModal"
            >
              <AppIcon icon="material-symbols:add-rounded" class="text-[18px]" aria-hidden="true" />
              <span>创建新取件码</span>
            </button>
          </div>
        </div>

        <!-- 统计卡 -->
        <div class="grid grid-cols-1 gap-5 md:grid-cols-3">
          <div class="flex flex-col gap-3.5 rounded-md bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span
                class="text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                >活跃取件码</span
              >
              <span
                class="flex h-8 w-8 items-center justify-center rounded-sm bg-primary-tint text-primary"
              >
                <AppIcon icon="material-symbols:lock-open-outline-rounded" class="text-[18px]" />
              </span>
            </div>
            <div class="flex items-baseline gap-2">
              <p class="text-title-1 text-label">{{ stats.activeCount }}</p>
              <span
                class="rounded-full bg-primary-tint px-2.5 py-0.5 text-caption font-semibold text-primary"
                >↑ 12%</span
              >
            </div>
          </div>

          <div class="flex flex-col gap-3.5 rounded-md bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span
                class="text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                >总下载量</span
              >
              <span
                class="flex h-8 w-8 items-center justify-center rounded-sm bg-info-tint text-info"
              >
                <AppIcon icon="material-symbols:download-rounded" class="text-[18px]" />
              </span>
            </div>
            <div class="flex items-baseline gap-2">
              <p class="text-title-1 text-label">{{ stats.totalDownloads.toLocaleString() }}</p>
              <span
                class="rounded-full bg-info-tint px-2.5 py-0.5 text-caption font-semibold text-info"
                >↑ 5%</span
              >
            </div>
          </div>

          <div class="flex flex-col gap-3.5 rounded-md bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span
                class="text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                >即将过期取件码</span
              >
              <span
                class="flex h-8 w-8 items-center justify-center rounded-sm bg-warning-tint text-warning"
              >
                <AppIcon icon="material-symbols:timer-outline-rounded" class="text-[18px]" />
              </span>
            </div>
            <div class="flex items-baseline gap-2">
              <p class="text-title-1 text-label">{{ stats.expiringSoon }}</p>
              <span
                class="rounded-full bg-surface-tertiary px-2.5 py-0.5 text-caption font-medium text-label-secondary"
                >未来 7 天</span
              >
            </div>
          </div>
        </div>

        <!-- 取件码仓库表 -->
        <div class="overflow-hidden rounded-md bg-surface shadow-card">
          <div
            class="flex items-center justify-between gap-3 border-b border-hairline px-4 py-3.5 sm:px-5"
          >
            <h3 class="text-headline text-label">活跃取件码仓库</h3>
            <div class="flex items-center gap-1">
              <button
                class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="筛选"
              >
                <AppIcon
                  icon="material-symbols:filter-list-rounded"
                  class="text-[20px]"
                  aria-hidden="true"
                />
              </button>
              <button
                class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="排序"
              >
                <AppIcon
                  icon="material-symbols:sort-rounded"
                  class="text-[20px]"
                  aria-hidden="true"
                />
              </button>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full min-w-[820px] border-collapse text-left">
              <thead>
                <tr>
                  <th
                    scope="col"
                    class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                  >
                    取件码
                  </th>
                  <th
                    scope="col"
                    class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                  >
                    关联文件
                  </th>
                  <th
                    scope="col"
                    class="hidden whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary sm:table-cell"
                  >
                    使用进度
                  </th>
                  <th
                    scope="col"
                    class="hidden whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary md:table-cell"
                  >
                    下载次数
                  </th>
                  <th
                    scope="col"
                    class="hidden whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary lg:table-cell"
                  >
                    过期时间
                  </th>
                  <th
                    scope="col"
                    class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                  >
                    状态
                  </th>
                  <th
                    scope="col"
                    class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-right text-caption font-semibold uppercase tracking-[0.06em] text-label-secondary"
                  >
                    操作
                  </th>
                </tr>
              </thead>

              <tbody>
                <tr v-if="loading">
                  <td colspan="7" class="px-6 py-10">
                    <div class="flex flex-col items-center justify-center gap-2.5">
                      <AppIcon
                        icon="material-symbols:progress-activity"
                        class="animate-spin text-[24px] text-primary"
                      />
                      <p class="text-subhead text-label-tertiary">加载中…</p>
                    </div>
                  </td>
                </tr>
                <tr v-else-if="pickupList.length === 0">
                  <td colspan="7" class="px-6 py-10 text-center text-subhead text-label-tertiary">
                    暂无取件码，点击右上角「创建新取件码」开始
                  </td>
                </tr>
                <tr
                  v-else
                  v-for="item in pickupList"
                  :key="item.code"
                  class="border-b border-hairline transition-colors duration-150 last:border-b-0 hover:bg-surface-secondary"
                >
                  <td class="px-4 py-3.5">
                    <div
                      class="inline-block rounded-sm border border-hairline bg-surface-secondary px-2.5 py-1 font-mono text-[15px] font-bold tracking-[0.14em] text-label"
                    >
                      {{ item.code }}
                    </div>
                  </td>
                  <td class="px-4 py-3.5">
                    <div class="flex min-w-0 items-center gap-3">
                      <span
                        class="flex h-8 w-8 flex-none items-center justify-center rounded-sm"
                        :class="
                          item.type === 'folder'
                            ? 'bg-primary-tint text-primary'
                            : 'bg-warning-tint text-warning'
                        "
                      >
                        <AppIcon :icon="typeIcon(item.type)" class="text-[16px]" />
                      </span>
                      <div class="min-w-0">
                        <p class="max-w-[180px] truncate text-[15px] font-semibold text-label">
                          {{ item.name }}
                        </p>
                        <p class="truncate text-caption text-label-secondary">
                          {{ typeLabel(item.type) }}
                        </p>
                      </div>
                    </div>
                  </td>
                  <td class="hidden px-4 py-3.5 sm:table-cell">
                    <div class="h-1.5 w-32 overflow-hidden rounded-full bg-surface-tertiary">
                      <div
                        class="h-full rounded-full"
                        :class="item.status === 'Active' ? 'bg-primary' : 'bg-danger'"
                        :style="{ width: `${progressPercent(item)}%` }"
                      ></div>
                    </div>
                  </td>
                  <td
                    class="hidden whitespace-nowrap px-4 py-3.5 text-subhead text-label-secondary md:table-cell"
                  >
                    {{ item.download }} / {{ item.max_download }}
                  </td>
                  <td
                    class="hidden whitespace-nowrap px-4 py-3.5 text-subhead text-label-secondary lg:table-cell"
                  >
                    {{ formatDate(item.expire_time) }}
                  </td>
                  <td class="px-4 py-3.5">
                    <span
                      class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-caption font-semibold"
                      :class="
                        item.status === 'Active'
                          ? 'bg-primary-tint text-primary'
                          : 'bg-danger-tint text-danger'
                      "
                    >
                      <span class="size-1.5 rounded-full bg-current"></span>
                      {{ statusLabel(item.status) }}
                    </span>
                  </td>
                  <td class="px-4 py-3.5 text-right" @click="onStopPropagation">
                    <button
                      class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-label"
                      type="button"
                      aria-label="取件码操作菜单"
                      @click="e => openPickupMenu(item, e)"
                    >
                      <AppIcon icon="material-symbols:more-vert" aria-hidden="true" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 分页条（FilePagination 视觉：32px 圆 pill，当前页 primary-tint） -->
          <div
            class="flex flex-col gap-3 border-t border-hairline px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5"
          >
            <p class="text-caption text-label-secondary">
              第
              <span class="font-bold text-label">{{ startIndex }}-{{ endIndex }}</span>
              项，共
              <span class="font-bold text-label">{{ totalCount }}</span>
              项
            </p>
            <div class="flex flex-wrap items-center gap-1">
              <button
                class="flex h-8 min-w-8 items-center justify-center rounded-full px-1.5 text-[13px] font-semibold text-label-secondary transition-[background-color,color] duration-150 hover:bg-surface-secondary hover:text-label disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
                type="button"
                aria-label="上一页"
                :disabled="currentPage <= 1 || loading"
                @click="handlePageChange(currentPage - 1)"
              >
                <AppIcon
                  class="text-[16px]"
                  icon="material-symbols:chevron-left"
                  aria-hidden="true"
                />
              </button>
              <button
                v-for="p in pageNumbers"
                :key="p"
                class="flex h-8 min-w-8 items-center justify-center rounded-full px-1.5 text-[13px] font-semibold transition-[background-color,color] duration-150 disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
                :class="
                  p === currentPage
                    ? 'bg-primary-tint text-primary'
                    : 'text-label-secondary hover:bg-surface-secondary hover:text-label'
                "
                type="button"
                :aria-label="`第 ${p} 页`"
                :aria-current="p === currentPage ? 'page' : undefined"
                :disabled="loading"
                @click="handlePageChange(p)"
              >
                {{ p }}
              </button>
              <span v-if="totalPages > 3" class="px-1 text-[13px] text-label-tertiary">…</span>
              <button
                class="flex h-8 min-w-8 items-center justify-center rounded-full px-1.5 text-[13px] font-semibold text-label-secondary transition-[background-color,color] duration-150 hover:bg-surface-secondary hover:text-label disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
                type="button"
                aria-label="下一页"
                :disabled="currentPage >= totalPages || loading"
                @click="handlePageChange(currentPage + 1)"
              >
                <AppIcon
                  class="text-[16px]"
                  icon="material-symbols:chevron-right"
                  aria-hidden="true"
                />
              </button>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- 创建取件码弹窗（标准弹窗遮罩：scrim + blur；内容组件保持原样） -->
    <Teleport to="body">
      <AnimatePresence>
        <Motion
          v-if="showCreateModal"
          key="create-modal-overlay"
          :initial="{ opacity: 0 }"
          :animate="{ opacity: 1 }"
          :exit="{ opacity: 0 }"
          :transition="fadeTransition"
          class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4 backdrop-blur-sm"
          @click.self="closeCreateModal"
        >
          <!-- 面板 Motion 内联（与 ConfirmDialog 等弹窗同构；子组件保持纯内容） -->
          <Motion
            :initial="{ opacity: 0, scale: 0.96 }"
            :animate="{ opacity: 1, scale: 1 }"
            :exit="{ opacity: 0, scale: 0.98 }"
            :transition="springSnappy"
            class="mx-4 w-full max-w-[520px]"
          >
            <CreatePickupCodeModal @close="closeCreateModal" @success="handleCreateSuccess" />
          </Motion>
        </Motion>
      </AnimatePresence>
    </Teleport>

    <!-- 详情弹窗（标准弹窗模式：遮罩淡入 + 面板弹簧 scale 0.96→1，motion-v） -->
    <Teleport to="body">
      <AnimatePresence>
        <Motion
          v-if="showDetailModal && selectedItem"
          key="detail-modal-overlay"
          :initial="{ opacity: 0 }"
          :animate="{ opacity: 1 }"
          :exit="{ opacity: 0 }"
          :transition="fadeTransition"
          class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          @click.self="handleCloseDetail"
        >
          <Motion
            :initial="{ opacity: 0, scale: 0.96 }"
            :animate="{ opacity: 1, scale: 1 }"
            :exit="{ opacity: 0, scale: 0.98 }"
            :transition="springSnappy"
            class="w-full max-w-md overflow-hidden rounded-lg bg-surface shadow-popover"
          >
            <div class="flex items-center justify-between border-b border-hairline px-5 py-4">
              <h3 class="text-title-3">取件码详情</h3>
              <button
                class="flex h-9 w-9 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="关闭详情"
                @click="handleCloseDetail"
              >
                <AppIcon
                  icon="material-symbols:close-rounded"
                  class="text-[20px]"
                  aria-hidden="true"
                />
              </button>
            </div>

            <div class="space-y-5 p-5">
              <div class="flex justify-center">
                <div
                  class="rounded-sm border border-hairline bg-surface-secondary px-5 py-3 font-mono text-[20px] font-bold tracking-[0.3em] text-label"
                >
                  {{ selectedItem.code }}
                </div>
              </div>

              <div class="space-y-3">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-subhead text-label-secondary">文件名</span>
                  <span class="truncate text-[15px] font-medium text-label">{{
                    selectedItem.name
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-subhead text-label-secondary">类型</span>
                  <span class="text-[15px] font-medium text-label">{{
                    typeLabel(selectedItem.type)
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-subhead text-label-secondary">下载次数</span>
                  <span class="text-[15px] font-medium text-label"
                    >{{ selectedItem.download }} / {{ selectedItem.max_download }}</span
                  >
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-subhead text-label-secondary">过期时间</span>
                  <span class="text-[15px] font-medium text-label">{{
                    formatDate(selectedItem.expire_time)
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-subhead text-label-secondary">状态</span>
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-caption font-semibold"
                    :class="
                      selectedItem.status === 'Active'
                        ? 'bg-primary-tint text-primary'
                        : 'bg-danger-tint text-danger'
                    "
                  >
                    <span class="size-1.5 rounded-full bg-current"></span>
                    {{ statusLabel(selectedItem.status) }}
                  </span>
                </div>
              </div>
            </div>

            <div class="flex justify-end gap-3 border-t border-hairline px-5 py-4">
              <button
                type="button"
                class="h-10 rounded-full bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97]"
                aria-label="关闭详情"
                @click="handleCloseDetail"
              >
                关闭
              </button>
              <button
                type="button"
                class="flex h-10 items-center gap-2 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
                aria-label="复制取件码"
                @click="handleCopyCode(selectedItem.code)"
              >
                <AppIcon
                  class="text-[16px]"
                  icon="material-symbols:content-copy"
                  aria-hidden="true"
                />
                复制取件码
              </button>
            </div>
          </Motion>
        </Motion>
      </AnimatePresence>
    </Teleport>

    <ConfirmDialog
      v-model="showDeleteConfirm"
      title="确认删除取件码"
      :message="`将删除取件码「${deleteTargetItem?.code || '-'}」，此操作不可撤销。`"
      confirm-text="确认删除"
      cancel-text="取消"
      :loading="deletingCodeId !== null"
      :danger="true"
      @cancel="closeDeleteConfirm"
      @confirm="confirmDeletePickupCode"
    >
      <template #confirm-icon>
        <AppIcon
          v-if="deletingCodeId !== null"
          icon="material-symbols:progress-activity"
          class="animate-spin"
        />
      </template>
    </ConfirmDialog>

    <!-- Toast（顶部居中胶囊，material-thick；非阻塞不加遮罩；motion-v 弹簧） -->
    <AnimatePresence>
      <Motion
        v-if="showToast"
        key="pickup-toast"
        :initial="{ opacity: 0, y: -8 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0, y: -8 }"
        :transition="springSnappy"
        class="fixed left-1/2 top-4 z-[60] flex h-10 -translate-x-1/2 items-center gap-2 rounded-full bg-material-thick px-4 shadow-floating backdrop-blur-[20px] backdrop-saturate-[180%]"
      >
        <AppIcon
          v-if="toastType === 'success'"
          icon="material-symbols:check-circle"
          class="text-[18px] text-primary"
        />
        <AppIcon
          v-else-if="toastType === 'error'"
          icon="material-symbols:error"
          class="text-[18px] text-danger"
        />
        <AppIcon v-else icon="material-symbols:info" class="text-[18px] text-info" />
        <span class="text-sm font-medium text-label">{{ toastMessage }}</span>
      </Motion>
    </AnimatePresence>

    <!-- 行内操作菜单（FileActionMenu 模式：从触发源弹簧 scale 0.95→1，danger 置底） -->
    <Teleport to="body">
      <AnimatePresence>
        <Motion
          v-if="openMenuId && menuPosition"
          key="pickup-row-menu"
          :initial="{ opacity: 0, scale: 0.95 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.95 }"
          :transition="springSnappy"
          class="fixed z-50 w-48 origin-top-right rounded-md bg-surface p-1.5 shadow-popover"
          :style="{ top: `${menuPosition.top}px`, left: `${menuPosition.left}px` }"
          @click="onStopPropagation"
        >
          <button
            class="flex h-10 w-full items-center gap-2.5 rounded-sm px-3 text-left text-[15px] text-label transition-colors duration-150 hover:bg-surface-secondary focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="复制取件码"
            @click="handleCopyCode(menuTargetItem?.code || '')"
          >
            <AppIcon
              class="text-[18px] text-label-secondary"
              icon="material-symbols:content-copy"
              aria-hidden="true"
            />
            复制取件码
          </button>
          <button
            class="flex h-10 w-full items-center gap-2.5 rounded-sm px-3 text-left text-[15px] text-label transition-colors duration-150 hover:bg-surface-secondary focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="查看取件码详情"
            @click="handleViewDetailFromMenu"
          >
            <AppIcon
              class="text-[18px] text-label-secondary"
              icon="material-symbols:visibility"
              aria-hidden="true"
            />
            查看详情
          </button>
          <div class="mx-2.5 my-1 h-px bg-hairline"></div>
          <button
            class="flex h-10 w-full items-center gap-2.5 rounded-sm px-3 text-left text-[15px] text-danger transition-colors duration-150 hover:bg-danger-tint focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="删除取件码"
            :disabled="deletingCodeId !== null"
            @click="handleDeleteFromMenu"
          >
            <AppIcon
              class="text-[18px]"
              :icon="
                deletingCodeId !== null
                  ? 'material-symbols:progress-activity'
                  : 'material-symbols:delete'
              "
              :class="deletingCodeId !== null ? 'animate-spin' : ''"
              aria-hidden="true"
            />
            删除
          </button>
        </Motion>
      </AnimatePresence>
    </Teleport>
  </div>
</template>

<style lang="sass" scoped></style>
