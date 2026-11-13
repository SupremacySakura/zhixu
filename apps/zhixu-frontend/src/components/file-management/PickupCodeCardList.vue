<script setup lang="ts">
import AppBadge from '../ui/AppBadge.vue'
import AppIcon from '../ui/AppIcon.vue'
import type { PickupCodeItem, PickupCodeStatus, PickupCodeType } from '../../services/types/file'

defineProps<{
  items: readonly PickupCodeItem[]
}>()

const emit = defineEmits<{
  (e: 'open-menu', item: PickupCodeItem, event: MouseEvent): void
}>()

const typeIcon = (type: PickupCodeType) =>
  type === 'folder'
    ? 'material-symbols:folder-zip-outline-rounded'
    : 'material-symbols:description-outline-rounded'

const typeLabel = (type: PickupCodeType) => (type === 'folder' ? '文件夹' : '文件')

const statusLabel = (status: PickupCodeStatus) => (status === 'Active' ? '生效中' : '已过期')

const statusTone = (status: PickupCodeStatus) => (status === 'Active' ? 'primary' : 'danger')

const progressPercent = (item: PickupCodeItem) => {
  if (item.max_download <= 0) return 0
  return Math.min(100, Math.round((item.download / item.max_download) * 100))
}

const formatDate = (dateStr: string) => new Date(dateStr).toISOString().split('T')[0]
</script>

<template>
  <!-- 只负责「有数据时的行」，加载态与空态由页面统一渲染一次：
       两套形态各自维护一份同样的中文文案，迟早会漂。 -->
  <div class="divide-y divide-hairline">
    <article v-for="item in items" :key="item.code" class="flex flex-col gap-3 px-4 py-3.5">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p
            class="inline-block border border-hairline bg-surface-secondary px-2 py-0.5 font-mono text-note font-bold tracking-[0.14em] text-label"
          >
            {{ item.code }}
          </p>
          <p class="mt-1.5 truncate text-lead text-label">{{ item.name }}</p>
        </div>
        <button
          class="flex h-9 w-9 flex-none items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-secondary hover:text-label"
          type="button"
          aria-label="取件码操作菜单"
          @click="e => emit('open-menu', item, e)"
        >
          <AppIcon icon="material-symbols:more-vert" aria-hidden="true" />
        </button>
      </div>

      <!-- 两列信息栅格：窄屏能保住的最少事实，一行一件事，不做横向滚动 -->
      <dl class="grid grid-cols-2 gap-x-4 gap-y-2">
        <div class="min-w-0">
          <dt class="text-micro font-normal uppercase text-label-tertiary">关联文件</dt>
          <dd class="flex min-w-0 items-center gap-1.5">
            <AppIcon :icon="typeIcon(item.type)" class="atom-glyph" aria-hidden="true" />
            <span class="truncate text-note text-label-secondary">{{ typeLabel(item.type) }}</span>
          </dd>
        </div>
        <div class="min-w-0">
          <dt class="text-micro font-normal uppercase text-label-tertiary">下载次数</dt>
          <dd class="text-note text-label-secondary">
            {{ item.download }} / {{ item.max_download }}
          </dd>
        </div>
        <div class="min-w-0">
          <dt class="text-micro font-normal uppercase text-label-tertiary">过期时间</dt>
          <dd class="text-note text-label-secondary">{{ formatDate(item.expire_time) }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-micro font-normal uppercase text-label-tertiary">状态</dt>
          <dd>
            <AppBadge :tone="statusTone(item.status)" size="sm" dot>{{
              statusLabel(item.status)
            }}</AppBadge>
          </dd>
        </div>
      </dl>

      <div class="h-1 w-full overflow-hidden bg-surface-tertiary">
        <div
          class="h-full"
          :class="item.status === 'Active' ? 'bg-primary' : 'bg-danger'"
          :style="{ width: `${progressPercent(item)}%` }"
        ></div>
      </div>
    </article>
  </div>
</template>
