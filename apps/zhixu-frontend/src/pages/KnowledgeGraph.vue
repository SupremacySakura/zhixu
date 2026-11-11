<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Motion } from 'motion-v'
import KnowledgeGraphChart from '../components/knowledge/KnowledgeGraphChart.vue'
import type { RelationStatus } from '../services/apis/knowledge'
import { KNOWLEDGE_FORMATS, knowledgeSourceLabel } from '../utils/knowledge'
import { relationTypeLabel } from '../utils/relation'
import { springSnappy } from '../utils/motion'
import { useKnowledgeGraph } from '../composables/useKnowledgeGraph'

const {
  graph,
  loading,
  error,
  selectedId,
  searchTerm,
  searchResults,
  searchOpen,
  searching,
  searchHint,
  tags,
  tagId,
  sourceType,
  showSuggested,
  nodeFiltersActive,
  filtersActive,
  selectedNode,
  oneHopRelations,
  scheduleSearch,
  clearFilters,
  backToRecent,
  chooseCenter,
  recenter,
} = useKnowledgeGraph()

/** 来源选项与知识列表共用能力表，文件导入格式（markdown/pdf 等）同样可筛选。 */
const sourceOptions = [
  { key: '', label: '全部来源' },
  { key: 'manual', label: '手动创建' },
  { key: 'web', label: '网页导入' },
  ...KNOWLEDGE_FORMATS.map(format => ({ key: format.type, label: format.label })),
]

const STATUS_LABELS: Record<RelationStatus, string> = {
  suggested: '建议',
  confirmed: '已确认',
  rejected: '已拒绝',
}
</script>

