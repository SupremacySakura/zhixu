<script setup lang="ts">
/**
 * 提示条零件：订阅式，全应用只挂一次。
 *
 * 为什么是订阅而不是 props：迁移前三处（组件 + 组合式函数 + store）各发各的，
 * 界面上可能出现两个提示。收敛成总线后，任何地方只调 toast()，组件自己订阅，
 * 所以「界面上一共几个提示」不再取决于谁记得去渲染它。
 *
 * 为什么只渲染最新一条：提示是「刚刚发生了一件事」的通报，不是日志。
 * 叠三条会把视线挡在内容上，而用户真正需要看到的永远是最后那条。
 * 队列留给需要回看的东西（那是操作日志的职责）。
 *
 * 为什么错误用 assertive 而其余用 polite：全都 assertive 会让连续提示
 * 互相打断；错误是需要立刻被听见的那一条，抢话是它的本分。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppIcon from './AppIcon.vue'
import { subscribe } from '../../utils/toastBus'
import type { ToastMessage, ToastType } from '../../utils/toastBus'

const props = withDefaults(
  defineProps<{
    /** 自动消失时长（毫秒）。 */
    duration?: number
  }>(),
  { duration: 3000 },
)

const current = ref<ToastMessage | null>(null)
let timer: ReturnType<typeof setTimeout> | null = null
let unsubscribe: (() => void) | null = null

/**
 * 图标取自本地 106 字形表，不新增字形：世界里没有独立的「警告」三角形，
 * 警告用**描边**错误字形表示「还没失败，但要看一眼」，失败用实心。
 * 真正区分四类的是左侧那条强调色，字形只做辅助。
 */
const ICONS: Record<ToastType, string> = {
  success: 'material-symbols:check-circle',
  error: 'material-symbols:error',
  warning: 'material-symbols:error-outline-rounded',
  info: 'material-symbols:info',
}

/** 语气类名按类型给：颜色只体现在左侧 3px 强调条与图标上，文字始终是墨色。 */
const TONE_CLASS: Record<ToastType, string> = {
  success: 'atom-toast-success',
  error: 'atom-toast-error',
  warning: 'atom-toast-warning',
  info: 'atom-toast-info',
}

const icon = computed(() => (current.value ? ICONS[current.value.type] : ''))
const toneClass = computed(() => (current.value ? TONE_CLASS[current.value.type] : ''))

function clearTimer() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
}

/** 新消息直接顶掉旧的：只留一条，所以不需要队列。 */
function show(payload: ToastMessage) {
  clearTimer()
  current.value = payload
  timer = setTimeout(() => {
    current.value = null
    timer = null
  }, props.duration)
}

onMounted(() => {
  unsubscribe = subscribe(show)
})

// 取消订阅是硬要求：热更新与组件销毁后仍持有监听会让消息落到已卸载的组件上。
onBeforeUnmount(() => {
  clearTimer()
  unsubscribe?.()
  unsubscribe = null
})
</script>

<template>
  <!-- 外层只负责定位，pointer-events 关闭以免挡住页面；内层再打开。 -->
  <div class="atom-toast-layer">
    <div
      v-if="current"
      :key="current.id"
      class="atom-toast"
      :class="toneClass"
      :role="current.type === 'error' ? 'alert' : 'status'"
      :aria-live="current.type === 'error' ? 'assertive' : 'polite'"
    >
      <AppIcon :icon="icon" class="atom-glyph atom-toast-icon" aria-hidden="true" />
      <span class="atom-toast-message">{{ current.message }}</span>
    </div>
  </div>
</template>
