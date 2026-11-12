<script setup lang="ts">
/**
 * 垂直切片承载页 —— prototype 技能规则：
 * 单一路由、`?variant=` 切换、浮动底栏、生产隐藏。
 * 三个原型（索引列表 / 长文阅读 / 画布）各自三个结构上不同的变体。
 */
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SliceRail from '../components/prototype/SliceRail.vue'
import SliceSwitcher from '../components/prototype/SliceSwitcher.vue'
import ListA from '../prototype/ListA.vue'
import ListB from '../prototype/ListB.vue'
import ListC from '../prototype/ListC.vue'
import ArticleA from '../prototype/ArticleA.vue'
import ArticleB from '../prototype/ArticleB.vue'
import ArticleC from '../prototype/ArticleC.vue'
import CanvasA from '../prototype/CanvasA.vue'
import CanvasB from '../prototype/CanvasB.vue'
import CanvasC from '../prototype/CanvasC.vue'
import { useSliceData } from '../composables/useSliceData'
import type { KnowledgeItem } from '../services/apis/knowledge'

const route = useRoute()
const router = useRouter()

const views = [
  { key: 'list', label: '① 索引列表' },
  { key: 'article', label: '② 长文阅读' },
  { key: 'canvas', label: '③ 画布' },
]

const variants = {
  list: [
    { key: 'a', label: 'A 硬卡 + 纸条', note: '招牌动作正面呈现：卡内左正文、右交叉引用纸条' },
    { key: 'b', label: 'B 索引表 + 详情栏', note: '一屏看更多；交叉引用挪到右侧常驻栏' },
    { key: 'c', label: 'C 双栏密排卡', note: '两列网格，密度最高，纸条压成一行' },
  ],
  article: [
    { key: 'a', label: 'A 居中书页 + 页边注', note: 'measure 640 居中，右侧页边注放引用' },
    { key: 'b', label: 'B 全宽 + 左侧目录', note: '正文左对齐栅格，目录常驻左侧' },
    { key: 'c', label: 'C 双页对照', note: '左右摊开：原文 / 骨架与引证' },
  ],
  canvas: [
    { key: 'a', label: 'A 暗色岛 + 关系清单', note: '浅外壳里的深色画布（#31 待裁方向）' },
    { key: 'b', label: 'B 浅色格纸图', note: '画布留在卡盒世界：可见格线 + 纸片节点' },
    { key: 'c', label: 'C 关系对位矩阵', note: '不画图，用行列对位读关系' },
  ],
} as const

type ViewKey = keyof typeof variants

const view = computed<ViewKey>(() => {
  const value = String(route.query.view ?? 'list')
  return value in variants ? (value as ViewKey) : 'list'
})

const variant = computed(() => {
  const value = String(route.query.variant ?? 'a')
  const keys = variants[view.value].map(v => v.key) as string[]
  return keys.includes(value) ? value : 'a'
})

const setView = (next: string) => {
  void router.replace({ query: { ...route.query, view: next, variant: 'a' } })
}
const setVariant = (next: string) => {
  void router.replace({ query: { ...route.query, variant: next } })
}

const {
  items,
  total,
  loading,
  article,
  articleLoading,
  graph,
  vectorReady,
  pending,
  failed,
  relationsByItem,
  shortDate,
  num,
  excerpt,
} = useSliceData()

const selectedId = ref<number | null>(null)
const selectItem = (item: KnowledgeItem) => {
  selectedId.value = item.id
}
const openItem = (item: KnowledgeItem) => {
  selectedId.value = item.id
  void router.replace({ query: { ...route.query, view: 'article' } })
}

const articleRelations = computed(() =>
  article.value ? (relationsByItem.value.get(article.value.id) ?? []) : [],
)

// 生产环境隐藏切换器（prototype 技能要求）
const isDev = !import.meta.env.PROD
const onKey = (event: KeyboardEvent) => {
  if (!isDev) return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  if (!event.altKey) return
  const order = views.map(v => v.key) as string[]
  const index = order.indexOf(view.value)
  const next = (index + (event.key === 'ArrowRight' ? 1 : -1) + order.length) % order.length
  if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
    event.preventDefault()
    setView(order[next]!)
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-canvas">
    <SliceRail :counts="{ index: total, chunks: items.reduce((s, i) => s + i.chunks_total, 0) }" />

    <main class="flex min-h-0 min-w-0 flex-1 flex-col">
      <template v-if="view === 'list'">
        <ListA
          v-if="variant === 'a'"
          :items="items"
          :total="total"
          :loading="loading"
          :relations-by-item="relationsByItem"
          :vector-ready="vectorReady"
          :pending="pending"
          :failed="failed"
          :short-date="shortDate"
          :num="num"
          :excerpt="excerpt"
          :on-open="openItem"
        />
        <ListB
          v-else-if="variant === 'b'"
          :items="items"
          :total="total"
          :loading="loading"
          :selected-id="selectedId"
          :relations-by-item="relationsByItem"
          :vector-ready="vectorReady"
          :pending="pending"
          :short-date="shortDate"
          :num="num"
          :excerpt="excerpt"
          :on-open="openItem"
          :on-select="selectItem"
        />
        <ListC
          v-else
          :items="items"
          :total="total"
          :relations-by-item="relationsByItem"
          :vector-ready="vectorReady"
          :pending="pending"
          :num="num"
          :excerpt="excerpt"
          :on-open="openItem"
        />
      </template>

      <template v-else-if="view === 'article'">
        <ArticleA
          v-if="variant === 'a'"
          :article="article"
          :loading="articleLoading"
          :relations="articleRelations"
          :num="num"
          :on-back="() => setView('list')"
        />
        <ArticleB
          v-else-if="variant === 'b'"
          :article="article"
          :loading="articleLoading"
          :relations="articleRelations"
          :num="num"
          :on-back="() => setView('list')"
        />
        <ArticleC
          v-else
          :article="article"
          :loading="articleLoading"
          :relations="articleRelations"
          :num="num"
          :on-back="() => setView('list')"
        />
      </template>

      <template v-else>
        <CanvasA v-if="variant === 'a'" :nodes="graph.nodes" :edges="graph.edges" :num="num" />
        <CanvasB v-else-if="variant === 'b'" :nodes="graph.nodes" :edges="graph.edges" :num="num" />
        <CanvasC v-else :nodes="graph.nodes" :edges="graph.edges" :num="num" />
      </template>
    </main>

    <SliceSwitcher
      v-if="isDev"
      :views="views"
      :view="view"
      :variants="[...variants[view]]"
      :variant="variant"
      @update:view="setView"
      @update:variant="setVariant"
    />
  </div>
</template>
