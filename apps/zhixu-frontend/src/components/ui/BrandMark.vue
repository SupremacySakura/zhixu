<script setup lang="ts">
/**
 * 品牌行：一枚 16px 方章 + 字标「知序」。
 *
 * 为什么方章要写成内联 SVG 而不是图标字形：世界里的品牌不是「一个图标」，
 * 是一枚盖上去的章 —— 蓝底 + 1px 蓝描边 + 反白「序」。图标表里没有它，
 * 也不该有：图标是功能语汇，方章是身份语汇。
 *
 * 为什么字标**要**加 letter-spacing（`.2em`）：DESIGN.md 的 Brand Assets
 * 一节规定字标是「系统黑体 700 + letter-spacing: .2em」，Zero Letter-Spacing
 * Rule 也为它开了明确的豁免——字标不是正文，是被当作图形使用的两字标志。
 * 这里曾经按「中文恒 0」把 `.2em` 拿掉，那是误读了规则：Named Rule 管的是
 * 正文与标题的排版，管不到标志图形。侧栏 `SideBar.vue` 的字标一直是
 * `tracking-[0.2em]`，实测 4.2px = 0.2em；两处必须一致，否则同一个品牌
 * 在登录页与侧栏会长得不一样。
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
    <span class="text-title tracking-[0.2em] text-label">知序</span>
  </div>
</template>
