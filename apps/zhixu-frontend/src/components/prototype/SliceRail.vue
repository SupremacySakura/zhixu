<script setup lang="ts">
/**
 * 原型侧栏：验证 #21 定案的 200px + 五组导航 + 底部索引计数。
 * 刻意不用图标、不用圆角、不用毛玻璃 —— 全靠字级、字重、左边条与格线。
 * 一次性代码（prototype 分支），不进 main。
 */
const groups = [
  { name: '总览', items: [{ label: '仪表盘' }, { label: '演化中心' }] },
  {
    name: '知识',
    items: [{ label: '知识库' }, { label: '知识图谱', on: true }, { label: '分块结构' }],
  },
  { name: '来源', items: [{ label: '文件管理' }, { label: '上传' }] },
  { name: '对话', items: [{ label: 'AI 问答' }] },
  { name: '系统', items: [{ label: '取件码' }, { label: '设置' }] },
]

defineProps<{ counts?: { index: number; chunks: number } }>()
</script>

<template>
  <aside
    class="hidden w-[200px] flex-none flex-col border-r border-hairline bg-surface-secondary lg:flex"
    aria-label="主导航"
  >
    <div class="border-b border-hairline px-4 pb-3 pt-4">
      <p class="text-[21px] font-semibold leading-6 tracking-[0.2em] text-label">知序</p>
      <p class="mt-1 font-type text-[10px] uppercase tracking-[0.18em] text-label-secondary">
        Kardex · 索引卡盒
      </p>
    </div>

    <nav class="min-h-0 flex-1 overflow-y-auto py-1">
      <div v-for="group in groups" :key="group.name" class="mt-3">
        <p class="px-3 pb-1 font-type text-[10px] uppercase tracking-[0.22em] text-label-tertiary">
          {{ group.name }}
        </p>
        <a
          v-for="item in group.items"
          :key="item.label"
          href="#"
          class="flex h-[29px] items-center border-l-[3px] pl-3 text-[13px] no-underline"
          :class="
            item.on
              ? 'border-primary bg-[#e6e2d4] font-semibold text-primary'
              : 'border-transparent text-label hover:bg-surface-tertiary'
          "
          @click.prevent
        >
          {{ item.label }}
        </a>
      </div>
    </nav>

    <div
      class="border-t border-hairline px-4 py-3 font-type text-[10px] leading-4 tracking-[0.06em] text-label-secondary"
    >
      <p>索引 {{ counts?.index ?? 31 }} · 分块 {{ counts?.chunks ?? 412 }}</p>
      <p>最近同步 22:04</p>
    </div>
  </aside>
</template>
