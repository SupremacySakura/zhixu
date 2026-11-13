<!--
  守卫的合成 fixture：故意写满六类违规 + 响应式前缀变体。
  文件名带 .fixture.vue，不匹配 vitest 的 *.test.ts 通配，不增加用例计数。
  真正的断言在 eslint 本身 —— 跑 ./node_modules/.bin/eslint 就能看到这些被拦住。
-->
<template>
  <div>
    <!-- 应该被拦：圆角（含响应式/状态前缀） -->
    <div class="rounded-full" />
    <div class="sm:rounded-full" />
    <div class="md:rounded-lg" />
    <div class="hover:rounded-sm" />
    <div class="focus:rounded-full" />
    <div class="lg:group-hover:rounded-full" />

    <!-- 应该被拦：任意值字号 -->
    <p class="text-[13px]">文字</p>
    <p class="lg:text-[42px]">响应式任意字号</p>

    <!-- 应该被拦：框架阴影档 -->
    <div class="shadow-md" />
    <div class="hover:shadow-lg" />
    <div class="md:shadow-xl" />

    <!-- 应该被拦：毛玻璃 / material -->
    <div class="backdrop-blur-md" />
    <div class="bg-material-thick" />
    <div class="sm:backdrop-blur-xl" />

    <!-- 应该被拦：框架容器宽度 -->
    <div class="max-w-6xl" />
    <div class="md:max-w-6xl" />

    <!-- 应该被拦：过渡时长 -->
    <div class="duration-150" />
    <div class="duration-[90ms]" />
    <div class="hover:duration-200" />

    <!-- 应该被拦：按压缩放 -->
    <button class="active:scale-[0.97]" type="button">按钮</button>
    <button class="md:active:scale-95" type="button">按钮</button>

    <!-- 应该被拦：transition-all -->
    <div class="transition-all" />

    <!-- 应该被拦：旋转/脉冲 -->
    <span class="animate-spin" />
    <span class="animate-pulse" />

    <!-- 应该被拦：自写焦点样式 -->
    <button class="focus:ring-2 focus:ring-primary" type="button">按钮</button>
    <button class="focus-visible:outline-primary" type="button">按钮</button>

    <!-- 动态 class 绑定也要拦 -->
    <div :class="['rounded-full', condition ? 'shadow-lg' : 'rounded-md']" />
    <div :class="{ 'backdrop-blur-sm': active }" />

    <!-- 以下不该被拦（世界里合法） -->
    <div class="shadow-card" />
    <div class="shadow-popover" />
    <div class="shadow-floating" />
    <div class="max-w-measure" />
    <div class="max-w-panel" />
    <div class="max-w-wide" />
    <p class="text-body">语义档</p>
    <p class="text-lead font-normal">语义档</p>
    <div class="rounded-none" />
    <div class="duration-standard" />
    <div class="duration-sheet" />
    <div class="transition-colors" />
    <div class="atom-glyph" :style="{ '--atom-glyph-size': '18px' }" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const condition = ref(true)
const active = ref(true)
</script>
