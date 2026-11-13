<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import AppIcon from '../components/ui/AppIcon.vue'
import { useUserStore } from '../stores/user'
import { useOperationLogs } from '../composables/useOperationLogs'
import LoginRequiredPlaceholder from '../components/bussiness/LoginRequiredPlaceholder.vue'
import { getDashboardOverview } from '../services/apis/file'
import { getEvolutionSummary } from '../services/apis/evolution'
import type { EvolutionSummary } from '../services/apis/evolution'
import type { DashboardFileStatItem } from '../services/types/file'
import { formatBytes } from '../utils/file'
import { describeOperationLog } from '../utils/operationLog'

const userStore = useUserStore()

type FileStatCard = {
  type: string
  title: string
  count: string
  size: string
  icon: string
  colorClass: string
}

// 演化提醒只取计数（summary 接口），首页不拉提醒列表，也不引图表库
const evolutionSummary = ref<EvolutionSummary | null>(null)
// 规格 #8：暂不处理不计入首页待处理数量，首页只取 pending；含 deferred 的总数在演化中心页头。
const evolutionPendingCount = computed(() => evolutionSummary.value?.pending ?? 0)
const evolutionTypeStats = computed(() => [
  { key: 'duplicate', label: '重复', count: evolutionSummary.value?.by_type.duplicate ?? 0 },
  { key: 'outdated', label: '过时', count: evolutionSummary.value?.by_type.outdated ?? 0 },
  { key: 'conflict', label: '冲突', count: evolutionSummary.value?.by_type.conflict ?? 0 },
])

const storageUsedPercent = ref(0)
const storageUsedBytes = ref(0)
const storageTotalBytes = ref(0)
const storageLeftBytes = ref(0)
const isLoading = ref(false)

// 操作日志 feed：数据源与分页由 composable 负责，页面只做展示编排
const {
  logs: operationLogs,
  total: operationLogTotal,
  isLoading: isOperationLogsLoading,
  isLoadingMore,
  hasMore,
  load: loadOperationLogs,
  loadMore: loadMoreOperationLogs,
  reset: resetOperationLogs,
} = useOperationLogs()

const storageUsedStr = computed(() => formatBytes(storageUsedBytes.value))
const storageTotalStr = computed(() => formatBytes(storageTotalBytes.value))
const storageLeftStr = computed(() => formatBytes(storageLeftBytes.value))

// 用量 ≥ 90% 转 warning 警示（DESIGN.md：warning 用于空间不足）
const isStorageWarning = computed(() => storageUsedPercent.value >= 90)

const fileStats = ref<FileStatCard[]>([])

const fileTypeMeta: Record<string, { title: string; icon: string; colorClass: string }> = {
  image: {
    title: '图片',
    icon: 'material-symbols:image-outline-rounded',
    colorClass: 'bg-info-tint text-info',
  },
  video: {
    title: '视频',
    icon: 'material-symbols:videocam-outline-rounded',
    colorClass: 'bg-pink-tint text-pink',
  },
  audio: {
    title: '音频',
    icon: 'material-symbols:music-note-rounded',
    colorClass: 'bg-purple-tint text-purple',
  },
  document: {
    title: '文档',
    icon: 'material-symbols:description-outline-rounded',
    colorClass: 'bg-warning-tint text-warning',
  },
  other: {
    title: '其他',
    icon: 'material-symbols:description-outline-rounded',
    colorClass: 'bg-surface-tertiary text-label-secondary',
  },
}

const dashboardTypeOrder = ['image', 'video', 'audio', 'document', 'other']

const mapFileStats = (stats: DashboardFileStatItem[]): FileStatCard[] => {
  const map = new Map(stats.map(item => [item.type, item]))
  return dashboardTypeOrder.map(type => {
    const item = map.get(type)
    const meta = fileTypeMeta[type] ?? fileTypeMeta.other
    return {
      type,
      title: meta.title,
      count: (item?.count ?? 0).toLocaleString('zh-CN'),
      size: formatBytes(item?.size ?? 0),
      icon: meta.icon,
      colorClass: meta.colorClass,
    }
  })
}

const formatRelativeTime = (value: string) => {
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return '-'
  const diff = Date.now() - timestamp
  const minute = 60 * 1000
  const hour = 60 * minute
  const day = 24 * hour
  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  return `${Math.floor(diff / day)} 天前`
}

// 操作日志行卡片：展示语义由 describeOperationLog 纯函数决定
const operationLogCards = computed(() =>
  operationLogs.value.map(log => ({
    id: log.id,
    time: formatRelativeTime(log.created_at),
    ...describeOperationLog(log),
  })),
)

