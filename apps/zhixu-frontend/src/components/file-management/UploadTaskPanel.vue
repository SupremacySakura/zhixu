<script setup lang="ts">
import { computed, ref } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import {
  AnimatePresence,
  Motion,
  animate,
  useDragControls,
  useMotionValue,
  type PanInfo,
} from 'motion-v'
import { springSnappy } from '../../utils/motion'
import type { UploadTask } from '../../types/file'
import { iconForFile } from '../../utils/file'
import { uploadTaskMeta } from '../../utils/file-management'

const props = defineProps<{
  modelValue: boolean
  tasks: readonly UploadTask[]
  isUploading: boolean
  overallProgress: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  close: []
  'clear-completed': []
  retry: [task: UploadTask]
  cancel: [task: UploadTask]
  remove: [taskId: string]
}>()

const completedCount = computed(() => props.tasks.filter(task => task.status === 'success').length)
const hasCompletedTasks = computed(() => completedCount.value > 0)
const normalizedOverallProgress = computed(() =>
  Math.min(100, Math.max(0, Math.floor(props.overallProgress))),
)
const taskRows = computed(() =>
  props.tasks.map(task => {
    const meta = uploadTaskMeta(task.status)
    const percent = Math.min(100, Math.max(0, Math.floor(task.percent)))
    return {
      task,
      icon: iconForFile(task.file),
      meta,
      percent,
      statusLabel: meta.label ?? `${percent}%`,
      removable: ['success', 'failed', 'canceled'].includes(task.status),
    }
  }),
)

const closePanel = () => {
  emit('update:modelValue', false)
  emit('close')
}

const iconButtonClass =
  'flex h-8 w-8 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-surface-secondary hover:text-label'

// —— 拖拽换位 + 折叠（DESIGN.md §5.3）——
const isCollapsed = ref(false)
const dragControls = useDragControls()
const x = useMotionValue(0)
const y = useMotionValue(0)
const panelEl = ref<HTMLElement | null>(null)

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const handlePanelPanEnd = (_event: unknown, _info: PanInfo) => {
  // 边界橡皮筋释放：把面板弹回视口内（留 8px 边距）
  const el = panelEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const targetLeft = clamp(rect.left, 8, Math.max(8, window.innerWidth - rect.width - 8))
  const targetTop = clamp(rect.top, 8, Math.max(8, window.innerHeight - rect.height - 8))
  const dx = targetLeft - rect.left
  const dy = targetTop - rect.top
  if (dx !== 0) animate(x, x.get() + dx, springSnappy)
  if (dy !== 0) animate(y, y.get() + dy, springSnappy)
}
</script>

