<script setup lang="ts">
import AppIcon from '../ui/AppIcon.vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const goToLogin = () => {
  router.push({
    path: '/login',
    query: { redirect: router.currentRoute.value.fullPath },
  })
}
</script>

<template>
  <div class="flex h-full min-h-[400px] flex-col items-center justify-center p-8 text-center">
    <div
      class="mb-5 flex h-20 w-20 items-center justify-center rounded-full bg-primary-tint text-primary"
    >
      <AppIcon icon="material-symbols:lock-outline" class="h-10 w-10" />
    </div>
    <h2 class="text-title-3 mb-2">登录后可用</h2>
    <p class="text-body mb-6 text-label-secondary">
      此功能需要登录后才能使用，登录后即可享受完整的知识库服务
    </p>
    <button
      @click="goToLogin"
      class="flex h-12 items-center justify-center rounded-full bg-primary px-8 text-[16px] font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
    >
      立即登录
    </button>
  </div>
</template>
