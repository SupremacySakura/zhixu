<script setup lang="ts">
/**
 * 索引列表原型 · 变体 C —— 「双栏密排卡」
 * 结构性差异：两列网格、卡更矮、交叉引用压到卡底一行文字，不再有右侧纸条。
 * 密度最高，一屏可见约 12 张。
 */
import type { KnowledgeItem } from '../services/apis/knowledge'

defineProps<{
  items: KnowledgeItem[]
  total: number
  relationsByItem: Map<number, { label: string; id: number; title: string }[]>
  vectorReady: number
  pending: number
  num: (id: number) => string
  excerpt: (item: KnowledgeItem) => string
  onOpen: (item: KnowledgeItem) => void
}>()
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-5 border-b border-hairline px-6">
      <div>
        <h1 class="text-title">知识库</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          {{ total }} 张卡 · {{ vectorReady }} 已向量化 · {{ pending }} 待处理
        </p>
      </div>
      <div class="ml-auto flex gap-2">
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">筛选 3</button>
        <button type="button" class="cabinet-btn cabinet-btn-primary h-8 px-3 text-[12.5px]">
          新建知识
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-6 py-4">
      <div class="grid grid-cols-2 gap-3">
        <article
          v-for="item in items"
          :key="item.id"
          class="flex h-[132px] cursor-pointer flex-col border border-[rgb(24_24_24_/_0.34)] bg-surface px-3 pb-2 pt-1.5 shadow-[2px_2px_0_rgb(24_24_24_/_0.13)]"
          @click="onOpen(item)"
        >
          <div class="flex items-baseline gap-2 border-b border-[rgb(31_78_121_/_0.3)] pb-1">
            <span class="font-type text-[10px] font-bold tracking-[0.1em] text-primary">{{
              num(item.id)
            }}</span>
            <h2 class="min-w-0 flex-1 truncate font-type text-[13px] font-bold tracking-[0.03em]">
              {{ item.title }}
            </h2>
          </div>
          <p class="mt-1 line-clamp-3 text-[11px] leading-[17px] text-[#3d3931]">
            {{ excerpt(item) }}
          </p>
          <div class="mt-auto flex items-center gap-2 text-[10px] text-label-secondary">
            <span class="font-type">{{ item.chunks_total }} 块</span>
            <span class="text-hairline">·</span>
            <span v-if="item.vector_state === 'partial'" class="font-bold text-danger">失败</span>
            <span v-else-if="item.vector_state === 'ready'" class="text-primary">已向量化</span>
            <span v-else>待向量化</span>
            <span class="ml-auto flex gap-1.5 overflow-hidden">
              <template
                v-for="rel in (relationsByItem.get(item.id) ?? []).slice(0, 2)"
                :key="rel.id"
              >
                <span class="whitespace-nowrap font-type text-[9px] text-primary"
                  >{{ rel.label }} {{ String(rel.id).padStart(3, '0') }}</span
                >
              </template>
            </span>
          </div>
        </article>
      </div>
      <p class="py-4 text-center font-type text-[10.5px] text-label-secondary">
        共 {{ total }} 张 · 每页 20
      </p>
    </div>
  </div>
</template>
