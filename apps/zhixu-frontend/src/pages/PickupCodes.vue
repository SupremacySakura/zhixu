<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import AppBadge from '../components/ui/AppBadge.vue'
import AppButton from '../components/ui/AppButton.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import AppLoading from '../components/ui/AppLoading.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../utils/motion'
import { getPickupCodeList, getPickupCodeCount, deletePickupCode } from '../services/apis/file'
import type { PickupCodeItem, PickupCodeStatus, PickupCodeType } from '../services/types/file'
import CreatePickupCodeModal from '../components/bussiness/CreatePickupCodeModal.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import PickupCodeCardList from '../components/file-management/PickupCodeCardList.vue'
import { sanitizeFileName } from '../utils/file'
import { pickupCodeStats } from '../utils/pickup-codes'
import { useToast } from '../composables/useToast'

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

// 消息提示统一走外壳宿主：本页不再自持 toast 状态，也不再画胶囊。
const { displayToast } = useToast()

const currentPage = ref(1)
const pageSize = ref(10)
const totalCount = ref(0)

// 统计口径：总数来自 /code/count（账号级），其余三项只能诚实地限定在「本页」。
// 后端只有 SELECT count(*) WHERE user_id=? 一个聚合接口，没有活跃数、没有总下载量、
// 没有即将过期数 —— 所以卡面标题必须写清限定语，不能把本页 10 条冒充账号级数字。
const stats = computed(() => pickupCodeStats(pickupList.value, totalCount.value))

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

const statusLabel = (status: PickupCodeStatus) => {
  return status === 'Active' ? '生效中' : '已过期'
}