<template>
  <!-- 非阻塞浮层：不铺遮罩，页面照常可操作（DESIGN.md §Layout） -->
  <div class="pointer-events-none fixed inset-0 z-40">
    <AnimatePresence>
      <Motion
        v-if="modelValue && tasks.length > 0"
        key="upload-task-panel"
        ref="panelEl"
        :initial="{ opacity: 0, y: 12 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0, y: 12 }"
        :transition="springSnappy"
        :style="{ x, y }"
        drag
        :drag-controls="dragControls"
        :drag-listener="false"
        :drag-momentum="false"
        :drag-elastic="0"
        class="pointer-events-auto absolute bottom-4 right-4 flex max-h-[500px] w-[calc(100vw-2rem)] flex-col overflow-hidden border border-hairline bg-surface shadow-floating sm:w-96"
        @pan-end="handlePanelPanEnd"
      >
        <!-- 拖拽把手：按住标题区拖动换位 -->
        <div
          class="flex flex-none cursor-grab touch-none items-center justify-between border-b border-hairline px-4 py-3 active:cursor-grabbing"
          @pointerdown="dragControls.start($event)"
        >
          <div class="flex items-center gap-2">
            <AppLoading v-if="isUploading" :size="18" class="text-primary" />
            <AppIcon
              v-else
              icon="material-symbols:check-circle"
              class="atom-glyph text-primary"
              :style="{ '--atom-glyph-size': '18px' }"
            />
            <span class="text-lead font-semibold text-label">
              上传任务 ({{ completedCount }}/{{ tasks.length }})
            </span>
          </div>
          <div class="flex items-center gap-1">
            <button
              :class="iconButtonClass"
              type="button"
              :aria-label="isCollapsed ? '展开上传面板' : '折叠上传面板'"
              :aria-expanded="!isCollapsed"
              @pointerdown.stop
              @click="isCollapsed = !isCollapsed"
            >
              <AppIcon
                class="atom-glyph transition-transform"
                :class="isCollapsed ? '-rotate-180' : ''"
                icon="material-symbols:expand-more"
                :style="{ '--atom-glyph-size': '18px' }"
              />
            </button>
            <button
              v-if="hasCompletedTasks"
              :class="iconButtonClass"
              type="button"
              aria-label="清理已完成的上传任务"
              title="清理已完成"
              @pointerdown.stop
              @click="emit('clear-completed')"
            >
              <AppIcon
                class="atom-glyph"
                icon="material-symbols:cleaning-services"
                :style="{ '--atom-glyph-size': '18px' }"
              />
            </button>
            <button
              :class="iconButtonClass"
              type="button"
              aria-label="关闭上传面板"
              title="关闭面板"
              @pointerdown.stop
              @click="closePanel"
            >
              <AppIcon
                class="atom-glyph"
                icon="material-symbols:close"
                :style="{ '--atom-glyph-size': '18px' }"
              />
            </button>
          </div>
        </div>

        <!-- 可折叠主体：总体进度 + 任务列表 -->
        <Motion
          :initial="false"
          :animate="{ height: isCollapsed ? 0 : 'auto', opacity: isCollapsed ? 0 : 1 }"
          :transition="springSnappy"
          class="flex min-h-0 flex-col overflow-hidden"
        >
          <div class="border-b border-hairline bg-surface-secondary px-4 py-2.5">
            <div class="mb-1 flex items-center justify-between text-caption">
              <span class="text-label-secondary">总体进度</span>
              <span class="font-semibold text-label">{{ normalizedOverallProgress }}%</span>
            </div>
            <div class="h-1.5 overflow-hidden bg-surface-tertiary">
              <div
                class="h-full bg-primary transition-[width] duration-standard"
                :style="{ width: `${normalizedOverallProgress}%` }"
              ></div>
            </div>
          </div>

          <div class="max-h-[350px] flex-1 space-y-1.5 overflow-y-auto p-2">
            <div
              v-for="row in taskRows"
              :key="row.task.id"
              class="flex items-center gap-3 bg-surface-secondary/70 p-3"
            >
              <div
                class="flex h-8 w-8 shrink-0 items-center justify-center"
                :class="`${row.icon.bg} ${row.icon.fg}`"
              >
                <AppIcon
                  :icon="row.icon.icon"
                  class="atom-glyph"
                  :style="{ '--atom-glyph-size': '16px' }"
                />
              </div>

              <div class="min-w-0 flex-1">
                <div class="mb-1 flex items-center justify-between gap-2">
                  <p class="truncate text-body font-medium text-label">
                    {{ row.task.file.name }}
                  </p>
                  <span class="flex-none text-caption font-semibold" :class="row.meta.color">
                    {{ row.statusLabel }}
                  </span>
                </div>
                <div class="h-1 overflow-hidden bg-surface-tertiary">
                  <div
                    class="h-full transition-[width] duration-standard"
                    :class="row.meta.bar"
                    :style="{ width: `${row.percent}%` }"
                  ></div>
                </div>
                <p class="mt-1 truncate text-note text-label-tertiary">{{ row.task.message }}</p>
              </div>

              <div class="flex flex-none items-center gap-0.5">
                <button
                  v-if="row.task.status === 'failed'"
                  class="flex h-7 w-7 items-center justify-center text-label-secondary transition-colors tap-target hover:text-primary"
                  type="button"
                  aria-label="重试上传"
                  @click="emit('retry', row.task)"
                >
                  <AppIcon
                    class="atom-glyph"
                    icon="material-symbols:replay"
                    :style="{ '--atom-glyph-size': '16px' }"
                  />
                </button>
                <button
                  v-if="row.meta.active"
                  class="flex h-7 w-7 items-center justify-center text-label-secondary transition-colors tap-target hover:text-danger"
                  type="button"
                  aria-label="取消上传"
                  @click="emit('cancel', row.task)"
                >
                  <AppIcon
                    class="atom-glyph"
                    icon="material-symbols:close"
                    :style="{ '--atom-glyph-size': '16px' }"
                  />
                </button>
                <button
                  v-if="row.removable"
                  class="flex h-7 w-7 items-center justify-center text-label-secondary transition-colors tap-target hover:text-danger"
                  type="button"
                  aria-label="移除任务"
                  @click="emit('remove', row.task.id)"
                >
                  <AppIcon
                    class="atom-glyph"
                    icon="material-symbols:delete"
                    :style="{ '--atom-glyph-size': '16px' }"
                  />
                </button>
              </div>
            </div>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </div>
</template>