<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <p class="mb-4 text-subhead text-label-secondary">知识库 / 知识图谱</p>
      <header class="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-title-1">知识图谱</h1>
          <p class="mt-2 text-subhead text-label-secondary">
            以一条知识为中心查看它的一跳关系，也可以从最近更新的知识开始探索。
          </p>
        </div>
      </header>

      <section class="mb-4 flex flex-wrap items-center gap-3">
        <div class="relative w-full max-w-sm">
          <input
            v-model="searchTerm"
            class="knowledge-input w-full"
            placeholder="搜索知识，设为图谱中心"
            type="search"
            @input="scheduleSearch"
            @focus="searchOpen = true"
          />
          <div
            v-if="searchOpen && (searchResults.length || searching || searchHint)"
            class="absolute z-20 mt-2 w-full overflow-hidden rounded-lg border border-hairline bg-surface shadow-popover"
          >
            <p v-if="searching" class="px-3 py-2 text-subhead text-label-secondary">正在搜索…</p>
            <p v-else-if="searchHint" class="px-3 py-2 text-subhead text-label-secondary">
              {{ searchHint }}
            </p>
            <button
              v-for="item in searchResults"
              :key="item.id"
              type="button"
              class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-surface-secondary"
              @click="chooseCenter(item)"
            >
              <span class="truncate text-subhead text-label">{{ item.title }}</span>
              <span class="shrink-0 text-caption text-label-secondary">
                {{ knowledgeSourceLabel(item.source_type) }}
              </span>
            </button>
          </div>
        </div>
        <button type="button" class="knowledge-button-secondary" @click="backToRecent">
          回到最近
        </button>
        <label class="flex items-center gap-2 text-caption text-label-secondary">
          <span>标签</span>
          <select
            v-model.number="tagId"
            data-test="tag-filter"
            class="knowledge-input w-auto py-1 text-caption"
          >
            <option :value="0">全部标签</option>
            <option v-for="tag in tags" :key="tag.id" :value="tag.id">{{ tag.name }}</option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-caption text-label-secondary">
          <span>来源</span>
          <select
            v-model="sourceType"
            data-test="source-filter"
            class="knowledge-input w-auto py-1 text-caption"
          >
            <option v-for="option in sourceOptions" :key="option.key" :value="option.key">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-caption text-label-secondary">
          <input
            v-model="showSuggested"
            data-test="suggested-toggle"
            type="checkbox"
            class="size-4 accent-primary"
          />
          显示建议边
        </label>
        <button
          v-if="filtersActive"
          type="button"
          class="knowledge-text-action"
          data-test="clear-filters"
          @click="clearFilters"
        >
          清空筛选
        </button>
      </section>

      <p
        v-if="graph.truncated"
        class="mb-4 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
        role="status"
      >
        节点较多，已截断为连接度最高的 100 个节点。可以缩小范围，以某条知识为中心查看局部图谱。
      </p>
      <p
        v-if="error"
        class="mb-4 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
        role="alert"
      >
        {{ error }}
      </p>
      <p
        v-if="loading && graph.nodes.length === 0"
        class="rounded-lg border border-hairline bg-surface p-6 text-subhead text-label-secondary"
      >
        正在装载图谱…
      </p>
      <p
        v-else-if="loading"
        class="mb-4 rounded-lg border border-hairline bg-surface p-4 text-subhead text-label-secondary"
        role="status"
      >
        正在更新图谱…
      </p>

      <div
        v-if="!loading && !error && graph.nodes.length === 0"
        class="rounded-lg border border-hairline bg-surface px-6 py-16 text-center"
      >
        <template v-if="nodeFiltersActive">
          <h2 class="text-title-3">没有符合筛选条件的知识</h2>
          <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
            试试放宽标签或来源类型，或者清空筛选查看全部知识。
          </p>
          <button
            type="button"
            class="knowledge-button-secondary mt-5"
            data-test="clear-filters-empty"
            @click="clearFilters"
          >
            清空筛选
          </button>
        </template>
        <template v-else>
          <h2 class="text-title-3">还没有可展示的知识</h2>
          <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
            先导入或创建知识，再回到这里查看它们之间的关系。
          </p>
          <RouterLink to="/home/knowledge" class="knowledge-button-primary mt-5">
            去知识库
          </RouterLink>
        </template>
      </div>

      <template v-if="!error && graph.nodes.length">
        <p
          v-if="graph.edges.length === 0"
          class="mb-4 rounded-lg border border-hairline bg-surface p-4 text-subhead text-label-secondary"
        >
          {{
            showSuggested
              ? '这些知识之间还没有已确认或建议关系。打开任意知识详情页，在「相关知识」面板中建立或确认关系后再回来查看。'
              : '这些知识之间还没有已确认关系。打开任意知识详情页，在「相关知识」面板中建立或确认关系后再回来查看。'
          }}
        </p>
        <section class="rounded-lg border border-hairline bg-surface p-2 shadow-card">
          <KnowledgeGraphChart
            :nodes="graph.nodes"
            :edges="graph.edges"
            :selected-id="selectedId"
            @node-click="selectedId = $event"
          />
          <div
            class="mt-2 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-hairline px-2 pb-1 pt-3 text-caption text-label-secondary"
            data-test="graph-legend"
            aria-label="图例"
          >
            <span class="flex items-center gap-2">
              <span class="h-0 w-6 border-t-2 border-solid border-label-secondary" />
              已确认关系
            </span>
            <span class="flex items-center gap-2">
              <span class="h-0 w-6 border-t-2 border-dashed border-label-secondary" />
              建议关系
            </span>
            <span class="flex items-center gap-2">
              <span class="size-2 rounded-full bg-info" />
              AI 关联
            </span>
            <span class="flex items-center gap-2">
              <span class="size-2 rounded-full bg-label-tertiary" />
              手动关联
            </span>
          </div>
        </section>
      </template>
    </div>

    <div
      v-if="selectedNode"
      class="fixed inset-0 z-40"
      role="dialog"
      aria-modal="true"
      aria-label="知识节点详情"
    >
      <div class="absolute inset-0 bg-scrim" @click="selectedId = null" />
      <Motion
        :initial="{ x: 32, opacity: 0 }"
        :animate="{ x: 0, opacity: 1 }"
        :transition="springSnappy"
        class="absolute inset-y-0 right-0 flex w-full max-w-sm flex-col overflow-y-auto border-l border-hairline bg-surface p-6 shadow-floating"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="text-caption text-label-secondary">图谱节点</p>
            <h2 class="mt-1 text-title-3">{{ selectedNode.title }}</h2>
          </div>
          <button
            type="button"
            class="knowledge-icon-button shrink-0"
            aria-label="关闭"
            @click="selectedId = null"
          >
            ×
          </button>
        </div>

        <dl class="mt-5 grid grid-cols-2 gap-3">
          <div>
            <dt class="text-caption text-label-secondary">来源类型</dt>
            <dd class="mt-1 text-subhead text-label">
              {{ knowledgeSourceLabel(selectedNode.source_type) }}
            </dd>
          </div>
          <div>
            <dt class="text-caption text-label-secondary">连接度</dt>
            <dd class="mt-1 text-subhead text-label">度数 {{ selectedNode.degree }}</dd>
          </div>
        </dl>

        <p v-if="selectedNode.summary" class="mt-4 text-subhead leading-7 text-label-secondary">
          {{ selectedNode.summary }}
        </p>

        <section class="mt-6">
          <h3 class="text-headline">一跳关系</h3>
          <p v-if="oneHopRelations.length === 0" class="mt-2 text-subhead text-label-secondary">
            还没有已确认的一跳关系。
          </p>
          <ul v-else class="mt-3 space-y-2">
            <li
              v-for="relation in oneHopRelations"
              :key="relation.edge.id"
              class="rounded-md border border-hairline p-3"
            >
              <div
                class="flex items-center justify-between gap-2 text-caption text-label-secondary"
              >
                <span>{{ relationTypeLabel(relation.edge.relation_type) }}</span>
                <span>
                  {{ STATUS_LABELS[relation.edge.status] }} ·
                  {{ relation.edge.source === 'manual' ? '手动' : 'AI' }}
                </span>
              </div>
              <RouterLink
                :to="`/home/knowledge/${relation.counterpartId}`"
                class="mt-1 block truncate text-subhead font-medium text-primary hover:underline"
              >
                {{ relation.counterpartTitle }}
              </RouterLink>
            </li>
          </ul>
        </section>

        <div class="mt-6 flex flex-wrap gap-3">
          <button type="button" class="knowledge-button-primary" @click="recenter">
            以此为中心
          </button>
          <RouterLink :to="`/home/knowledge/${selectedNode.id}`" class="knowledge-button-secondary">
            查看详情
          </RouterLink>
        </div>
      </Motion>
    </div>
  </main>
</template>
