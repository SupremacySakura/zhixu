<script setup lang="ts">
/**
 * 按钮零件：一个按钮长什么样，从此只有一个答案。
 *
 * 为什么只有 tone × size 两个开关：世界里的圆角恒为 0、阴影只有三档固定
 * 硬偏移。把 rounded / shadow / color 暴露成变体，等于允许每个调用方自己
 * 重定义世界——那正是迁移前 52 种主按钮写法的成因。
 *
 * 为什么样式在 styles/atoms.css 而不在 scoped style 里：骨架必须能被别的
 * 零件复用（AppDialog 的按钮、AppMenu 的项），scoped 样式带 data-v 哈希，
 * 跨组件就复用不了；@utility 定义的骨架才能被 @apply 组合。
 *
 * 为什么加载态不设原生 disabled：设了按钮会掉焦点，键盘用户按 Tab 之后
 * 焦点凭空消失。改用「提交闸」——click 处理里直接早退，同时 aria-busy
 * 告诉辅助技术正在忙。视觉上按钮保持自己的语气色，不会被误读成禁用。
 */
import { computed, useAttrs } from 'vue'
import AppIcon from './AppIcon.vue'
import AppLoading from './AppLoading.vue'

const props = withDefaults(
  defineProps<{
    /** 四档语气，没有第五档。 */
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档尺寸：26 / 32 / 36px。 */
    size?: 'sm' | 'md' | 'lg'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    /** 忙态：显示 16px 线环、锁宽度、挡住重复提交。 */
    loading?: boolean
    /** 前置图标名（material-symbols 字形）。 */
    icon?: string
    /** 只有图标没有文字：正方形热区。必须用 aria-label 给可访问名称。 */
    iconOnly?: boolean
    /** 提交闸被拦下的点击是否向上抛 click 事件（默认不抛）。 */
    emitWhileBusy?: boolean
  }>(),
  {
    tone: 'secondary',
    size: 'md',
    type: 'button',
    disabled: false,
    loading: false,
    icon: '',
    iconOnly: false,
    emitWhileBusy: false,
  },
)

const emit = defineEmits<{ click: [event: MouseEvent] }>()
const attrs = useAttrs()

/** 尺寸三档 → 高度 / 内边距 / 字号 / 图标字号。票面只钉了高度，其余按字级表就近取档。 */
const SCALES = {
  sm: { height: 26, padding: 8, font: 'var(--text-note)', glyph: 14 },
  md: { height: 32, padding: 12, font: 'var(--text-body)', glyph: 16 },
  lg: { height: 36, padding: 14, font: 'var(--text-lead)', glyph: 18 },
} as const

const TONE_CLASS = {
  primary: 'atom-tone-primary',
  secondary: 'atom-tone-secondary',
  quiet: 'atom-tone-quiet',
  danger: 'atom-tone-danger',
} as const

const scale = computed(() => SCALES[props.size])

const rootClass = computed(() => [
  'atom-btn',
  TONE_CLASS[props.tone],
  { 'atom-btn-square': props.iconOnly },
])

/** 尺寸走 CSS 自定义属性：调用方还能用 style 覆写单个值，不必加第四档。 */
const rootStyle = computed(() => ({
  '--atom-control-height': `${scale.value.height}px`,
  '--atom-control-padding': `${scale.value.padding}px`,
  '--atom-control-font-size': scale.value.font,
}))

const glyphStyle = computed(() => ({ '--atom-glyph-size': `${scale.value.glyph}px` }))

/** 图标按钮必须有可访问名称，否则屏幕阅读器只会念「按钮」。 */
if (import.meta.env.DEV) {
  const hasName = () => Boolean(attrs['aria-label'] || attrs['aria-labelledby'] || attrs.title)
  if (props.iconOnly && !hasName()) {
    console.warn('[AppButton] iconOnly 必须配 aria-label，否则按钮没有可访问名称。')
  }
}

function onClick(event: MouseEvent) {
  // 提交闸：忙态直接吞掉点击，防止同一请求被重复提交。
  if (props.loading || props.disabled) {
    if (props.emitWhileBusy) emit('click', event)
    return
  }
  emit('click', event)
}
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || undefined"
    :aria-busy="loading ? 'true' : undefined"
    :class="rootClass"
    :style="rootStyle"
    @click="onClick"
  >
    <!-- 内容始终留在 DOM 里（忙态只切 visibility），宽度因此一格都不动；
         这也是「锁 min-width」最稳的实现——不依赖 JS 测量。 -->
    <span class="atom-btn-content" :data-busy="loading ? 'true' : 'false'">
      <AppIcon v-if="icon" :icon="icon" class="atom-glyph" :style="glyphStyle" aria-hidden="true" />
      <span v-if="!iconOnly" class="truncate"><slot /></span>
    </span>
    <AppLoading v-if="loading" size="md" class="atom-btn-spinner" />
  </button>
</template>
