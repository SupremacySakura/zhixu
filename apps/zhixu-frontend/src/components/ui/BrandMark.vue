<script setup lang="ts">
/**
 * 品牌行：一枚 16px 方章 + 字标「知序」。
 *
 * 为什么方章要写成内联 SVG 而不是图标字形：世界里的品牌不是「一个图标」，
 * 是一枚盖上去的章 —— 蓝底 + 1px 蓝描边 + 反白「序」。图标表里没有它，
 * 也不该有：图标是功能语汇，方章是身份语汇。
 *
 * 为什么字标不加 letter-spacing：DESIGN.md 的 Brand Assets 一节写着
 * `.2em`，但同一份规范的 Zero Letter-Spacing Rule 明令「中文的
 * letter-spacing 恒为 0，标题也不许取负值」。两条冲突时取 Named Rule ——
 * 它讲的是中文排版的物理事实（加字距会把汉字拆成孤立方块），而 `.2em`
 * 那句是从侧栏品牌区的旧稿里带过来的。
 *
 * 为什么方章 aria-hidden：旁边紧跟着文字「知序」，图形再报一遍只是噪音。
 */
</script>

<template>
  <div class="flex items-center gap-2">
    <svg
      class="flex-none text-primary"
      width="16"
      height="16"
      viewBox="0 0 16 16"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="0.5" y="0.5" width="15" height="15" fill="currentColor" />
      <text
        x="8"
        y="11.6"
        text-anchor="middle"
        font-size="10"
        font-weight="700"
        fill="var(--color-surface-tertiary)"
      >
        序
      </text>
    </svg>
    <span class="text-title text-label">知序</span>
  </div>
</template>
