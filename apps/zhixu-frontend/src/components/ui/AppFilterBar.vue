<script setup lang="ts">
/**
 * 页面级筛选行（#39 窄屏收起）
 *
 * 为什么需要这一层：筛选控件是桌面工具。7 个下拉在 375px 里要么换行成三排把
 * 内容顶到屏幕外，要么被压成碎块。DESIGN.md 要求窄屏「收成单按钮 + 底部升起
 * 面板」—— 但 `FilterMenu` 本身不能动：它在全仓 21 处使用，其中 17 处在对话框 /
 * 表单 / 面板内部（那些地方没有「窄屏收起」的语义，收起来反而是 bug）。
 *
 * 所以这层只做一件事：**给页面级的筛选行换一套窄屏外壳**。
 * - ≥lg：插槽原样平铺，与接入前完全一致（桌面不受本组件影响）。
 * - <lg：只留一枚「筛选」按钮，点开从底部升起面板，面板里是同一批控件。
 *
 * 关键实现约束：**插槽只渲染一次**，宽窄切换纯靠 CSS（`hidden` ↔ `lg:flex`）。
 * 理由有两层：其一，插槽里绑的是父组件的 v-model，渲染两遍就是两份状态源，
 * 而其中一份永远藏在 `display:none` 里，改了不重绘；其二，页面把
 * `data-test="type-filter"` 这类钩子放在插槽里，渲染两遍会让 `wrapper.find`
 * 拿到哪一份变成实现细节 —— 现有 16 个钩子的测试契约会被动摇。
 *
 * 面板**不用 `<aside>`**：ChatPage.test.ts 用 `findAll('aside')` 做 1→2→1 计数，
 * 外壳里再冒出 aside 会把那套计数打乱。这里用 `role="dialog"`。
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 已启用的筛选数量，显示在窄屏按钮上（0 时不带角标）。 */
    activeCount?: number
    /** 窄屏按钮与面板标题的文案。 */
    label?: string
  }>(),
  { activeCount: 0, label: '筛选' },
)

const open = ref(false)

const buttonLabel = computed(() =>
  props.activeCount > 0 ? `${props.label} ${props.activeCount}` : props.label,
)

/** 面板升起时锁住背景滚动，收起时恢复（不锁会在面板后面滚页面）。 */
watch(open, value => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = value ? 'hidden' : ''
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') document.body.style.overflow = ''
})

function close() {
  open.value = false
}
</script>

<template>
  <!--
    根节点用 `contents`：桌面端这一层**完全不参与布局**，插槽里的控件直接成为
    父容器（shell-toolbar）的 flex 子项 —— 与接入 AppFilterBar 之前的 DOM 树
    逐项等价。否则这一层会变成一个 w-full 的块，把整排筛选顶到第二行。
  -->
  <div class="contents">
    <!-- 窄屏触发按钮：桌面隐藏。 -->
    <button
      type="button"
      data-test="filter-bar-trigger"
      class="flex min-h-9 items-center gap-1.5 border border-hairline bg-surface px-2.5 text-note font-semibold text-label-secondary transition-colors hover:text-label lg:hidden"
      :aria-expanded="open"
      @click="open = true"
    >
      <AppIcon icon="material-symbols:tune" />
      {{ buttonLabel }}
    </button>

    <!-- 遮罩：只在窄屏且面板打开时存在。 -->
    <div v-if="open" class="fixed inset-0 z-menu bg-scrim lg:hidden" @click="close"></div>

    <!--
      同一个插槽，两套外壳：
      桌面 → 普通 flex 行（hidden 被 lg:flex 覆盖）；
      窄屏 → 收起时 display:none，打开时变成底部升起的面板。
    -->
    <div
      data-test="filter-bar-panel"
      :role="open ? 'dialog' : undefined"
      :aria-modal="open ? 'true' : undefined"
      :aria-label="open ? label : undefined"
      :class="
        open
          ? 'fixed inset-x-0 bottom-0 z-menu flex max-h-[80vh] flex-col gap-2 overflow-y-auto border-t border-hairline bg-surface px-4 pb-6 pt-4 [&_button[aria-haspopup]]:w-full'
          : 'hidden'
      "
      class="lg:static lg:z-auto lg:contents lg:overflow-visible lg:border-0 lg:bg-transparent lg:p-0"
    >
      <!-- 面板自带的标题行与完成按钮：桌面隐藏。 -->
      <div class="mb-1 flex items-center justify-between lg:hidden">
        <p class="font-type text-micro text-label-tertiary">{{ label }}</p>
        <button
          type="button"
          class="flex h-9 w-9 items-center justify-center text-label-secondary tap-target hover:text-label"
          aria-label="关闭筛选"
          @click="close"
        >
          <AppIcon icon="material-symbols:close-rounded" />
        </button>
      </div>

      <slot />

      <button
        type="button"
        class="atom-btn atom-tone-primary atom-btn-lg mt-3 w-full lg:hidden"
        @click="close"
      >
        完成
      </button>
    </div>
  </div>
</template>
