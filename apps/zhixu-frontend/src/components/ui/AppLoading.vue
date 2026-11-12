<script setup lang="ts">
/**
 * 加载指示零件：全仓唯一的「等待」形态 —— 1s 匀速线环。
 *
 * 为什么不是图标 + animate-spin：线环是几何，图标是字形。世界里的图标
 * 来自 material-symbols 的字形表，用它当 spinner 会让「加载」和「转圈图标」
 * 混成一个概念，且换图标族时加载态会跟着变样。线环是纯 CSS，跟图标族解耦。
 *
 * 为什么 size 允许数字：票面三档 14 / 16 / 20 装不下既有的 28 / 30 / 24 / 18px
 * 七处非按钮 spinner（空状态插画、预览加载、加载行）。扩第四档等于承认
 * 「尺寸可以随便定」，所以这里留一个显式的数字逃生口：三档是契约，
 * 数字是调用方对某个具体场景的个别决定，读代码时一眼能看出谁破了档。
 *
 * 为什么默认 aria-hidden：它几乎总是嵌在别的东西里（按钮的 aria-busy、
 * 面板的 aria-busy、对话框的标题）。独立使用时传 label，才会渲染一条
 * sr-only 的 role="status"，由它来播报。
 */
withDefaults(
  defineProps<{
    /** 三档是契约（14 / 16 / 20px）；数字是逃生口，见文件头说明。 */
    size?: 'sm' | 'md' | 'lg' | number
    /** 独立使用时给一个可播报的名字；嵌在别处时不要传。 */
    label?: string
  }>(),
  { size: 'md', label: '' },
)

const DIAMETERS = { sm: 14, md: 16, lg: 20 } as const

/** 尺寸同时写进 CSS 变量：调用方还能用 style="--atom-ring-size: 30px" 覆盖。 */
function diameter(size: 'sm' | 'md' | 'lg' | number): string {
  return `${typeof size === 'number' ? size : DIAMETERS[size]}px`
}
</script>

<template>
  <span
    class="atom-ring"
    :style="{ '--atom-ring-size': diameter(size) }"
    :aria-hidden="label ? undefined : 'true'"
  >
    <span v-if="label" class="sr-only" role="status">{{ label }}</span>
  </span>
</template>
