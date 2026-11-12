<script setup lang="ts">
/**
 * 空状态零件：当这里什么都没有的时候，说清楚为什么、下一步做什么。
 *
 * 为什么强制 title：一个只有插画的空状态等于一句「没有」。空状态的职责
 * 是解释「为什么空」与「接下来能做什么」，所以 title 是必填，description
 * 与 actions 才是可选的。
 *
 * 为什么图标盒是方的而不是圆环插画：旧世界的双层圆环是「浅色 Apple」的
 * 语汇；世界里没有圆，插画位置改成一个带描边的方章，图标坐在里面。
 */
import { computed } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 必填：空状态的第一句话。 */
    title: string
    description?: string
    /** 插画位置的图标名。 */
    icon?: string
    /** 四档语气，决定方章的颜色。 */
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档尺寸：只决定方章大小与留白。 */
    size?: 'sm' | 'md' | 'lg'
  }>(),
  {
    description: '',
    // 默认字形必须取自本地 106 字形表（守门脚本会把不存在的名字判为失败）
    icon: 'material-symbols:folder-open-outline',
    tone: 'secondary',
    size: 'md',
  },
)

const SCALES = {
  sm: { mark: 40, padding: 24 },
  md: { mark: 48, padding: 32 },
  lg: { mark: 64, padding: 48 },
} as const

const TONE_CLASS = {
  primary: 'atom-tint-primary',
  secondary: 'atom-tint-secondary',
  quiet: 'atom-tint-quiet',
  danger: 'atom-tint-danger',
} as const

const scale = computed(() => SCALES[props.size])
const rootStyle = computed(() => ({
  '--atom-empty-mark-size': `${scale.value.mark}px`,
  '--atom-empty-padding': `${scale.value.padding}px`,
}))

defineSlots<{
  /** 操作区（按钮），通常 0–2 个。 */
  actions?: () => unknown
  /** 完全自定义内容，给了就不渲染默认的图标方章。 */
  default?: () => unknown
}>()
</script>

<template>
  <div class="atom-empty" :class="TONE_CLASS[tone]" :style="rootStyle">
    <template v-if="$slots.default">
      <slot />
    </template>
    <template v-else>
      <span class="atom-empty-mark">
        <AppIcon :icon="icon" class="atom-glyph" aria-hidden="true" />
      </span>
      <p class="atom-empty-title">{{ title }}</p>
      <p v-if="description" class="atom-empty-description">{{ description }}</p>
    </template>
    <div v-if="$slots.actions" class="atom-empty-actions">
      <slot name="actions" />
    </div>
  </div>
</template>
