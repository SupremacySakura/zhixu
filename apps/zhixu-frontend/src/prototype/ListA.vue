<script setup lang="ts">
/**
 * 索引列表原型 · 变体 A —— 「硬卡 + 右侧交叉引用纸条」（招牌动作的正面呈现）
 * 结构：页头带 64 → 工具栏 48（吸顶）→ 一列硬卡，卡内左正文右纸条。
 */
import type { KnowledgeItem } from '../services/apis/knowledge'

defineProps<{
  items: KnowledgeItem[]
  total: number
  loading: boolean
  relationsByItem: Map<number, { label: string; id: number; title: string }[]>
  vectorReady: number
  pending: number
  failed: number
  shortDate: (v: string) => string
  num: (id: number) => string
  excerpt: (item: KnowledgeItem) => string
  onOpen: (item: KnowledgeItem) => void
}>()
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <!-- 页头带 64：标题 19/26/700 + 等宽计数；右端 1 主 + 2 次 -->
    <header class="flex h-16 flex-none items-center gap-5 border-b border-hairline px-6">
      <div class="flex-none">
        <h1 class="text-title">知识库</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          {{ total }} 条 · 已向量化 {{ vectorReady }} · 待处理 {{ pending
          }}<span v-if="failed"> · 失败 {{ failed }}</span>
        </p>
      </div>
      <label
        class="flex h-8 max-w-[420px] flex-1 items-center border border-rule bg-surface shadow-[2px_2px_0_rgb(24_24_24_/_0.1)]"
      >
        <input
          class="h-full min-w-0 flex-1 bg-transparent px-2.5 text-[12.5px] text-label outline-none placeholder:text-label-tertiary"
          placeholder="搜索标题、标签或编号"
          aria-label="搜索知识"
        />
        <span
          class="mr-2 border border-hairline bg-surface-tertiary px-1 font-type text-[10px] text-label-secondary"
          >⌘K</span
        >
      </label>
      <div class="ml-auto flex flex-none gap-2">
        <button type="button" class="cabinet-btn cabinet-btn-primary h-8 px-3 text-[12.5px]">
          <span class="font-type">+</span> 新建知识
        </button>
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">从文件加入</button>
      </div>
    </header>

    <!-- 工具栏 48 吸顶：筛选 26px 直角 + 右端命中数 -->
    <div
      class="sticky top-0 z-[20] flex h-12 flex-none items-center gap-2 border-b border-hairline bg-canvas px-6"
    >
      <button
        type="button"
        class="flex h-[26px] items-center gap-2 border border-hairline bg-surface px-2 text-[11.5px]"
      >
        <span class="text-[11px] tracking-[0.04em] text-label-tertiary">状态</span><span>全部</span>
        <span class="font-type text-[8px] text-label-tertiary">▾</span>
      </button>
      <button
        type="button"
        class="flex h-[26px] items-center gap-2 border border-hairline bg-surface px-2 text-[11.5px]"
      >
        <span class="text-[11px] tracking-[0.04em] text-label-tertiary">向量</span><span>全部</span>
        <span class="font-type text-[8px] text-label-tertiary">▾</span>
      </button>
      <button
        type="button"
        class="flex h-[26px] items-center gap-2 border border-hairline bg-surface px-2 text-[11.5px]"
      >
        <span class="text-[11px] tracking-[0.04em] text-label-tertiary">来源</span><span>全部</span>
        <span class="font-type text-[8px] text-label-tertiary">▾</span>
      </button>
      <button
        type="button"
        class="flex h-[26px] items-center gap-2 border border-primary bg-surface px-2 text-[11.5px] shadow-[2px_2px_0_rgb(31_78_121_/_0.18)]"
      >
        <span class="text-[11px] tracking-[0.04em] text-label-tertiary">排序</span>
        <span class="font-semibold text-primary">最近更新</span>
        <span class="font-type text-[8px] text-primary">▾</span>
      </button>
      <span class="ml-auto font-type text-[10.5px] tracking-[0.08em] text-label-secondary">
        命中 <b class="text-[12px] text-label">{{ items.length }}</b> / {{ total }}
      </span>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-6 pt-4">
      <p v-if="loading" class="py-10 text-center text-body text-label-secondary">正在取卡片…</p>
      <article
        v-for="item in items"
        :key="item.id"
        class="mb-3 flex h-[118px] cursor-pointer border border-[rgb(24_24_24_/_0.34)] bg-surface shadow-[2px_2px_0_rgb(24_24_24_/_0.13),3px_3px_0_rgb(24_24_24_/_0.05)]"
        @click="onOpen(item)"
      >
        <div class="flex min-w-0 flex-1 flex-col px-3.5 pb-2 pt-1.5">
          <div class="flex items-baseline gap-2.5 border-b border-[rgb(31_78_121_/_0.3)] pb-1.5">
            <span class="font-type text-[10.5px] font-bold tracking-[0.1em] text-primary">{{
              num(item.id)
            }}</span>
            <h2 class="min-w-0 flex-1 truncate font-type text-[14px] font-bold tracking-[0.03em]">
              {{ item.title }}
            </h2>
          </div>
          <p
            class="mt-1 line-clamp-3 bg-[repeating-linear-gradient(to_bottom,transparent_0_17px,rgb(31_78_121_/_0.2)_17px_18px)] text-[11.5px] leading-[18px] text-[#3d3931]"
          >
            {{ excerpt(item) }}
          </p>
          <div
            class="mt-auto flex items-center gap-2.5 whitespace-nowrap pt-0.5 text-[10.5px] text-label-secondary"
          >
            <span
              >分块
              <b class="font-type text-[11px] text-[#2f2c26]">{{ item.chunks_total }}</b></span
            >
            <span class="text-hairline">·</span>
            <span v-if="item.vector_state === 'ready'" class="font-semibold text-primary"
              >已向量化</span
            >
            <span v-else-if="item.vector_state === 'partial'" class="font-bold text-danger"
              >失败</span
            >
            <span v-else>待向量化</span>
            <span class="text-hairline">·</span>
            <span>索引 {{ item.id }}</span>
          </div>
        </div>
        <aside
          class="flex w-[176px] flex-none flex-col justify-between border-l border-hairline px-2.5 py-2"
        >
          <div>
            <div
              v-for="rel in relationsByItem.get(item.id) ?? []"
              :key="`${rel.id}-${rel.label}`"
              class="-mb-px ml-[5px] flex h-[17px] items-baseline gap-1.5 overflow-hidden border border-[rgb(24_24_24_/_0.34)] bg-surface-tertiary px-1.5 shadow-[1px_1px_0_rgb(24_24_24_/_0.12)]"
            >
              <span
                class="flex-none text-[9.5px] font-bold tracking-[0.04em]"
                :class="rel.label === '矛盾' ? 'text-label' : 'text-primary'"
                >{{ rel.label }}</span
              >
              <span
                class="flex-none font-type text-[9px] font-bold tracking-[0.05em] text-primary"
                >{{ String(rel.id).padStart(3, '0') }}</span
              >
              <span
                class="overflow-hidden text-ellipsis whitespace-nowrap text-[9.5px] text-[#2e2b25]"
                >{{ rel.title }}</span
              >
            </div>
            <p
              v-if="!(relationsByItem.get(item.id) ?? []).length"
              class="text-[9.5px] text-label-tertiary"
            >
              暂无交叉引用
            </p>
          </div>
          <span
            class="self-end border border-primary bg-[rgb(31_78_121_/_0.05)] px-1.5 font-type text-[10px] tracking-[0.08em] text-primary"
          >
            {{ shortDate(item.updated_at) }}
          </span>
        </aside>
      </article>
      <p class="py-4 text-center font-type text-[10.5px] text-label-secondary">
        第一页 · 共 {{ total }} 条 · 每页 20 条
      </p>
    </div>
  </div>
</template>
