<script setup lang="tsx">
import AppIcon from '../ui/AppIcon.vue'
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

/**
 * 侧栏导航项。
 *
 * `group` 是 #39 新增的：DESIGN.md §Layout 要求导航分五组（总览 / 知识 / 来源 / 对话 / 系统），
 * 每组一行 12px 等宽小组标。标成可选是为了两件事：
 * 1. 未标注的项落进「其他」兜底组，不会因为漏写字段就从界面上消失；
 * 2. 侧栏的既有测试传的是空数组/临时项，不该被分组字段绑架。
 */
export type NavItem = {
  label: string
  icon: string
  to: string
  group?: string
}

const props = withDefaults(
  defineProps<{
    brandTitle?: string
    brandSubtitle?: string
    navItems?: NavItem[]
    /** 已索引知识条数；未取到时为 null，底部计数行显示占位符而不是假数据。 */
    indexCount?: number | null
    /** 已分块总数；未取到时为 null。 */
    chunkCount?: number | null
    compact?: boolean
    userName?: string
    userEmail?: string
    userAvatar?: string
  }>(),
  {
    brandTitle: '知序',
    brandSubtitle: '',
    navItems: () => [],
    indexCount: null,
    chunkCount: null,
    compact: false,
    userName: '',
    userEmail: '',
    userAvatar: '',
  },
)

const emit = defineEmits<{ logout: [] }>()

const route = useRoute()

/**
 * 五个分组与它们的固定顺序（DESIGN.md §Layout）。
 * 顺序是外壳骨架的一部分，不随传入顺序漂移 —— 侧栏每次挂载都长得一样，
 * 位置记忆才成立。组内顺序仍由传入顺序决定。
 */
const GROUP_ORDER = ['总览', '知识', '来源', '对话', '系统'] as const
/** 未标注 group 的项落在这里，排在五组之后。 */
const FALLBACK_GROUP = '其他'

const groups = computed(() => {
  const buckets = new Map<string, NavItem[]>()
  for (const item of props.navItems ?? []) {
    const key = item.group?.trim() || FALLBACK_GROUP
    const bucket = buckets.get(key)
    if (bucket) bucket.push(item)
    else buckets.set(key, [item])
  }
  const ordered = [...GROUP_ORDER, FALLBACK_GROUP]
  return ordered
    .filter(name => buckets.has(name))
    .map(name => ({ name, items: buckets.get(name)! }))
})

