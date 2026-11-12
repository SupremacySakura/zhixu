<script setup lang="ts">
/**
 * 长文阅读原型 · 变体 C —— 「双页对照」
 * 结构性差异：左右两页并列（原文 / 结构化摘录），中间用规则线分隔，像摊开的两张散页。
 * 用真实数据：左页原文，右页用 h2 提取的骨架 + 交叉引用。
 */
import { computed } from 'vue'
import type { KnowledgeItem } from '../services/apis/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'

const props = defineProps<{
  article: KnowledgeItem | null
  loading: boolean
  relations: { label: string; id: number; title: string }[]
  num: (id: number) => string
  onBack: () => void
}>()

const html = computed(() =>
  props.article ? renderKnowledgeContent(props.article.content, props.article.source_type) : '',
)
const outline = computed(() =>
  [...(props.article?.content ?? '').matchAll(/^##\s+(.+)$/gm)].map(m => m[1]!.trim()),
)
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]" @click="onBack">
        ← 知识库
      </button>
      <p class="font-type text-[10.5px] text-label-secondary">
        {{ article ? num(article.id) : '—' }} · 双页对照
      </p>
      <span class="ml-auto font-type text-[10.5px] text-label-secondary">摊开 · 左右各一页</span>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-6 py-6">
      <div
        class="grid min-h-full grid-cols-2 border border-[rgb(24_24_24_/_0.34)] bg-surface shadow-[2px_2px_0_rgb(24_24_24_/_0.13)]"
      >
        <div class="border-r border-rule px-7 py-6">
          <p class="font-type text-[9.5px] uppercase tracking-[0.22em] text-label-tertiary">
            左页 · 原文
          </p>
          <template v-if="article">
            <h1 class="mt-2 text-display">{{ article.title }}</h1>
            <div class="knowledge-markdown mt-5" v-html="html"></div>
          </template>
        </div>
        <div class="px-7 py-6">
          <p class="font-type text-[9.5px] uppercase tracking-[0.22em] text-label-tertiary">
            右页 · 骨架与引证
          </p>
          <ol class="mt-3 space-y-2">
            <li
              v-for="(heading, index) in outline"
              :key="heading"
              class="flex items-baseline gap-2.5"
            >
              <span class="font-type text-[10px] text-primary">{{
                String(index + 1).padStart(2, '0')
              }}</span>
              <span class="text-[14px] font-semibold text-label">{{ heading }}</span>
            </li>
          </ol>
          <p v-if="!outline.length" class="mt-3 text-[12px] text-label-tertiary">这篇没有分节。</p>

          <div class="mt-8 border-t border-rule pt-3">
            <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">
              交叉引用
            </p>
            <a
              v-for="rel in relations"
              :key="rel.id"
              href="#"
              class="mt-2 flex items-baseline gap-2 border border-[rgb(24_24_24_/_0.34)] bg-surface-tertiary px-2 py-1 text-[11px] no-underline"
              @click.prevent
            >
              <span class="font-type text-[9px] font-bold text-primary">{{ rel.label }}</span>
              <span class="font-type text-[9px] text-primary">{{
                String(rel.id).padStart(3, '0')
              }}</span>
              <span class="truncate">{{ rel.title }}</span>
            </a>
            <p v-if="!relations.length" class="mt-2 text-[11px] text-label-tertiary">暂无。</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
