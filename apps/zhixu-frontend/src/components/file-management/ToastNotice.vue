<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
import type { ToastType } from '../../types/file'

const props = defineProps<{
  visible: boolean
  message: string
  type: ToastType
}>()

const icon = computed(() => {
  if (props.type === 'success') return 'material-symbols:check-circle'
  if (props.type === 'error') return 'material-symbols:error'
  return 'material-symbols:info'
})
const iconClass = computed(() => {
  if (props.type === 'success') return 'text-primary'
  if (props.type === 'error') return 'text-danger'
  return 'text-info'
})
</script>

<template>
  <!--
    顶部居中 Toast：外层仅负责定位（flex 居中，避免 translate 冲突），
    内层胶囊由 motion-v 弹簧驱动（DESIGN.md §2.4：damping 1.0 / response 0.3 观感，
    降级由 App.vue MotionConfig reduced-motion="user" 处理）。
  -->
  <div class="pointer-events-none fixed inset-x-0 top-[72px] z-60 flex justify-center px-4">
    <AnimatePresence>
      <Motion
        v-if="visible"
        key="toast"
        :initial="{ opacity: 0, y: -8, scale: 0.98 }"
        :animate="{ opacity: 1, y: 0, scale: 1 }"
        :exit="{ opacity: 0, y: -8 }"
        :transition="springSnappy"
        class="pointer-events-auto flex h-10 items-center gap-2 rounded-full bg-material-thick px-4.5 shadow-floating backdrop-blur-[20px] backdrop-saturate-[180%]"
        :role="type === 'error' ? 'alert' : 'status'"
        aria-live="polite"
      >
        <AppIcon :icon="icon" class="text-lg" :class="iconClass" />
        <span class="text-sm font-medium text-label">{{ message }}</span>
      </Motion>
    </AnimatePresence>
  </div>
</template>
