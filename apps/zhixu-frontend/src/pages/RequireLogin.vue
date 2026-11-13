<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'
import BrandMark from '../components/ui/BrandMark.vue'

const route = useRoute()
const redirectPath = typeof route.query.redirect === 'string' ? route.query.redirect : '/home'
</script>

<template>
  <!-- 认证独立页（DESIGN.md 原型⑥）：480px 单卡、无页头带。
       两个动作是 RouterLink 而不是按钮元素，所以不能换 AppButton
       （AppButton 没有 as prop）；这里直接用同一对原子画出同一个东西。 -->
  <div class="flex min-h-screen flex-col">
    <main
      id="main-content"
      tabindex="-1"
      class="flex flex-1 flex-col items-center justify-center px-4 py-12"
    >
      <div class="mb-6 flex w-full max-w-[480px] items-center justify-between">
        <BrandMark />
        <RouterLink
          to="/register"
          class="tap-target text-body font-semibold text-primary hover:underline"
        >
          注册
        </RouterLink>
      </div>

      <div class="cabinet-card w-full max-w-[480px]">
        <div class="flex flex-col items-center px-8 pb-8 pt-10 text-center">
          <div class="mb-5 flex h-16 w-16 items-center justify-center bg-primary-tint text-primary">
            <svg
              class="atom-glyph"
              style="--atom-glyph-size: 30px"
              viewBox="0 0 24 24"
              fill="none"
              aria-hidden="true"
            >
              <rect
                x="4.5"
                y="10.5"
                width="15"
                height="9.5"
                stroke="currentColor"
                stroke-width="1.8"
              />
              <path
                d="M8 10.5V7.8a4 4 0 0 1 8 0v2.7"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
              <circle cx="12" cy="15.3" r="1.2" fill="currentColor" />
            </svg>
          </div>
          <h1 class="text-display mb-2">需要登录</h1>
          <p class="text-body text-label-secondary">此页面需要登录后才能访问。</p>
          <p class="mb-8 mt-1.5 text-note text-label-tertiary">
            登录成功后将自动回到 {{ redirectPath }}
          </p>

          <div class="flex w-full flex-col gap-3">
            <RouterLink
              :to="{ path: '/login', query: { redirect: redirectPath } }"
              class="atom-btn atom-tone-primary w-full justify-center"
              style="--atom-control-height: 44px"
            >
              立即登录
            </RouterLink>
            <RouterLink
              to="/register"
              class="atom-btn atom-tone-secondary w-full justify-center"
              style="--atom-control-height: 44px"
            >
              注册新账号
            </RouterLink>
          </div>
        </div>

        <div class="border-t border-hairline px-8 py-4 text-center text-body text-label-secondary">
          没有账号？注册后即可访问所有功能。
        </div>
      </div>
    </main>
  </div>
</template>
