<script setup lang="tsx">
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

export type NavItem = {
  label: string
  icon: string
  to: string
}

const props = withDefaults(
  defineProps<{
    brandTitle?: string
    brandSubtitle?: string
    navItems?: NavItem[]
    storagePercent?: number
    storageDetail?: string
    compact?: boolean
    userName?: string
    userEmail?: string
    userAvatar?: string
  }>(),
  {
    brandTitle: '知序',
    brandSubtitle: 'Personal Knowledge Base',
    storagePercent: 0,
    storageDetail: '',
    compact: false,
    userName: '',
    userEmail: '',
    userAvatar: '',
  },
)

const emit = defineEmits<{ logout: [] }>()

const route = useRoute()

const defaultNavItems: NavItem[] = [
  { label: 'Dashboard', icon: 'material-symbols:dashboard', to: '/home/dashboard' },
  { label: 'File Manager', icon: 'material-symbols:folder-open', to: '/home/files' },
  { label: 'Pickup Codes', icon: 'material-symbols:key-outline', to: '/home/pickup-codes' },
  { label: 'Analytics', icon: 'material-symbols:bar-chart', to: '/home/analytics' },
  { label: 'Settings', icon: 'material-symbols:settings', to: '/home/settings' },
]

const items = computed(() => props.navItems ?? defaultNavItems)

// 用量 ≥ 90% 视为空间不足（DESIGN.md：warning 语义）
const isStorageWarning = computed(() => props.storagePercent >= 90)
const storagePercentClamped = computed(() => Math.min(100, Math.max(0, props.storagePercent)))

const normalizePath = (path: string) => {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

// 取最长前缀命中的导航项，避免 /home/knowledge/graph 同时点亮「知识库」与「知识图谱」。
const activeTarget = computed(() => {
  const current = normalizePath(route.path)
  let matched = ''
  for (const item of items.value) {
    const target = normalizePath(item.to)
    const hit = current === target || (target !== '/' && current.startsWith(`${target}/`))
    if (hit && target.length > matched.length) matched = target
  }
  return matched
})

const isActive = (to: string) =>
  activeTarget.value !== '' && normalizePath(to) === activeTarget.value

// 无头像时使用用户名首字符占位（中英文均取第一个字符）
const avatarInitial = computed(() => (props.userName.trim()[0] || '?').toUpperCase())

const linkClass = (to: string) => {
  const base =
    'flex h-10 items-center gap-3 rounded-sm px-3 text-[15px] font-medium transition-colors duration-150 focus:outline-none active:bg-primary/20 active:text-primary-pressed'
  return isActive(to)
    ? `${base} bg-primary-tint font-semibold text-primary`
    : `${base} text-label-secondary hover:bg-surface-secondary hover:text-label`
}
</script>

<template>
  <aside
    :class="
      props.compact
        ? 'flex h-full w-full flex-col p-4'
        : 'material flex h-full w-64 shrink-0 flex-col border-r border-hairline p-6 backdrop-blur-[20px] backdrop-saturate-[180%]'
    "
  >
    <div class="flex items-center gap-2.5 px-2 py-0.5">
      <svg
        class="h-7 w-7 flex-none text-primary"
        :class="props.compact && 'h-6 w-6'"
        fill="none"
        viewBox="0 0 48 48"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
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
      <h1 class="text-[17px] font-bold tracking-[-0.01em] text-label">
        {{ brandTitle }}
      </h1>
    </div>
    <p
      v-if="brandSubtitle"
      class="mt-0.5 px-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-primary"
    >
      {{ brandSubtitle }}
    </p>

    <nav class="mt-6 flex flex-col gap-1" role="navigation" aria-label="主导航">
      <RouterLink
        v-for="item in items"
        :key="item.to"
        :to="item.to"
        :class="linkClass(item.to)"
        role="link"
        :aria-current="isActive(item.to) ? 'page' : undefined"
      >
        <Icon :icon="item.icon" class="flex-none text-[20px]" />
        <span class="whitespace-nowrap">{{ item.label }}</span>
      </RouterLink>
    </nav>

    <!-- 存储指示：父级传入 storageDetail 时渲染（默认隐藏，避免展示假数据） -->
    <div class="mt-auto">
      <div v-if="storageDetail" class="px-2 pb-0.5 pt-4">
        <div class="mb-2 flex items-baseline justify-between">
          <span class="text-subhead text-label-secondary">存储空间</span>
          <span
            class="text-[15px] font-semibold"
            :class="isStorageWarning ? 'text-danger' : 'text-label'"
          >
            {{ storagePercentClamped }}%
          </span>
        </div>
        <div class="h-1.5 overflow-hidden rounded-full bg-surface-tertiary">
          <div
            class="h-full rounded-full transition-[width] duration-500 ease-out"
            :class="isStorageWarning ? 'bg-warning' : 'bg-primary'"
            :style="{ width: `${storagePercentClamped}%` }"
          />
        </div>
        <div class="mt-2 flex items-center justify-between gap-2">
          <p class="text-caption text-label-tertiary">{{ storageDetail }}</p>
          <span
            v-if="isStorageWarning"
            class="inline-flex h-[22px] flex-none items-center rounded-full bg-warning-tint px-2.5 text-caption font-semibold text-warning-strong"
          >
            空间不足
          </span>
        </div>
      </div>

      <!-- 当前用户：加载完成后渲染，退出登录由父级确认后执行 -->
      <div
        v-if="userName"
        class="mt-3 flex items-center gap-2.5 border-t border-hairline px-2 pb-0.5 pt-3"
      >
        <img
          v-if="userAvatar"
          :src="userAvatar"
          :alt="`${userName} 的头像`"
          class="h-9 w-9 flex-none rounded-full object-cover"
        />
        <span
          v-else
          class="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-primary-tint text-[15px] font-semibold text-primary-pressed"
          aria-hidden="true"
          >{{ avatarInitial }}</span
        >
        <div class="min-w-0 flex-1">
          <p class="truncate text-subhead font-semibold text-label">{{ userName }}</p>
          <p v-if="userEmail" class="truncate text-caption text-label-tertiary">{{ userEmail }}</p>
        </div>
        <button
          type="button"
          aria-label="退出登录"
          title="退出登录"
          class="flex h-9 w-9 flex-none items-center justify-center rounded-sm text-label-secondary transition-[background-color,color,transform] duration-150 hover:bg-surface-secondary hover:text-label focus-visible:outline-2 focus-visible:outline-primary active:scale-[0.95]"
          @click="emit('logout')"
        >
          <Icon icon="material-symbols:logout-rounded" class="text-[18px]" aria-hidden="true" />
        </button>
      </div>
    </div>
  </aside>
</template>