const loadEvolutionSummary = async () => {
  try {
    evolutionSummary.value = await getEvolutionSummary()
  } catch (error) {
    // 提醒计数是首页的附加信息，取不到就退回空态，不影响存储与日志展示
    evolutionSummary.value = null
    console.error('获取演化提醒摘要失败', error)
  }
}

const resetDashboardData = () => {
  evolutionSummary.value = null
  storageUsedPercent.value = 0
  storageUsedBytes.value = 0
  storageTotalBytes.value = 0
  storageLeftBytes.value = 0
  fileStats.value = mapFileStats([])
  resetOperationLogs()
}

const loadDashboardData = async () => {
  isLoading.value = true
  try {
    const data = await getDashboardOverview()
    storageUsedPercent.value = Math.max(0, Math.min(100, data.storage_used_percent))
    storageUsedBytes.value = data.storage_used
    storageTotalBytes.value = data.storage_total
    storageLeftBytes.value = data.storage_left
    fileStats.value = mapFileStats(data.file_stats ?? [])
    void loadEvolutionSummary()
    void loadOperationLogs()
  } catch (error) {
    resetDashboardData()
    console.error('获取仪表盘数据失败', error)
  } finally {
    isLoading.value = false
  }
}

// Calculate SVG dash offset
const circumference = 364.4
const dashOffset = computed(() => circumference - (circumference * storageUsedPercent.value) / 100)

