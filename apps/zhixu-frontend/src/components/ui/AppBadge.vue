<script setup lang="ts">
/**
 * 徽章零件：一个状态词，不是按钮。
 *
 * 为什么没有 iconOnly：徽章的文字就是它的全部信息，去掉字只剩色块
 * 等于让色盲用户读不到状态。
 *
 * 为什么 dot 用 currentColor：状态点必须和字同色，否则一个徽章会有两种
 * 语义色，读起来像「两件事」。颜色由语气类统一给出。
 */
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 四档语气，与按钮同一套词表。 */
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档尺寸：26 / 32 / 36px。 */
    size?: 'sm' | 'md' | 'lg'
    /** 状态点：给「就绪 / 进行中 / 失败」这类持续状态用。 */
    dot?: boolean
    /** 前置图标名。 */
    icon?: string
    /** 语义上不可用（如「暂不可用」状态）。不是交互控件，只标 aria-disabled。 */
    disabled?: boolean
  }>(),
  { tone: 'secondary', size: 'md', dot: false, icon: '', disabled: false },
)

/** 高度严格用契约的三档 26 / 32 / 36，不因为「徽章看起来该更矮」而另开刻度。 */
const SCALES = {
  sm: { height: 26, padding: 8, font: 'var(--text-note)', glyph: 12 },
  md: { height: 32, padding: 10, font: 'var(--text-note)', glyph: 14 },
  lg: { height: 36, padding: 12, font: 'var(--text-body)', glyph: 16 },
} as const

const TONE_CLASS = {
  primary: 'atom-tint-primary',
  secondary: 'atom-tint-secondary',
  quiet: 'atom-tint-quiet',
  danger: 'atom-tint-danger',
} as const

const scale = computed(() => SCALES[props.size])
const rootStyle = computed(() => ({
  '--atom-control-height': `${scale.value.height}px`,
  '--atom-control-padding': `${scale.value.padding}px`,
  '--atom-control-font-size': scale.value.font,
  '--atom-glyph-size': `${scale.value.glyph}px`,
}))
</script>

<template>
  <span
    class="atom-badge"
    :class="TONE_CLASS[tone]"
    :style="rootStyle"
    :aria-disabled="disabled ? 'true' : undefined"
  >
    <span v-if="dot" class="atom-badge-dot" aria-hidden="true" />
    <AppIcon v-if="icon" :icon="icon" class="atom-glyph" aria-hidden="true" />
    <slot />
  </span>
</template>
