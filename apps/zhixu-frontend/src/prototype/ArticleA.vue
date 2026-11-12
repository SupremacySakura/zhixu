<script setup lang="ts">
/**
 * 长文阅读原型 · 变体 A —— 「居中书页 + 右侧页边注」
 * 主体 measure(640px) 居中；右侧页边注列放交叉引用与元数据（像批注页边）。
 */
import type { KnowledgeItem } from '../services/apis/knowledge'
import { computed } from 'vue'
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
const chars = computed(() => props.article?.content.length ?? 0)
const minutes = computed(() => Math.max(1, Math.round(chars.value / 350)))
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <header class="flex h-16 flex-none items-center gap-4 border-b border-hairline px-6">
      <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]" @click="onBack">
        ← 知识库
      </button>
      <p class="font-type text-[10.5px] tracking-[0.06em] text-label-secondary">
        知识库 / {{ article ? num(article.id) : '—' }}
      </p>
      <p class="ml-auto font-type text-[10.5px] text-label-secondary">
        {{ chars }} 字 · 约 {{ minutes }} 分钟
      </p>
      <div class="flex gap-2">
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">编辑</button>
        <button type="button" class="cabinet-btn h-8 px-3 text-[12.5px]">分块</button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto">
      <div class="mx-auto flex w-full max-w-wide gap-10 px-6 py-8">
        <article class="min-w-0 max-w-measure flex-1">
          <p v-if="loading" class="text-body text-label-secondary">正在取出卡片…</p>
          <template v-else-if="article">
            <p class="font-type text-[10px] uppercase tracking-[0.22em] text-label-tertiary">
              {{ num(article.id) }} · 版本 {{ article.version }}
            </p>
            <h1 class="mt-2 text-display">{{ article.title }}</h1>
            <div class="mt-3 border-t border-rule"></div>
            <div class="knowledge-markdown mt-6" v-html="html"></div>
          </template>
        </article>

        <aside class="w-[240px] flex-none pt-10">
          <div class="border-t border-rule pt-3">
            <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">
              页边注 · 交叉引用
            </p>
            <a
              v-for="rel in relations"
              :key="rel.id"
              href="#"
              class="mt-2 flex items-baseline gap-1.5 border-l-2 border-primary pl-2 text-[11px] no-underline"
              @click.prevent
            >
              <span class="font-type text-[9px] font-bold text-primary">{{ rel.label }}</span>
              <span class="font-type text-[9px] text-primary">{{
                String(rel.id).padStart(3, '0')
              }}</span>
              <span class="truncate text-label-secondary">{{ rel.title }}</span>
            </a>
            <p v-if="!relations.length" class="mt-2 text-[11px] text-label-tertiary">
              这篇还没有交叉引用。
            </p>
          </div>
          <div v-if="article" class="mt-6 border-t border-rule pt-3">
            <p class="font-type text-[9.5px] uppercase tracking-[0.2em] text-label-tertiary">
              这张卡
            </p>
            <dl class="mt-2 space-y-1 text-[11px]">
              <div class="flex justify-between">
                <dt class="text-label-secondary">分块</dt>
                <dd class="font-type">{{ article.chunks_total }}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-label-secondary">向量</dt>
                <dd>{{ article.vector_state === 'ready' ? '已向量化' : '待向量化' }}</dd>
              </div>
              <div class="flex justify-between">
                <dt class="text-label-secondary">来源</dt>
                <dd>{{ article.source_type }}</dd>
              </div>
            </dl>
          </div>
        </aside>
      </div>
    </div>
  </div>
</template>