const normalizePath = (path: string) => {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

// 取最长前缀命中的导航项，避免 /home/knowledge/graph 同时点亮「知识库」与「知识图谱」。
const activeTarget = computed(() => {
  const current = normalizePath(route.path)
  let matched = ''
  for (const item of props.navItems ?? []) {
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

/**
 * 导航项几何：29px 高、激活态 = 3px 蓝左边条 + #e6e2d4 底 + 蓝字。
 *
 * 为什么激活态不是 `bg-primary-tint` 整块填充：DESIGN.md:580 明确
 * 「表示选中：3px 蓝左边条 + #e6e2d4 底，不是换整个底色」——
 * 整块 tint 是旧世界的选中语言，已随换骨退役。
 * 未激活项保留一条同宽的透明左边条，否则激活瞬间文字会横跳 3px。
 */
const linkClass = (to: string) => {
  const base =
    'flex h-nav-item items-center gap-2.5 border-l-[3px] pl-[13px] pr-[14px] text-body font-medium transition-colors active:bg-nav-active active:text-primary-pressed'
  return isActive(to)
    ? `${base} border-l-primary bg-nav-active font-semibold text-primary`
    : `${base} border-l-transparent text-label-secondary hover:bg-nav-active/60 hover:text-label`
}

/** 千位分隔：1286 → 1,286。走 en-US 分组符，中文语境里数字分组习惯与西文一致。 */
const formatCount = (value: number | null | undefined) =>
  typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString('en-US') : '—'

/** 底部计数行的完整文案；空值时整行仍渲染，只是数字变成破折号。 */
const countsLine = computed(
  () => `索引 ${formatCount(props.indexCount)} · 分块 ${formatCount(props.chunkCount)}`,
)
</script>

<template>
  <aside
    :class="
      props.compact
        ? 'flex h-full w-full flex-col px-3 pb-3'
        : 'flex h-full w-rail shrink-0 flex-col border-r border-hairline bg-surface-secondary px-3 pb-3'
    "
  >
    <!-- 品牌区 62px：字标「知序」+ 16px 方章。
         字标走系统黑体（中文没有打字机字面，硬凑只会得到错字距的中西混排），
         方章是蓝底反白「序」—— 品牌不是贴上去的标志，是索引卡左上角的分类号。 -->
    <div data-test="brand" class="flex h-brand flex-none items-center gap-2 px-2">
      <h1 data-test="brand-wordmark" class="text-brand tracking-[0.2em] text-label">
        {{ brandTitle }}
      </h1>
      <span
        data-test="brand-seal"
        class="flex h-seal w-seal flex-none items-center justify-center bg-primary text-note font-bold text-surface-tertiary"
        aria-hidden="true"
        >序</span
      >
    </div>
    <p v-if="brandSubtitle" class="mb-1 px-2 font-type text-micro uppercase text-label-tertiary">
      {{ brandSubtitle }}
    </p>

    <nav
      class="mt-2 flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto"
      role="navigation"
      aria-label="主导航"
    >
      <div
        v-for="group in groups"
        :key="group.name"
        data-test="nav-group"
        :data-group="group.name"
        class="flex flex-col gap-0.5"
      >
        <p data-test="nav-group-title" class="px-2 pb-1 font-type text-micro text-label-tertiary">
          {{ group.name }}
        </p>
        <RouterLink
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          :class="linkClass(item.to)"
          role="link"
          :aria-current="isActive(item.to) ? 'page' : undefined"
        >
          <AppIcon
            :icon="item.icon"
            class="flex-none atom-glyph"
            :style="{ '--atom-glyph-size': '16px' }"
          />
          <span class="truncate">{{ item.label }}</span>
        </RouterLink>
      </div>
    </nav>

    <!-- 底部固定一行等宽计数：页面级计数与同步时间下沉到侧栏，页面里就不再有页脚。 -->
    <div
      data-test="shell-counts"
      class="mt-auto flex-none border-t border-hairline px-2 pt-2 font-type text-micro text-label-tertiary tabular-nums"
    >
      {{ countsLine }}
    </div>

    <!-- 当前用户：加载完成后渲染，退出登录由父级确认后执行 -->
    <div v-if="userName" class="mt-2 flex flex-none items-center gap-2 px-2 pb-0.5">
      <img
        v-if="userAvatar"
        :src="userAvatar"
        :alt="`${userName} 的头像`"
        class="h-7 w-7 flex-none object-cover"
      />
      <span
        v-else
        class="flex h-7 w-7 flex-none items-center justify-center bg-primary-tint text-note font-semibold text-primary-pressed"
        aria-hidden="true"
        >{{ avatarInitial }}</span
      >
      <div class="min-w-0 flex-1">
        <p class="truncate text-body font-semibold text-label">{{ userName }}</p>
        <p v-if="userEmail" class="truncate text-note text-label-tertiary">{{ userEmail }}</p>
      </div>
      <button
        type="button"
        aria-label="退出登录"
        title="退出登录"
        class="flex h-7 w-7 flex-none items-center justify-center text-label-secondary transition-colors tap-target hover:bg-nav-active hover:text-label"
        @click="emit('logout')"
      >
        <AppIcon
          icon="material-symbols:logout-rounded"
          class="atom-glyph"
          aria-hidden="true"
          :style="{ '--atom-glyph-size': '16px' }"
        />
      </button>
    </div>
  </aside>
</template>
