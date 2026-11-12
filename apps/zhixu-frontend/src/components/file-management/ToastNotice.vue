<script setup lang="ts">
/**
 * 消息提示条：全仓唯一挂载的提示宿主（挂在 HomePage 外壳里，只挂一次）。
 *
 * 它不再接受 props——旧写法要求每个页面自己持有 showToast/message/type 三个状态
 * 再传给本组件，于是「谁在说话」这件事有三条并存的链路。现在它只订阅
 * toastBus，任何地方调 toast() 都会在这里渲染，界面上因此只有一个提示。
 * 外观（定位、弹簧、胶囊、图标配色）保持迁移前原样。
 */
import { computed, onScopeDispose, ref } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
import { subscribe, type ToastMessage, type ToastType } from '../../utils/toastBus'

/** 停留时长：沿用旧 useToast 的默认 3000ms，行为不因架构收敛而改变。 */
const DURATION = 3000

const current = ref<ToastMessage | null>(null)
let timer: ReturnType<typeof setTimeout> | null = null

function clearTimer() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
}

// 后来的消息直接替换前一条（与旧 useToast「新消息顶掉旧消息」一致），而不是排队。
const unsubscribe = subscribe(message => {
  clearTimer()
  current.value = message
  timer = setTimeout(() => {
    current.value = null
    timer = null
  }, DURATION)
})

onScopeDispose(() => {
  unsubscribe()
  clearTimer()
})

const type = computed<ToastType>(() => current.value?.type ?? 'info')

const icon = computed(() => {
  if (type.value === 'success') return 'material-symbols:check-circle'
  if (type.value === 'error') return 'material-symbols:error'
  if (type.value === 'warning') return 'material-symbols:error-outline-rounded'
  return 'material-symbols:info'
})
const iconClass = computed(() => {
  if (type.value === 'success') return 'text-primary'
  if (type.value === 'error') return 'text-danger'
  if (type.value === 'warning') return 'text-warning'
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
        v-if="current"
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
        <span class="text-sm font-medium text-label">{{ current.message }}</span>
      </Motion>
    </AnimatePresence>
  </div>
</template>
