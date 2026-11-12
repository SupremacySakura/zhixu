<script setup lang="ts">
/**
 * 图标零件：把第三方图标库藏在身后。
 *
 * 存在的理由是让页面不再直接 import '@iconify/vue'——测试的 mock 目标
 * 从第三方包换成这个模块，图标来源换成本地数据时页面代码零改动。
 * 尺寸与颜色照旧由调用方的 class 决定（图标是 1em 见方的 SVG，
 * font-size 即尺寸），所以 class 与 aria-hidden 一律经 fallthrough 落到根节点。
 */
import { Icon } from '@iconify/vue/offline'
import '../../icons'

defineProps<{ icon: string }>()
</script>

<template>
  <Icon :icon="icon" />
</template>
