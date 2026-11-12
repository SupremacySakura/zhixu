<script setup lang="ts">
/**
 * 面板零件：世界里「一张卡纸」。
 *
 * 为什么层级的表达只有描边与硬偏移：四表面亮度比全部 < 1.2（1.078–1.277），
 * 换底色根本看不出来。所以 tone 不换底色的深浅，只换描边强度与是否有顶线。
 *
 * 为什么忙态盖一层而不是把内容变淡：变淡会把文字一起压到 AA 以下。
 * 这里用一层半透明纸色遮住内容、中间放线环，内容本身不动。
 */
import { computed } from 'vue'
import AppLoading from './AppLoading.vue'

const props = withDefaults(
  defineProps<{
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档尺寸只决定内边距，不决定字号（面板里的字由内容自己决定）。 */
    size?: 'sm' | 'md' | 'lg'
    title?: string
    description?: string
    disabled?: boolean
    loading?: boolean
  }>(),
  {
    tone: 'secondary',
    size: 'md',
    title: '',
    description: '',
    disabled: false,
    loading: false,
  },
)

const SCALES = {
  sm: { padding: 12 },
  md: { padding: 16 },
  lg: { padding: 20 },
} as const

const TONE_CLASS = {
  primary: 'atom-tint-primary atom-panel-primary',
  secondary: 'atom-tint-secondary',
  quiet: 'atom-tint-quiet atom-panel-quiet',
  danger: 'atom-tint-danger atom-panel-danger',
} as const

const scale = computed(() => SCALES[props.size])
const rootStyle = computed(() => ({ '--atom-panel-padding': `${scale.value.padding}px` }))

defineSlots<{
  default?: () => unknown
  /** 标题右侧的操作区（按钮、菜单）。 */
  actions?: () => unknown
  /** 自定义页头，给了就完全替换默认的 title/description。 */
  header?: () => unknown
  /** 页脚（底部按钮）。 */
  footer?: () => unknown
}>()
</script>

<template>
  <section
    class="atom-panel"
    :class="TONE_CLASS[tone]"
    :style="rootStyle"
    :aria-disabled="disabled ? 'true' : undefined"
    :aria-busy="loading ? 'true' : undefined"
  >
    <div v-if="$slots.header || title || description || $slots.actions" class="atom-panel-header">
      <slot name="header">
        <div class="min-w-0">
          <h2 v-if="title" class="atom-panel-title">{{ title }}</h2>
          <p v-if="description" class="atom-panel-description">{{ description }}</p>
        </div>
        <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
          <slot name="actions" />
        </div>
      </slot>
    </div>
    <div class="atom-panel-body">
      <slot />
    </div>
    <div v-if="$slots.footer" class="atom-panel-footer">
      <slot name="footer" />
    </div>
    <!-- 忙态遮罩：内容保留在 DOM 里，只盖一层纸色 + 线环，文字不参与降级 -->
    <div v-if="loading" class="atom-panel-busy">
      <AppLoading size="lg" />
    </div>
  </section>
</template>
