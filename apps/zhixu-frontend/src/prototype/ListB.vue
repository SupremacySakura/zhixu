<script setup lang="ts">
/**
 * 索引列表原型 · 变体 B —— 「索引行 + 右侧常驻详情栏」
 * 结构性差异：不是卡片流，而是一张紧凑索引表；交叉引用不贴在卡上，
 * 而是选中一行后在右侧栏里读。密度更高、一屏看得更多。
 */
import type { KnowledgeItem } from '../services/apis/knowledge'

defineProps<{
  items: KnowledgeItem[]
  total: number
  loading: boolean
  selectedId: number | null
  relationsByItem: Map<number, { label: string; id: number; title: string }[]>
  vectorReady: number
  pending: number
  shortDate: (v: string) => string
  num: (id: number) => string
  excerpt: (item: KnowledgeItem) => string
  onOpen: (item: KnowledgeItem) => void
  onSelect: (item: KnowledgeItem) => void
}>()
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-5 border-b border-hairline px-6">
      <div class="flex-none">
        <h1 class="text-title">知识库</h1>
        <p class="mt-0.5 font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
          索引 {{ total }} · 已向量化 {{ vectorReady }} · 待处理 {{ pending }}
        </p>
      </div>
      <label class="flex h-8 max-w-[380px] flex-1 items-center border border-rule bg-surface">
        <input
          class="h-full min-w-0 flex-1 bg-transparent px-2.5 text-[12.5px] outline-none placeholder:text-label-tertiary"
          placeholder="搜索索引"
          aria-label="搜索知识"
        />
      </label>
      <div class="ml-auto flex flex-none gap-2">
        <button type="button" class="cabinet-btn cabinet-btn-primary h-8 px-3 text-[12.5px]">
          新建知识
        </button>
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">导入</button>
      </div>
    </header>

    <div class="flex h-12 flex-none items-center gap-2 border-b border-hairline px-6">
      <span class="font-type text-[10px] uppercase tracking-[0.2em] text-label-tertiary">索引</span>
      <span class="text-[11.5px] text-label">按标题排序</span>
      <span class="ml-auto font-type text-[10.5px] text-label-secondary">
        显示 {{ items.length }} / {{ total }}
      </span>
    </div>

    <div class="flex min-h-0 flex-1">
      <!-- 索引表：无列线，只有行间 hairline；表头小写等宽 -->
      <div class="min-h-0 flex-1 overflow-y-auto">
        <table class="w-full border-collapse text-left">
          <thead>
            <tr class="border-b border-rule">
              <th
                class="px-6 py-2 font-type text-[9.5px] font-normal uppercase tracking-[0.18em] text-label-tertiary"
              >
                编号
              </th>
              <th
                class="py-2 font-type text-[9.5px] font-normal uppercase tracking-[0.18em] text-label-tertiary"
              >
                标题
              </th>
              <th
                class="w-[70px] py-2 text-right font-type text-[9.5px] font-normal uppercase tracking-[0.18em] text-label-tertiary"
              >
                分块
              </th>
              <th
                class="w-[92px] px-6 py-2 text-right font-type text-[9.5px] font-normal uppercase tracking-[0.18em] text-label-tertiary"
              >
                更新
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in items"
              :key="item.id"
              class="cursor-pointer border-b border-hairline"
              :class="item.id === selectedId ? 'bg-surface' : 'hover:bg-surface'"
              @click="onSelect(item)"
              @dblclick="onOpen(item)"
            >
              <td
                class="whitespace-nowrap px-6 py-2 font-type text-[10.5px] tracking-[0.06em] text-primary"
              >
                {{ num(item.id) }}
              </td>
              <td class="py-2 pr-3 text-[13px] text-label">
                <span class="flex items-center gap-1.5">
                  <span
                    v-if="item.vector_state === 'partial'"
                    class="h-[6px] w-[6px] flex-none bg-danger"
                    aria-label="向量失败"
                  ></span>
                  {{ item.title }}
                </span>
              </td>
              <td class="py-2 text-right font-type text-[11px] tabular-nums text-label-secondary">
                {{ item.chunks_total }}
              </td>
              <td
                class="whitespace-nowrap px-6 py-2 text-right font-type text-[10px] text-label-tertiary"
              >
                {{ shortDate(item.updated_at) }}
              </td>
            </tr>
          </tbody>
        </table>
        <p class="px-6 py-4 font-type text-[10.5px] text-label-secondary">共 {{ total }} 张卡片</p>
      </div>

      <!-- 右侧常驻详情栏：选中行的摘录 + 交叉引用 -->
      <aside class="w-[320px] flex-none border-l border-hairline bg-surface-secondary p-4">
        <template v-if="items.find(i => i.id === selectedId)">
          <p class="font-type text-[10px] uppercase tracking-[0.2em] text-label-tertiary">
            抽出的卡
          </p>
          <h2 class="mt-1 text-title">{{ items.find(i => i.id === selectedId)?.title }}</h2>
          <p class="mt-2 line-clamp-6 text-[12px] leading-5 text-label-secondary">
            {{ excerpt(items.find(i => i.id === selectedId)!) }}
          </p>
          <div class="mt-4 border-t border-hairline pt-3">
            <p class="font-type text-[10px] uppercase tracking-[0.2em] text-label-tertiary">
              交叉引用
            </p>
            <div
              v-for="rel in relationsByItem.get(selectedId!) ?? []"
              :key="rel.id"
              class="mt-1.5 flex items-baseline gap-1.5 border border-[rgb(24_24_24_/_0.34)] bg-surface-tertiary px-1.5 py-0.5"
            >
              <span class="font-type text-[9px] font-bold text-primary">{{ rel.label }}</span>
              <span class="font-type text-[9px] text-primary">{{
                String(rel.id).padStart(3, '0')
              }}</span>
              <span class="truncate text-[10px]">{{ rel.title }}</span>
            </div>
            <p
              v-if="!(relationsByItem.get(selectedId!) ?? []).length"
              class="mt-1 text-[10px] text-label-tertiary"
            >
              这张卡还没有交叉引用。
            </p>
          </div>
          <button
            type="button"
            class="cabinet-btn mt-4 h-8 w-full justify-center text-[12px]"
            @click="onOpen(items.find(i => i.id === selectedId)!)"
          >
            打开全文
          </button>
        </template>
        <p v-else class="text-[12px] text-label-tertiary">选中左侧任意一张卡查看摘录与交叉引用。</p>
      </aside>
    </div>
  </div>
</template>
