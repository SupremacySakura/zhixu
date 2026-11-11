<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { AnimatePresence, Motion, animate, type PanInfo } from 'motion-v'
import { springBouncy, springSnappy, fadeTransition } from '../utils/motion'
import { useRoute, useRouter } from 'vue-router'
import SideBar, { type NavItem } from '../components/bussiness/SideBar.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import { getProfile } from '../services/apis/auth'
import { useUserStore } from '../stores/user'

const NavItems: NavItem[] = [
  { label: '仪表盘', icon: 'material-symbols:dashboard', to: '/home/dashboard' },
  { label: '文件管理', icon: 'material-symbols:folder-open', to: '/home/files' },
  { label: '知识库', icon: 'material-symbols:menu-book-outline', to: '/home/knowledge' },
  {
    label: '知识图谱',
    icon: 'material-symbols:account-tree-outline',
    to: '/home/knowledge/graph',
  },
  { label: '对话', icon: 'material-symbols:chat-bubble-outline', to: '/home/chat' },
  { label: '取件码', icon: 'material-symbols:key-outline', to: '/home/pickup-codes' },
  { label: '上传', icon: 'material-symbols:cloud-upload', to: '/home/upload' },
  { label: '设置', icon: 'material-symbols:settings-outline', to: '/home/settings' },
]

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isMobileMenuOpen = ref(false)
const logoutConfirmOpen = ref(false)

// 进入主框架时静默刷新用户信息；401 由请求拦截器统一登出并跳转
async function loadProfile() {
  try {
    const res = await getProfile()
    if (res.code === 0 && res.data) userStore.setProfile(res.data)
  } catch {
    // 拉取失败保留已缓存的展示信息
  }
}

onMounted(() => {
  if (userStore.isLoggedIn) void loadProfile()
})

function confirmLogout() {
  logoutConfirmOpen.value = false
  userStore.logout()
  void router.push('/login')
}

const currentNavLabel = computed(() => {
  // 取最长匹配，避免 /home/knowledge/graph 被更短的「知识库」前缀抢先命中。
  const matched = NavItems.filter(
    item => route.path === item.to || route.path.startsWith(`${item.to}/`),
  ).sort((a, b) => b.to.length - a.to.length)[0]
  return matched?.label || '知序'
})

watch(
  () => route.fullPath,
  () => {
    isMobileMenuOpen.value = false
  },
)

// 抽屉拖拽关闭（DESIGN.md §2.4 手势：速度方向优先，动量投影辅助判断）
const panelEl = ref<HTMLElement | null>(null)
const setPanelRef = (instance: unknown) => {
  panelEl.value =
    (instance as { $el?: HTMLElement } | null)?.$el ?? (instance as HTMLElement) ?? null
}

const handleDrawerPanEnd = (_event: unknown, info: PanInfo) => {
  const shouldClose = info.velocity.x < -400 || (info.velocity.x <= 0 && info.offset.x < -80)
  if (shouldClose) {
    // AnimatePresence 退场：从当前拖拽位置连续滑出（x → -100%）
    isMobileMenuOpen.value = false
    return
  }
  if (panelEl.value) {
    // 未达关闭阈值：以弹簧弹回原位（可打断）
    animate(panelEl.value, { x: 0 }, springSnappy)
  }
}
</script>

<template>
  <div class="flex min-h-screen lg:h-screen">
    <!-- Skip to content link for accessibility -->
    <a
      href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-4 focus:py-2 focus:font-medium focus:text-white"
    >
      跳转到主要内容
    </a>
    <div class="hidden h-full lg:block">
      <SideBar
        :nav-items="NavItems"
        :brand-title="'知序'"
        :brand-subtitle="''"
        :user-name="userStore.profile?.username || ''"
        :user-email="userStore.profile?.email || ''"
        :user-avatar="userStore.profile?.avatar_url || ''"
        @logout="logoutConfirmOpen = true"
      />
    </div>

    <div class="flex min-h-screen min-w-0 flex-1 flex-col lg:min-h-0">
      <!-- 移动端顶栏：sticky + 半透明材质（DESIGN.md §3） -->
      <header
        class="material sticky top-0 z-20 flex h-[60px] items-center justify-between border-b border-hairline px-4 backdrop-blur-[20px] backdrop-saturate-[180%] lg:hidden"
      >
        <div class="min-w-0">
          <p class="text-[16px] font-bold tracking-[-0.01em] text-label">知序</p>
          <p class="truncate text-caption text-label-secondary">{{ currentNavLabel }}</p>
        </div>
        <button
          class="flex h-10 w-10 flex-none items-center justify-center rounded-sm border border-hairline bg-surface text-label transition-[transform,background-color] duration-150 hover:bg-surface-secondary active:scale-[0.92]"
          type="button"
          aria-label="打开导航菜单"
          @click="isMobileMenuOpen = true"
        >
          <Icon icon="material-symbols:menu-rounded" class="text-[20px]" />
        </button>
      </header>

      <router-view id="main-content" class="min-h-0 min-w-0 flex-1" tabindex="-1" />
    </div>

    <!-- 导航抽屉：面板弹簧滑入（damping ≈ 0.8 轻微回弹），可拖拽左滑关闭 -->
    <AnimatePresence>
      <Motion
        v-if="isMobileMenuOpen"
        key="mobile-drawer-layer"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-40 flex lg:hidden"
        @click="isMobileMenuOpen = false"
      >
        <div class="absolute inset-0 bg-scrim backdrop-blur-[6px]"></div>
        <Motion
          :ref="setPanelRef"
          :initial="{ x: '-100%' }"
          :animate="{ x: 0 }"
          :exit="{ x: '-100%' }"
          :transition="springBouncy"
          drag="x"
          :drag-constraints="{ left: -320, right: 0 }"
          :drag-elastic="{ left: 0.2, right: 0 }"
          :drag-momentum="false"
          class="relative flex h-full w-[min(18rem,86vw)] flex-col bg-surface shadow-popover"
          @click.stop
          @pan-end="handleDrawerPanEnd"
        >
          <div class="flex flex-none items-center justify-end border-b border-hairline px-3 py-2.5">
            <button
              class="flex h-10 w-10 items-center justify-center rounded-sm text-label-secondary transition-colors duration-150 hover:bg-surface-secondary hover:text-label"
              type="button"
              aria-label="关闭导航菜单"
              @click="isMobileMenuOpen = false"
            >
              <Icon icon="material-symbols:close-rounded" class="text-[20px]" />
            </button>
          </div>
          <div class="min-h-0 flex-1">
            <SideBar
              :nav-items="NavItems"
              :brand-title="'知序'"
              :brand-subtitle="''"
              :compact="true"
              :user-name="userStore.profile?.username || ''"
              :user-email="userStore.profile?.email || ''"
              :user-avatar="userStore.profile?.avatar_url || ''"
              @logout="logoutConfirmOpen = true"
            />
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
    <ConfirmDialog
      :model-value="logoutConfirmOpen"
      title="退出登录"
      message="退出后需要重新登录才能访问你的知识库。"
      confirm-text="退出登录"
      danger
      @cancel="logoutConfirmOpen = false"
      @confirm="confirmLogout"
    />
  </div>
</template>
