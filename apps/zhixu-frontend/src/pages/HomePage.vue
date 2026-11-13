<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppIcon from '../components/ui/AppIcon.vue'
import { AnimatePresence, Motion, animate, type PanInfo } from 'motion-v'
import { springBouncy, springSnappy, fadeTransition } from '../utils/motion'
import { useRoute, useRouter } from 'vue-router'
import SideBar, { type NavItem } from '../components/bussiness/SideBar.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import AppToast from '../components/ui/AppToast.vue'
import { getProfile } from '../services/apis/auth'
import { getKnowledgeCounts } from '../services/apis/knowledge'
import { useUserStore } from '../stores/user'

/**
 * 九个导航项分五组（DESIGN.md §Layout）。
 *
 * 分组不是装饰：外壳是索引卡盒，侧栏就是盒子侧面那排标签页。
 * 组名让「知识」与「来源」在视觉上先分开，再靠组内顺序表达依赖
 * （知识库 → 知识图谱 → 演化中心是一条工作流）。
 */
const NavItems: NavItem[] = [
  { label: '仪表盘', icon: 'material-symbols:dashboard', to: '/home/dashboard', group: '总览' },
  {
    label: '知识库',
    icon: 'material-symbols:menu-book-outline',
    to: '/home/knowledge',
    group: '知识',
  },
  {
    label: '知识图谱',
    icon: 'material-symbols:account-tree-outline',
    to: '/home/knowledge/graph',
    group: '知识',
  },
  {
    label: '演化中心',
    icon: 'material-symbols:sync-alt-rounded',
    to: '/home/evolution',
    group: '知识',
  },
  { label: '文件管理', icon: 'material-symbols:folder-open', to: '/home/files', group: '来源' },
  { label: '上传', icon: 'material-symbols:cloud-upload', to: '/home/upload', group: '来源' },
  {
    label: '取件码',
    icon: 'material-symbols:key-outline',
    to: '/home/pickup-codes',
    group: '来源',
  },
  { label: '对话', icon: 'material-symbols:chat-bubble-outline', to: '/home/chat', group: '对话' },
  { label: '设置', icon: 'material-symbols:settings-outline', to: '/home/settings', group: '系统' },
]

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isMobileMenuOpen = ref(false)
const logoutConfirmOpen = ref(false)

/**
 * 侧栏底部计数。取不到时保持 null，侧栏显示「索引 — · 分块 —」。
 * 为什么放在外壳而不是各页面：DESIGN.md §四段结构第 4 条
 * 「页面级计数与同步时间下沉到侧栏底部」—— 每页各算一遍会得到
 * 九个不同的数字，而且每页都要多背一次请求。
 */
const indexCount = ref<number | null>(null)
const chunkCount = ref<number | null>(null)

// 进入主框架时静默刷新用户信息；401 由请求拦截器统一登出并跳转
async function loadProfile() {
  try {
    const res = await getProfile()
    if (res.code === 0 && res.data) userStore.setProfile(res.data)
  } catch {
    // 拉取失败保留已缓存的展示信息
  }
}

async function loadCounts() {
  try {
    const counts = await getKnowledgeCounts()
    indexCount.value = counts.index
    chunkCount.value = counts.chunks
  } catch {
    // 计数是方位感，不是关键路径：失败就留占位符，不打扰用户
  }
}

onMounted(() => {
  if (!userStore.isLoggedIn) return
  void loadProfile()
  void loadCounts()
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
      class="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:bg-primary focus:px-4 focus:py-2 focus:font-medium focus:text-white"
    >
      跳转到主要内容
    </a>
    <div class="hidden h-full lg:block">
      <SideBar
        :nav-items="NavItems"
        :brand-title="'知序'"
        :brand-subtitle="''"
        :index-count="indexCount"
        :chunk-count="chunkCount"
        :user-name="userStore.profile?.username || ''"
        :user-email="userStore.profile?.email || ''"
        :user-avatar="userStore.profile?.avatar_url || ''"
        @logout="logoutConfirmOpen = true"
      />
    </div>

    <div class="flex min-h-screen min-w-0 flex-1 flex-col lg:min-h-0">
      <!-- 移动端全局条 60px：桌面端没有全局顶栏，这一条只在 <1024 存在。
           内容 = 「知序 + 当前页名」+ 40px 汉堡，与侧栏品牌区同一套字标语言。 -->
      <header
        class="sticky top-0 z-toolbar flex h-topbar flex-none items-center justify-between border-b border-hairline bg-surface px-4 lg:hidden"
      >
        <div class="flex min-w-0 items-baseline gap-2">
          <p class="flex-none font-semibold tracking-[0.2em] text-label">知序</p>
          <p class="truncate font-type text-micro text-label-tertiary">{{ currentNavLabel }}</p>
        </div>
        <button
          class="flex h-10 w-10 flex-none items-center justify-center border border-rule bg-surface text-label transition-colors tap-target hover:bg-surface-secondary"
          type="button"
          aria-label="打开导航菜单"
          @click="isMobileMenuOpen = true"
        >
          <AppIcon
            icon="material-symbols:menu-rounded"
            class="atom-glyph"
            :style="{ '--atom-glyph-size': '20px' }"
          />
        </button>
      </header>

      <router-view id="main-content" class="min-h-0 min-w-0 flex-1" tabindex="-1" />
    </div>

    <!-- 导航抽屉：面板弹簧滑入（damping ≈ 0.8 轻微回弹），可拖拽左滑关闭。
         宽度走 w-drawer = min(15rem, 84vw)：375 下恰好 240px，更宽视口封顶 240px。
         面板本身**不能**是 <aside>：ChatPage.test.ts 用 findAll('aside') 计数
         （桌面 1 / 抽屉开 2 / 关 1），外壳再贡献一个 aside 就会把这套计数打乱。 -->
    <AnimatePresence>
      <Motion
        v-if="isMobileMenuOpen"
        key="mobile-drawer-layer"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-menu flex lg:hidden"
        @click="isMobileMenuOpen = false"
      >
        <div class="absolute inset-0 bg-scrim"></div>
        <Motion
          :ref="setPanelRef"
          role="dialog"
          aria-modal="true"
          aria-label="导航菜单"
          :initial="{ x: '-100%' }"
          :animate="{ x: 0 }"
          :exit="{ x: '-100%' }"
          :transition="springBouncy"
          drag="x"
          :drag-constraints="{ left: -240, right: 0 }"
          :drag-elastic="{ left: 0.2, right: 0 }"
          :drag-momentum="false"
          class="relative flex h-full w-drawer flex-col bg-surface-secondary shadow-popover"
          @click.stop
          @pan-end="handleDrawerPanEnd"
        >
          <div class="flex flex-none items-center justify-end px-3 py-2">
            <button
              class="flex h-9 w-9 items-center justify-center text-label-secondary transition-colors tap-target hover:bg-nav-active hover:text-label"
              type="button"
              aria-label="关闭导航菜单"
              @click="isMobileMenuOpen = false"
            >
              <AppIcon
                icon="material-symbols:close-rounded"
                class="atom-glyph"
                :style="{ '--atom-glyph-size': '20px' }"
              />
            </button>
          </div>
          <div class="min-h-0 flex-1">
            <SideBar
              :nav-items="NavItems"
              :brand-title="'知序'"
              :brand-subtitle="''"
              :index-count="indexCount"
              :chunk-count="chunkCount"
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
    <!--
      全仓唯一的消息提示宿主：任何地方调 toast() 都在这里渲染。
      挂在外壳而不是各页面，是「界面上只有一个提示」的结构性保证。
    -->
    <AppToast />
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
