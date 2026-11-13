<script setup lang="ts">
import { MotionConfig } from 'motion-v'

// Skip to content 功能：按下 Enter 跳转到主内容区域
const skipToContent = () => {
  const mainContent = document.querySelector('main')
  if (mainContent) {
    mainContent.setAttribute('tabindex', '-1')
    mainContent.focus()
  }
}
</script>

<template>
  <!-- reducedMotion="user"：系统开启减弱动态效果时，所有 Motion 动画降级为透明度变化（DESIGN.md §6） -->
  <MotionConfig reduced-motion="user">
    <div>
      <!-- Skip to content 链接 - 为键盘用户提供快速导航到主内容 -->
      <a
        href="#main-content"
        class="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:bg-primary focus:px-4 focus:py-2 focus:font-semibold focus:text-white"
        @click.prevent="skipToContent"
      >
        跳转到主内容
      </a>
      <router-view></router-view>
    </div>
  </MotionConfig>
</template>