watch(
  () => userStore.isLoggedIn,
  isLoggedIn => {
    if (!isLoggedIn) {
      resetDashboardData()
      return
    }
    loadDashboardData()
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <LoginRequiredPlaceholder v-if="!userStore.isLoggedIn" />

    <!-- 页头带 64px：仪表盘是索引列表原型（DESIGN 原型 ①b），
         标题旁边挂的是存储用量这类等宽计数，不是操作按钮。 -->
    <header v-if="userStore.isLoggedIn" class="shell-band">
      <div class="min-w-0">
        <h1 class="text-title">仪表盘</h1>
        <p class="font-type text-micro text-label-tertiary">欢迎回来，这是您的存储情况。</p>
      </div>
    </header>

    <main v-if="userStore.isLoggedIn" class="shell-page">
      <div class="shell-wide space-y-6 lg:space-y-8">
        <!-- 待处理提醒：只取摘要计数，点击进演化中心逐条处理 -->
        <RouterLink
          to="/home/evolution"
          class="block bg-surface p-6 shadow-card transition-[transform,box-shadow] hover:-translate-y-0.5 hover:shadow-popover"
          data-test="evolution-card"
        >
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h3 class="text-title-3">待处理提醒</h3>
              <p class="mt-1 text-subhead text-label-secondary">
                重复、过时与冲突提醒集中在这里，逐条确认后才落地。
              </p>
            </div>
            <span class="text-subhead font-medium text-primary">前往演化中心 →</span>
          </div>

          <p v-if="evolutionPendingCount === 0" class="mt-4 text-subhead text-label-secondary">
            暂无待处理提醒
          </p>
          <div v-else class="mt-4 flex flex-wrap items-end gap-x-8 gap-y-3">
            <div>
              <p class="text-caption text-label-secondary">待处理</p>
              <p class="text-title-1 mt-0.5" data-test="evolution-pending-total">
                {{ evolutionPendingCount }}
              </p>
            </div>
            <div
              v-for="stat in evolutionTypeStats"
              :key="stat.key"
              class="border-l border-hairline pl-8"
            >
              <p class="text-caption text-label-secondary">{{ stat.label }}</p>
              <p class="mt-0.5 text-title-3">{{ stat.count }}</p>
            </div>
          </div>
        </RouterLink>

        <!-- 存储用量 + 类型统计 -->
        <div class="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
          <!-- 存储卡：品牌绿环形进度（≥90% 转 warning） -->
          <div class="bg-surface p-6 shadow-card lg:col-span-4 lg:self-start">
            <div class="mb-3 flex items-start justify-between gap-4">
              <h3 class="text-title-3">存储使用情况</h3>
              <span class="flex h-9 w-9 flex-none items-center justify-center text-label-tertiary">
                <AppIcon
                  icon="material-symbols:info-outline-rounded"
                  class="atom-glyph"
                  aria-hidden="true"
                  :style="{ '--atom-glyph-size': '20px' }"
                />
              </span>
            </div>

            <div class="relative flex items-center justify-center py-3">
              <svg class="h-32 w-32 -rotate-90">
                <circle
                  class="text-surface-tertiary"
                  cx="64"
                  cy="64"
                  fill="transparent"
                  r="58"
                  stroke="currentColor"
                  stroke-width="8"
                />
                <circle
                  :class="isStorageWarning ? 'text-warning' : 'text-primary'"
                  class="transition-[stroke-dashoffset] duration-standard"
                  cx="64"
                  cy="64"
                  fill="transparent"
                  r="58"
                  stroke="currentColor"
                  :stroke-dasharray="circumference"
                  :stroke-dashoffset="dashOffset"
                  stroke-width="8"
                  stroke-linecap="round"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center">
                <span class="text-title-1" :class="isStorageWarning && 'text-danger'">
                  {{ storageUsedPercent }}%
                </span>
                <span
                  class="text-caption font-semibold uppercase tracking-[0.08em] text-label-tertiary"
                >
                  已使用
                </span>
              </div>
            </div>

            <div class="mt-4 space-y-2.5 border-t border-hairline pt-4 text-subhead">
              <div class="flex items-center justify-between gap-4">
                <span class="text-label-secondary">已使用</span>
                <span class="font-semibold text-label">{{ storageUsedStr }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-label-secondary">总容量</span>
                <span class="font-semibold text-label">{{ storageTotalStr }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-label-secondary">剩余</span>
                <span
                  class="font-semibold"
                  :class="isStorageWarning ? 'text-danger' : 'text-primary'"
                  >{{ storageLeftStr }}</span
                >
              </div>
            </div>
          </div>

          <!-- 文件类型统计：hover 微浮起 + 阴影加深 -->
          <div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:col-span-8 xl:grid-cols-3">
            <div
              v-for="stat in fileStats"
              :key="stat.type"
              class="flex flex-col justify-between bg-surface p-5 shadow-card transition-[transform,box-shadow] hover:-translate-y-0.5 hover:shadow-popover"
            >
              <div :class="['mb-4 flex h-10 w-10 items-center justify-center', stat.colorClass]">
                <AppIcon
                  :icon="stat.icon"
                  class="atom-glyph"
                  :style="{ '--atom-glyph-size': '20px' }"
                />
              </div>
              <div>
                <h4 class="text-subhead text-label-secondary">{{ stat.title }}</h4>
                <p class="text-title-1 mt-0.5">{{ stat.count }}</p>
                <p class="text-caption mt-1.5 text-label-tertiary">共 {{ stat.size }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 操作日志：iCloud Drive 风格 48px 行 -->
        <div class="overflow-hidden bg-surface shadow-card">
          <div class="flex items-center justify-between border-b border-hairline p-6">
            <h3 class="text-title-3">操作日志</h3>
            <span class="text-subhead text-label-secondary">{{
              isOperationLogsLoading ? '加载中...' : `共 ${operationLogTotal} 条`
            }}</span>
          </div>

          <!-- 加载骨架 -->
          <div v-if="isOperationLogsLoading" class="space-y-3.5 p-6">
            <div v-for="n in 2" :key="n" class="flex items-center gap-3">
              <div class="h-8 w-8 flex-none bg-hairline"></div>
              <div class="flex-1 space-y-1.5">
                <div class="h-3 w-1/2 bg-hairline"></div>
                <div class="h-3 w-1/3 bg-hairline"></div>
              </div>
            </div>
          </div>

          <template v-else>
            <p
              v-if="operationLogCards.length === 0"
              class="py-8 text-center text-subhead text-label-secondary"
            >
              暂无操作记录
            </p>

            <template v-for="(card, index) in operationLogCards" :key="card.id">
              <div
                class="flex min-h-12 items-center gap-3 px-6 transition-colors hover:bg-surface-secondary"
              >
                <div
                  class="flex h-8 w-8 flex-none items-center justify-center"
                  :class="card.colorClass"
                >
                  <AppIcon
                    :icon="card.icon"
                    class="atom-glyph"
                    :style="{ '--atom-glyph-size': '16px' }"
                  />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-lead font-semibold leading-[22px]">
                    {{ card.title }}
                  </p>
                  <p class="truncate text-subhead text-label-secondary">
                    {{ card.subtitle
                    }}<span v-if="card.highlight" class="text-primary"
                      >{{ ' ' }}{{ card.highlight }}</span
                    >
                  </p>
                </div>
                <div class="flex-none text-right">
                  <p class="text-caption text-label-tertiary">{{ card.time }}</p>
                </div>
              </div>
              <div v-if="index < operationLogCards.length - 1" class="ml-17 h-px bg-hairline" />
            </template>

            <!-- 加载更多 -->
            <div v-if="hasMore" class="border-t border-hairline px-6 py-3">
              <button
                type="button"
                class="w-full py-2 text-subhead font-medium text-primary transition-colors hover:bg-surface-secondary disabled:opacity-50"
                :disabled="isLoadingMore"
                @click="loadMoreOperationLogs"
              >
                {{ isLoadingMore ? '加载中…' : '加载更多' }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </main>
  </div>
</template>
