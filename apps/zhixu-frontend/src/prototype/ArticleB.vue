<script setup lang="ts">
/**
 * 长文阅读原型 · 变体 B —— 「全宽格线页 + 左侧目录栏」
 * 结构性差异：正文左对齐到栅格（不居中），左侧常驻本页小标题目录（从 markdown 提取 h2）。
 * 更像索引卡盒里的「散页装订」，而不是一本书。
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
const headings = computed(() => {
  const text = props.article?.content ?? ''
  return [...text.matchAll(/^##\s+(.+)$/gm)].map(m => m[1]!.trim()).slice(0, 8)
})
const chars = computed(() => props.article?.content.length ?? 0)
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]" @click="onBack">
        ← 知识库
      </button>
      <p
        class="min-w-0 flex-1 truncate font-type text-[10.5px] tracking-[0.06em] text-label-secondary"
      >
        {{ article ? `${num(article.id)} · ${article.title}` : '—' }}
      </p>
      <span class="font-type text-[10.5px] text-label-secondary">{{ chars }} 字</span>
    </header>

    <div class="flex min-h-0 flex-1">
      <!-- 左侧目录：本页小标题，等宽编号 -->
      <nav class="w-[200px] flex-none border-r border-hairline bg-surface-secondary px-4 py-5">
        <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">本页</p>
        <a
          v-for="(heading, index) in headings"
          :key="heading"
          href="#"
          class="mt-2 flex items-baseline gap-2 border-l-[3px] border-transparent pl-2 text-[11.5px] text-label-secondary no-underline hover:border-primary hover:text-label"
          @click.prevent
        >
          <span class="font-type text-[9px] text-label-tertiary">{{
            String(index + 1).padStart(2, '0')
          }}</span>
          <span class="min-w-0 truncate">{{ heading }}</span>
        </a>
        <p v-if="!headings.length" class="mt-2 text-[11px] text-label-tertiary">这篇没有小标题。</p>
      </nav>

      <div class="min-h-0 flex-1 overflow-y-auto">
        <div class="px-10 py-8">
          <template v-if="article">
            <div class="flex items-baseline gap-3 border-b border-rule pb-2">
              <span class="font-type text-[10px] tracking-[0.1em] text-primary">{{
                num(article.id)
              }}</span>
              <h1 class="min-w-0 flex-1 text-display">{{ article.title }}</h1>
              <span class="font-type text-[10px] text-label-tertiary"
                >版本 {{ article.version }}</span
              >
            </div>
            <div class="mt-6 flex gap-10">
              <div class="knowledge-markdown min-w-0 max-w-measure flex-1" v-html="html"></div>
              <aside class="w-[220px] flex-none border-l border-hairline pl-4">
                <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">
                  交叉引用
                </p>
                <a
                  v-for="rel in relations"
                  :key="rel.id"
                  href="#"
                  class="mt-2 flex gap-1.5 text-[11px] no-underline"
                  @click.prevent
                >
                  <span class="font-type text-[9px] font-bold text-primary">{{ rel.label }}</span>
                  <span class="min-w-0 truncate text-label-secondary">{{ rel.title }}</span>
                </a>
                <p v-if="!relations.length" class="mt-2 text-[11px] text-label-tertiary">暂无。</p>
              </aside>
            </div>
          </template>
          <p v-else-if="loading" class="text-body text-label-secondary">正在取出卡片…</p>
        </div>
      </div>
    </div>
  </div>
</template>