// 状态标记是「方块点 + 文字」，不是彩色胶囊：形状交给 atom-badge-dot，
// 这里只负责把状态映射到语气。
const statusTone = (status: PickupCodeStatus) => (status === 'Active' ? 'primary' : 'danger')

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
})
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col bg-canvas text-label">
    <!-- 页头带 64px：索引列表原型。左标题 + 等宽计数行，右 1 主 + 1 次操作。 -->
    <header class="shell-band">
      <div class="min-w-0 flex-1">
        <h1 class="truncate text-title">取件码管理</h1>
        <p class="truncate font-type text-micro text-label-tertiary">
          创建、监控并撤销安全的文件提取码
        </p>
      </div>
      <div class="flex flex-none items-center gap-2">
        <button
          class="shell-band-action"
          type="button"
          aria-label="去取件页面"
          title="去取件"
          @click="goToFilePickup"
        >
          <AppIcon
            icon="material-symbols:download-rounded"
            class="atom-glyph"
            aria-hidden="true"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          <span class="hidden sm:inline">去取件</span>
        </button>
        <button
          class="shell-band-action-primary"
          type="button"
          aria-label="创建新取件码"
          title="创建新取件码"
          @click="openCreateModal"
        >
          <AppIcon
            icon="material-symbols:add-rounded"
            class="atom-glyph"
            aria-hidden="true"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          <span class="hidden sm:inline">创建新取件码</span>
        </button>
      </div>
    </header>

    <main class="shell-page">
      <div class="shell-wide space-y-6 lg:space-y-8">
        <!-- 统计卡。三张卡的口径各不相同，所以限定语必须写在卡面上：
             第一张是账号级总数（/code/count），后两张只有本页数据可算。
             原本第一张写「活跃取件码」、第二张写「总下载量」，数字却数的是
             当前页 10 条 —— 翻到第 2 页就从 10 掉到 1。这里把标题改成
             各自诚实的名字，而不是留着标题去迁就假数字。 -->
        <div class="grid grid-cols-1 gap-5 md:grid-cols-3">
          <div class="flex flex-col gap-3.5 bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span class="text-micro font-normal uppercase text-label-tertiary">取件码总数</span>
              <span class="flex h-8 w-8 items-center justify-center bg-primary-tint text-primary">
                <AppIcon
                  icon="material-symbols:lock-open-outline-rounded"
                  class="atom-glyph"
                  :style="{ '--atom-glyph-size': '18px' }"
                />
              </span>
            </div>
            <p class="text-display text-label">{{ stats.total }}</p>
          </div>

          <div class="flex flex-col gap-3.5 bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span class="text-micro font-normal uppercase text-label-tertiary">本页下载量</span>
              <span class="flex h-8 w-8 items-center justify-center bg-info-tint text-info">
                <AppIcon
                  icon="material-symbols:download-rounded"
                  class="atom-glyph"
                  :style="{ '--atom-glyph-size': '18px' }"
                />
              </span>
            </div>
            <p class="text-display text-label">{{ stats.pageDownloads.toLocaleString() }}</p>
          </div>

          <div class="flex flex-col gap-3.5 bg-surface p-5 shadow-card">
            <div class="flex items-center justify-between">
              <span class="text-micro font-normal uppercase text-label-tertiary">本页即将过期</span>
              <span class="flex h-8 w-8 items-center justify-center bg-warning-tint text-warning">
                <AppIcon
                  icon="material-symbols:timer-outline-rounded"
                  class="atom-glyph"
                  :style="{ '--atom-glyph-size': '18px' }"
                />
              </span>
            </div>
            <div class="flex items-baseline gap-2">
              <p class="text-display text-label">{{ stats.pageExpiringSoon }}</p>
              <!-- 「未来 7 天」是真的：它交代的是窗口，不是编出来的趋势。 -->
              <span class="bg-surface-tertiary px-2 py-0.5 text-note text-label-secondary"
                >未来 7 天</span
              >
            </div>
          </div>
        </div>

        <!-- 取件码仓库表 -->
        <div class="overflow-hidden bg-surface shadow-card">
          <div
            class="flex items-center justify-between gap-3 border-b border-hairline px-4 py-3.5 sm:px-5"
          >
            <h3 class="text-lead text-label">活跃取件码仓库</h3>
            <div class="flex items-center gap-1 pointer-coarse:gap-3">
              <button
                class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="筛选"
              >
                <AppIcon
                  icon="material-symbols:filter-list-rounded"
                  class="atom-glyph"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '20px' }"
                />
              </button>
              <button
                class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="排序"
              >
                <AppIcon
                  icon="material-symbols:sort-rounded"
                  class="atom-glyph"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '20px' }"
                />
              </button>
            </div>
          </div>

          <!-- 加载态与空态只写一份，两种形态共用：中文文案不该有第二份副本。 -->
          <div v-if="loading" class="flex flex-col items-center justify-center gap-2.5 px-6 py-10">
            <AppLoading :size="24" class="text-primary" />
            <p class="text-note text-label-tertiary">加载中…</p>
          </div>
          <p
            v-else-if="pickupList.length === 0"
            class="px-6 py-10 text-center text-note text-label-tertiary"
          >
            暂无取件码，点击右上角「创建新取件码」开始
          </p>

          <template v-else>
            <!-- ≥lg：七列表格。窄屏下这张表只能靠横向滚动才装得下，
                 而 DESIGN.md:514 明确「表格变两列卡列表，不做横向滚动」，
                 所以表格只在 lg 及以上出现。 -->
            <div class="hidden lg:block">
              <table class="w-full border-collapse text-left">
                <thead>
                  <tr>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      取件码
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      关联文件
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      使用进度
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      下载次数
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      过期时间
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-micro font-normal uppercase text-label-tertiary"
                    >
                      状态
                    </th>
                    <th
                      scope="col"
                      class="whitespace-nowrap border-b border-hairline bg-surface-secondary px-4 py-3 text-right text-micro font-normal uppercase text-label-tertiary"
                    >
                      操作
                    </th>
                  </tr>
                </thead>

                <tbody>
                  <tr
                    v-for="item in pickupList"
                    :key="item.code"
                    class="border-b border-hairline transition-colors last:border-b-0 hover:bg-surface-secondary"
                  >
                    <td class="px-4 py-3.5">
                      <div
                        class="inline-block border border-hairline bg-surface-secondary px-2.5 py-1 font-mono text-lead font-bold tracking-[0.14em] text-label"
                      >
                        {{ item.code }}
                      </div>
                    </td>
                    <td class="px-4 py-3.5">
                      <div class="flex min-w-0 items-center gap-3">
                        <span
                          class="flex h-8 w-8 flex-none items-center justify-center"
                          :class="
                            item.type === 'folder'
                              ? 'bg-primary-tint text-primary'
                              : 'bg-warning-tint text-warning'
                          "
                        >
                          <AppIcon
                            :icon="typeIcon(item.type)"
                            class="atom-glyph"
                            :style="{ '--atom-glyph-size': '16px' }"
                          />
                        </span>
                        <div class="min-w-0">
                          <p class="max-w-[180px] truncate text-lead text-label">
                            {{ item.name }}
                          </p>
                          <p class="truncate text-note text-label-secondary">
                            {{ typeLabel(item.type) }}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td class="px-4 py-3.5">
                      <div class="h-1.5 w-32 overflow-hidden bg-surface-tertiary">
                        <div
                          class="h-full"
                          :class="item.status === 'Active' ? 'bg-primary' : 'bg-danger'"
                          :style="{ width: `${progressPercent(item)}%` }"
                        ></div>
                      </div>
                    </td>
                    <td class="whitespace-nowrap px-4 py-3.5 text-note text-label-secondary">
                      {{ item.download }} / {{ item.max_download }}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3.5 text-note text-label-secondary">
                      {{ formatDate(item.expire_time) }}
                    </td>
                    <td class="px-4 py-3.5">
                      <AppBadge :tone="statusTone(item.status)" size="sm" dot>{{
                        statusLabel(item.status)
                      }}</AppBadge>
                    </td>
                    <td class="px-4 py-3.5 text-right" @click="onStopPropagation">
                      <button
                        class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
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

            <!-- <lg：两列卡片列表，零横向滚动 -->
            <div class="lg:hidden">
              <PickupCodeCardList :items="pickupList" @open-menu="openPickupMenu" />
            </div>
          </template>

          <!-- 分页条（FilePagination 视觉：32px 圆 pill，当前页 primary-tint） -->
          <div
            class="flex flex-col gap-3 border-t border-hairline px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5"
          >
            <p class="text-note text-label-secondary">
              第
              <span class="font-bold text-label">{{ startIndex }}-{{ endIndex }}</span>
              项，共
              <span class="font-bold text-label">{{ totalCount }}</span>
              项
            </p>
            <div class="flex flex-wrap items-center gap-1 pointer-coarse:gap-3">
              <button
                class="flex h-8 min-w-8 items-center justify-center px-1.5 text-body font-semibold text-label-secondary transition-[background-color,color] tap-target hover:bg-surface-secondary hover:text-label disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
                type="button"
                aria-label="上一页"
                :disabled="currentPage <= 1 || loading"
                @click="handlePageChange(currentPage - 1)"
              >
                <AppIcon
                  class="atom-glyph"
                  icon="material-symbols:chevron-left"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '16px' }"
                />
              </button>
              <button
                v-for="p in pageNumbers"
                :key="p"
                class="flex h-8 min-w-8 items-center justify-center px-1.5 text-body font-semibold transition-[background-color,color] disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
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
              <span v-if="totalPages > 3" class="px-1 text-body text-label-tertiary">…</span>
              <button
                class="flex h-8 min-w-8 items-center justify-center px-1.5 text-body font-semibold text-label-secondary transition-[background-color,color] tap-target hover:bg-surface-secondary hover:text-label disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent"
                type="button"
                aria-label="下一页"
                :disabled="currentPage >= totalPages || loading"
                @click="handlePageChange(currentPage + 1)"
              >
                <AppIcon
                  class="atom-glyph"
                  icon="material-symbols:chevron-right"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '16px' }"
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
          class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
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
          class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
          role="dialog"
          aria-modal="true"
          @click.self="handleCloseDetail"
        >
          <Motion
            :initial="{ opacity: 0, scale: 0.96 }"
            :animate="{ opacity: 1, scale: 1 }"
            :exit="{ opacity: 0, scale: 0.98 }"
            :transition="springSnappy"
            class="w-full max-w-md overflow-hidden bg-surface shadow-popover"
          >
            <div class="flex items-center justify-between border-b border-hairline px-5 py-4">
              <h3 class="text-title">取件码详情</h3>
              <button
                class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-tertiary hover:text-label"
                type="button"
                aria-label="关闭详情"
                @click="handleCloseDetail"
              >
                <AppIcon
                  icon="material-symbols:close-rounded"
                  class="atom-glyph"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '20px' }"
                />
              </button>
            </div>

            <div class="space-y-5 p-5">
              <div class="flex justify-center">
                <div
                  class="border border-hairline bg-surface-secondary px-5 py-3 font-mono text-title font-bold tracking-[0.3em] text-label"
                >
                  {{ selectedItem.code }}
                </div>
              </div>

              <div class="space-y-3">
                <div class="flex items-center justify-between gap-3">
                  <span class="text-note text-label-secondary">文件名</span>
                  <span class="truncate text-lead font-medium text-label">{{
                    selectedItem.name
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-note text-label-secondary">类型</span>
                  <span class="text-lead font-medium text-label">{{
                    typeLabel(selectedItem.type)
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-note text-label-secondary">下载次数</span>
                  <span class="text-lead font-medium text-label"
                    >{{ selectedItem.download }} / {{ selectedItem.max_download }}</span
                  >
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-note text-label-secondary">过期时间</span>
                  <span class="text-lead font-medium text-label">{{
                    formatDate(selectedItem.expire_time)
                  }}</span>
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span class="text-note text-label-secondary">状态</span>
                  <AppBadge :tone="statusTone(selectedItem.status)" size="sm" dot>{{
                    statusLabel(selectedItem.status)
                  }}</AppBadge>
                </div>
              </div>
            </div>

            <div class="flex justify-end gap-3 border-t border-hairline px-5 py-4">
              <AppButton aria-label="关闭详情" @click="handleCloseDetail">关闭</AppButton>
              <AppButton
                tone="primary"
                icon="material-symbols:content-copy"
                aria-label="复制取件码"
                @click="handleCopyCode(selectedItem.code)"
              >
                复制取件码
              </AppButton>
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
        <AppLoading v-if="deletingCodeId !== null" size="sm" />
      </template>
    </ConfirmDialog>

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
          class="fixed z-50 w-48 origin-top-right bg-surface p-1.5 shadow-popover"
          :style="{ top: `${menuPosition.top}px`, left: `${menuPosition.left}px` }"
          @click="onStopPropagation"
        >
          <button
            class="flex h-10 w-full items-center gap-2.5 px-3 text-left text-lead font-normal text-label transition-colors hover:bg-surface-secondary disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="复制取件码"
            @click="handleCopyCode(menuTargetItem?.code || '')"
          >
            <AppIcon
              class="atom-glyph text-label-secondary"
              icon="material-symbols:content-copy"
              aria-hidden="true"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            复制取件码
          </button>
          <button
            class="flex h-10 w-full items-center gap-2.5 px-3 text-left text-lead font-normal text-label transition-colors hover:bg-surface-secondary disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="查看取件码详情"
            @click="handleViewDetailFromMenu"
          >
            <AppIcon
              class="atom-glyph text-label-secondary"
              icon="material-symbols:visibility"
              aria-hidden="true"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            查看详情
          </button>
          <div class="mx-2.5 my-1 h-px bg-hairline"></div>
          <button
            class="flex h-10 w-full items-center gap-2.5 px-3 text-left text-lead font-normal text-danger transition-colors hover:bg-danger-tint disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            aria-label="删除取件码"
            :disabled="deletingCodeId !== null"
            @click="handleDeleteFromMenu"
          >
            <AppLoading v-if="deletingCodeId !== null" :size="18" />
            <AppIcon
              v-else
              class="atom-glyph"
              icon="material-symbols:delete"
              aria-hidden="true"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            删除
          </button>
        </Motion>
      </AnimatePresence>
    </Teleport>
  </div>
</template>

<style lang="sass" scoped></style>
