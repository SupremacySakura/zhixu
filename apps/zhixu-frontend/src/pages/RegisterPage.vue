<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { register } from '../services/apis/auth'
import AppButton from '../components/ui/AppButton.vue'
import BrandMark from '../components/ui/BrandMark.vue'

const router = useRouter()

const form = ref({
  username: '',
  email: '',
  password: '',
})

const showPassword = ref(false)
const passwordInputType = computed(() => (showPassword.value ? 'text' : 'password'))

const isSubmitting = ref(false)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

const validate = () => {
  if (!form.value.username.trim()) return '请输入用户名'
  if (!form.value.email.trim()) return '请输入邮箱'
  if (!emailRegex.test(form.value.email.trim())) return '邮箱格式不正确'
  if (!form.value.password) return '请输入密码'
  if (form.value.password.length < 6) return '密码至少 6 位'
  return null
}

const onSubmit = async () => {
  errorMessage.value = null
  successMessage.value = null

  const validationError = validate()
  if (validationError) {
    errorMessage.value = validationError
    return
  }

  isSubmitting.value = true
  try {
    const res = await register({
      username: form.value.username.trim(),
      email: form.value.email.trim(),
      password: form.value.password,
    })

    if (typeof res?.code === 'number' && res.code !== 0) {
      errorMessage.value = res.msg || '注册失败'
      return
    }

    successMessage.value = res?.msg || '注册成功'
    setTimeout(() => {
      router.push('/login')
    }, 600)
  } catch (error: unknown) {
    errorMessage.value = error instanceof Error ? error.message : '网络错误，请稍后重试'
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <!-- 认证独立页（DESIGN.md 原型⑥）：480px 单卡、无页头带。
       与登录页同构：品牌行 + 一张硬卡 + 卡内一条分隔线。 -->
  <div class="flex min-h-screen flex-col">
    <main
      id="main-content"
      tabindex="-1"
      class="flex flex-1 flex-col items-center justify-center px-4 py-12"
    >
      <div class="mb-6 flex w-full max-w-[480px] items-center justify-between">
        <BrandMark />
        <RouterLink
          to="/login"
          class="tap-target text-body font-semibold text-primary hover:underline"
        >
          登录
        </RouterLink>
      </div>

      <div class="cabinet-card w-full max-w-[480px]">
        <div class="px-8 py-9">
          <div class="mb-8">
            <h1 class="text-display mb-2">创建账号</h1>
            <p class="text-body text-label-secondary">加入知序，安全存储您的文件。</p>
          </div>

          <div
            v-if="errorMessage"
            class="mb-6 flex items-center gap-2 border-l-2 border-danger bg-danger-tint px-3.5 py-2.5 text-body font-medium text-danger"
            role="alert"
          >
            <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.8" />
              <path d="M12 8v5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              <circle cx="12" cy="16.2" r="1.1" fill="currentColor" />
            </svg>
            {{ errorMessage }}
          </div>
          <div
            v-if="successMessage"
            class="mb-6 flex items-center gap-2 border-l-2 border-primary bg-primary-tint px-3.5 py-2.5 text-body font-medium text-primary-pressed"
            role="status"
          >
            <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.8" />
              <path
                d="m8.5 12.2 2.4 2.4 4.6-5"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
            {{ successMessage }}
          </div>

          <form class="flex flex-col gap-6" @submit.prevent="onSubmit">
            <div class="flex flex-col">
              <label class="atom-field-label" for="username">用户名</label>
              <input
                id="username"
                v-model="form.username"
                name="username"
                type="text"
                placeholder="请输入唯一用户名"
                autocomplete="username"
                class="atom-field atom-tone-secondary"
                style="--atom-control-height: 44px; --atom-control-padding: 14px"
              />
            </div>

            <div class="flex flex-col">
              <label class="atom-field-label" for="email">邮箱地址</label>
              <input
                id="email"
                v-model="form.email"
                name="email"
                type="email"
                placeholder="name@example.com"
                autocomplete="email"
                class="atom-field atom-tone-secondary"
                style="--atom-control-height: 44px; --atom-control-padding: 14px"
              />
            </div>

            <div class="flex flex-col">
              <label class="atom-field-label" for="password">密码</label>
              <div class="relative flex items-center">
                <input
                  id="password"
                  v-model="form.password"
                  name="password"
                  :type="passwordInputType"
                  placeholder="请创建一个强密码"
                  autocomplete="new-password"
                  class="atom-field atom-tone-secondary pr-12"
                  style="--atom-control-height: 44px; --atom-control-padding: 14px"
                />
                <button
                  type="button"
                  :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                  class="absolute right-1 flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target-anchored hover:text-primary"
                  @click="showPassword = !showPassword"
                >
                  <svg
                    v-if="showPassword"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    class="atom-glyph"
                    style="--atom-glyph-size: 18px"
                    aria-hidden="true"
                  >
                    <path
                      d="M3 12C4.8 7.8 8.1 5.5 12 5.5C15.9 5.5 19.2 7.8 21 12C19.2 16.2 15.9 18.5 12 18.5C8.1 18.5 4.8 16.2 3 12Z"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linejoin="round"
                    />
                    <path
                      d="M12 15.2C10.2327 15.2 8.8 13.7673 8.8 12C8.8 10.2327 10.2327 8.8 12 8.8C13.7673 8.8 15.2 10.2327 15.2 12C15.2 13.7673 13.7673 15.2 12 15.2Z"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linejoin="round"
                    />
                  </svg>
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    class="atom-glyph"
                    style="--atom-glyph-size: 18px"
                    aria-hidden="true"
                  >
                    <path
                      d="M3 12C4.8 7.8 8.1 5.5 12 5.5C15.9 5.5 19.2 7.8 21 12C19.2 16.2 15.9 18.5 12 18.5C8.1 18.5 4.8 16.2 3 12Z"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linejoin="round"
                    />
                    <path
                      d="M9.6 14.4C9.03333 13.8333 8.7 12.9 8.8 12C8.9 11.1 9.2 10.4 9.6 9.6"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linecap="round"
                    />
                    <path
                      d="M4.5 4.5L19.5 19.5"
                      stroke="currentColor"
                      stroke-width="1.8"
                      stroke-linecap="round"
                    />
                  </svg>
                </button>
              </div>
            </div>

            <AppButton
              type="submit"
              tone="primary"
              size="lg"
              class="w-full"
              style="--atom-control-height: 44px"
              :loading="isSubmitting"
            >
              {{ isSubmitting ? '注册中...' : '注册' }}
            </AppButton>
          </form>
        </div>

        <div class="border-t border-hairline px-8 py-4 text-center text-body text-label-secondary">
          已经有账号了？
          <RouterLink class="tap-target font-semibold text-primary hover:underline" to="/login">
            返回登录
          </RouterLink>
        </div>
      </div>
    </main>
  </div>
</template>
