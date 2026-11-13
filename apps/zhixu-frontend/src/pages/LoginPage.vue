<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { login } from '../services/apis/auth'
import { useUserStore } from '../stores/user'
const router = useRouter()
const route = useRoute()

const form = ref({
  username: '',
  password: '',
})

const showPassword = ref(false)
const passwordInputType = computed(() => (showPassword.value ? 'text' : 'password'))

const isSubmitting = ref(false)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)
const userStore = useUserStore()
const validate = () => {
  if (!form.value.username.trim()) return '请输入用户名'
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
    const res = await login({
      username: form.value.username.trim(),
      password: form.value.password,
    })
    if (typeof res?.code === 'number' && res.code !== 0) {
      errorMessage.value = res.msg || '登录失败'
      return
    }

    const token = res?.data?.token
    if (!token) {
      errorMessage.value = res?.msg || '登录失败：未获取到 token'
      return
    }
    userStore.setToken(token)
    // 清掉可能存在的上一个账号的缓存信息，进入主页后会重新拉取
    userStore.setProfile(null)

    successMessage.value = res?.msg || '登录成功'
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/home'
    setTimeout(() => {
      router.push(redirect)
    }, 200)
  } catch (error: unknown) {
    errorMessage.value = error instanceof Error ? error.message : '网络错误，请稍后重试'
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <!-- 顶部品牌栏：半透明材质，内容滚动时从底下穿过 -->
    <header
      class="material sticky top-0 z-20 flex h-[60px] items-center justify-between border-b border-hairline px-6 lg:px-10"
    >
      <div class="flex items-center gap-2.5">
        <svg
          class="h-7 w-7 text-primary"
          fill="none"
          viewBox="0 0 48 48"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M13.8261 17.4264C16.7203 18.1174 20.2244 18.5217 24 18.5217C27.7756 18.5217 31.2797 18.1174 34.1739 17.4264C36.9144 16.7722 39.9967 15.2331 41.3563 14.1648L24.8486 40.6391C24.4571 41.267 23.5429 41.267 23.1514 40.6391L6.64374 14.1648C8.00331 15.2331 11.0856 16.7722 13.8261 17.4264Z"
            fill="currentColor"
          />
          <path
            clip-rule="evenodd"
            d="M39.998 12.236C39.9944 12.2537 39.9875 12.2845 39.9748 12.3294C39.9436 12.4399 39.8949 12.5741 39.8346 12.7175C39.8168 12.7597 39.7989 12.8007 39.7813 12.8398C38.5103 13.7113 35.9788 14.9393 33.7095 15.4811C30.9875 16.131 27.6413 16.5217 24 16.5217C20.3587 16.5217 17.0125 16.131 14.2905 15.4811C12.0012 14.9346 9.44505 13.6897 8.18538 12.8168C8.17384 12.7925 8.16216 12.767 8.15052 12.7408C8.09919 12.6249 8.05721 12.5114 8.02977 12.411C8.00356 12.3152 8.00039 12.2667 8.00004 12.2612C8.00004 12.261 8 12.2607 8.00004 12.2612C8.00004 12.2359 8.0104 11.9233 8.68485 11.3686C9.34546 10.8254 10.4222 10.2469 11.9291 9.72276C14.9242 8.68098 19.1919 8 24 8C28.8081 8 33.0758 8.68098 36.0709 9.72276C37.5778 10.2469 38.6545 10.8254 39.3151 11.3686C39.9006 11.8501 39.9857 12.1489 39.998 12.236ZM4.95178 15.2312L21.4543 41.6973C22.6288 43.5809 25.3712 43.5809 26.5457 41.6973L43.0534 15.223C43.0709 15.1948 43.0878 15.1662 43.104 15.1371L41.3563 14.1648C43.104 15.1371 43.1038 15.1374 43.104 15.1371L43.1051 15.135L43.1065 15.1325L43.1101 15.1261L43.1199 15.1082C43.1276 15.094 43.1377 15.0754 43.1497 15.0527C43.1738 15.0075 43.2062 14.9455 43.244 14.8701C43.319 14.7208 43.4196 14.511 43.5217 14.2683C43.6901 13.8679 44 13.0689 44 12.2609C44 10.5573 43.003 9.22254 41.8558 8.2791C40.6947 7.32427 39.1354 6.55361 37.385 5.94477C33.8654 4.72057 29.133 4 24 4C18.867 4 14.1346 4.72057 10.615 5.94478C8.86463 6.55361 7.30529 7.32428 6.14419 8.27911C4.99695 9.22255 3.99999 10.5573 3.99999 12.2609C3.99999 13.1275 4.29264 13.9078 4.49321 14.3607C4.60375 14.6102 4.71348 14.8196 4.79687 14.9689C4.83898 15.0444 4.87547 15.1065 4.9035 15.1529C4.91754 15.1762 4.92954 15.1957 4.93916 15.2111L4.94662 15.223L4.95178 15.2312ZM35.9868 18.996L24 38.22L12.0131 18.996C12.4661 19.1391 12.9179 19.2658 13.3617 19.3718C16.4281 20.1039 20.0901 20.5217 24 20.5217C27.9099 20.5217 31.5719 20.1039 34.6383 19.3718C35.082 19.2658 35.5339 19.1391 35.9868 18.996Z"
            fill="currentColor"
            fill-rule="evenodd"
          />
        </svg>
        <span class="text-title font-bold tracking-[-0.01em]">知序</span>
      </div>
      <RouterLink
        to="/register"
        class="flex h-9 items-center px-3 text-sm font-semibold text-primary transition-colors hover:bg-primary-tint"
      >
        注册
      </RouterLink>
    </header>

    <main class="flex flex-1 items-center justify-center px-4 py-12">
      <div class="w-full max-w-[400px] overflow-hidden bg-surface shadow-card">
        <div class="px-8 pb-0 pt-9">
          <div class="mb-7">
            <h1 class="text-display mb-2">欢迎回来</h1>
            <p class="text-body text-label-secondary">登录以继续使用知序。</p>
          </div>

          <div
            v-if="errorMessage"
            class="mb-5 flex items-center gap-2 bg-danger-tint px-3.5 py-2.5 text-body font-medium text-danger"
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
            class="mb-5 flex items-center gap-2 bg-primary-tint px-3.5 py-2.5 text-body font-medium text-primary-pressed"
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

          <form class="flex flex-col gap-4.5" @submit.prevent="onSubmit">
            <div class="flex flex-col gap-1.5">
              <label class="text-body font-semibold text-label-secondary" for="username">
                用户名
              </label>
              <input
                id="username"
                v-model="form.username"
                name="username"
                type="text"
                placeholder="请输入用户名"
                autocomplete="username"
                class="h-11 w-full border border-hairline bg-surface-secondary px-3.5 text-lead font-normal text-label transition-[border-color,background-color,box-shadow] placeholder:text-label-tertiary focus:border-primary focus:bg-surface"
              />
            </div>

            <div class="flex flex-col gap-1.5">
              <label class="text-body font-semibold text-label-secondary" for="password">
                密码
              </label>
              <div class="relative flex items-center">
                <input
                  id="password"
                  v-model="form.password"
                  name="password"
                  :type="passwordInputType"
                  placeholder="请输入密码"
                  autocomplete="current-password"
                  class="h-11 w-full border border-hairline bg-surface-secondary px-3.5 pr-11 text-lead font-normal text-label transition-[border-color,background-color,box-shadow] placeholder:text-label-tertiary focus:border-primary focus:bg-surface"
                />
                <button
                  type="button"
                  :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                  class="absolute right-1.5 flex h-8 w-8 items-center justify-center text-label-tertiary transition-colors tap-target-anchored hover:bg-primary-tint hover:text-primary"
                  @click="showPassword = !showPassword"
                >
                  <svg
                    v-if="showPassword"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    class="h-[18px] w-[18px]"
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
                    class="h-[18px] w-[18px]"
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

            <button
              class="mt-1.5 flex h-12 w-full items-center justify-center text-read font-semibold transition-colors disabled:opacity-60 knowledge-button-primary"
              type="submit"
              :disabled="isSubmitting"
            >
              {{ isSubmitting ? '登录中...' : '登录' }}
            </button>
          </form>

          <div
            class="-mx-8 mt-7 border-t border-hairline bg-surface-secondary px-8 py-4.5 text-center text-body text-label-secondary"
          >
            还没有账号？
            <RouterLink class="font-semibold text-primary hover:underline" to="/register">
              立即注册
            </RouterLink>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>
