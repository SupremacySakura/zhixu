<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import AppIcon from '../components/ui/AppIcon.vue'
import { Motion } from 'motion-v'
import KnowledgeGraphChart from '../components/knowledge/KnowledgeGraphChart.vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import AppFilterBar from '../components/ui/AppFilterBar.vue'
import type { RelationStatus } from '../services/apis/knowledge'
import { KNOWLEDGE_FORMATS, knowledgeSourceLabel } from '../utils/knowledge'
import { RELATION_TYPE_OPTIONS, relationTypeLabel } from '../utils/relation'
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
  relationType,
  visibleEdges,
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

/** FilterMenu 只处理字符串键：标签用 "0" 表示全部。 */
const tagOptions = computed(() => [
  { key: '0', label: '全部标签' },
  ...tags.value.map(tag => ({ key: String(tag.id), label: tag.name })),
])
const tagFilter = computed({
  get: () => String(tagId.value),
  set: value => {
    tagId.value = Number(value) || 0
  },
})

/**
 * 关系类型筛选项：全部 + 六种类型，文案取自 utils/relation.ts，保证筛选与图例同源。
 * 只做客户端裁剪——后端按类型过滤会连节点一起裁剪，与「图例说明线型」的语义不符。
 */
const relationTypeOptions = [
  { key: '', label: '全部关系' },
  ...RELATION_TYPE_OPTIONS.map(option => ({ key: option.value, label: option.label })),
]
/** FilterMenu 只收发字符串，这里做一次白名单转换，避免把任意字符串写进 RelationType。 */
const relationFilter = computed({
  get: () => relationType.value,
  set: value => {
    relationType.value = RELATION_TYPE_OPTIONS.find(option => option.value === value)?.value ?? ''
  },
})
/** 空态文案要能说出当前选的关系类型名。 */
const relationFilterLabel = computed(() =>
  relationType.value === '' ? '全部关系' : relationTypeLabel(relationType.value),
)

/** 图例只在图里真的出现矛盾边时补一项，避免给不存在的样式留说明。 */
const hasContradiction = computed(() =>
  visibleEdges.value.some(edge => edge.relation_type === 'contradicts'),
)

const STATUS_LABELS: Record<RelationStatus, string> = {
  suggested: '建议',
  confirmed: '已确认',
  rejected: '已拒绝',
}
</script>

<template>
  <div class="relative flex min-h-0 flex-1 flex-col">
    <!-- 画布页的页头带是**悬浮**的（DESIGN 原型 ③）：它盖在满铺画布上，
         仍然是 64px，仍然不吸顶。所以内容区要自己让出 pt-band 的高度。 -->
    <header class="shell-band-float">
      <div class="min-w-0 flex-1">
        <h1 class="text-title">知识图谱</h1>
        <p class="font-type text-micro text-label-tertiary">
          节点 {{ graph.nodes.length }} · 边 {{ visibleEdges.length }}
        </p>
      </div>
    </header>

    <main class="shell-full flex-1 overflow-y-auto pt-band">
      <div class="shell-full">
        <p class="px-4 pt-4 text-subhead text-label-secondary sm:px-5 lg:px-6">
          以一条知识为中心查看它的一跳关系，也可以从最近更新的知识开始探索。
        </p>

        <!-- 筛选行不用 shell-toolbar：那个载体带 sticky，会跟悬浮页头带抢同一条顶边
             （两者都是 z-toolbar）。画布页的筛选行随画布一起滚走。 -->
        <section
          class="mt-4 flex min-h-12 flex-wrap items-center gap-2 border-b border-hairline bg-surface px-4 py-2 sm:px-5 lg:px-6"
        >
          <div class="relative w-full max-w-sm">
            <input
              v-model="searchTerm"
              class="knowledge-input w-full pl-9"
              placeholder="搜索知识，设为图谱中心"
              type="search"
              @input="scheduleSearch"
              @focus="searchOpen = true"
            />
            <AppIcon
              class="pointer-events-none absolute left-3 top-3.5 text-label-secondary"
              icon="material-symbols:search"
            />
            <div
              v-if="searchOpen && (searchResults.length || searching || searchHint)"
              class="absolute z-20 mt-2 w-full overflow-hidden border border-hairline bg-surface shadow-popover"
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
          <!-- 角标口径与 filtersActive 一致：它已经算清了「有几个筛选在生效」，
             这里不再重数一遍（两处口径一旦分叉，清空按钮与角标会互相打脸）。 -->
          <AppFilterBar :active-count="filtersActive ? 1 : 0">
            <button type="button" class="knowledge-button-secondary" @click="backToRecent">
              回到最近
            </button>
            <FilterMenu
              v-model="tagFilter"
              :options="tagOptions"
              label="筛选标签"
              data-test="tag-filter"
            />
            <FilterMenu
              v-model="sourceType"
              :options="sourceOptions"
              label="筛选来源类型"
              data-test="source-filter"
            />
            <FilterMenu
              v-model="relationFilter"
              :options="relationTypeOptions"
              label="筛选关系类型"
              data-test="relation-filter"
            />
            <button
              type="button"
              role="switch"
              :aria-checked="showSuggested"
              data-test="suggested-toggle"
              class="flex min-h-11 items-center gap-2.5 px-[13px] text-body font-semibold transition-colors"
              :class="
                showSuggested
                  ? 'bg-primary-tint text-primary-pressed'
                  : 'bg-surface-tertiary text-label-secondary hover:text-label'
              "
              @click="showSuggested = !showSuggested"
            >
              显示建议边
              <span
                aria-hidden="true"
                class="relative h-5 w-9 shrink-0 transition-colors"
                :class="showSuggested ? 'bg-primary' : 'bg-label-tertiary'"
              >
                <span
                  class="absolute left-0.5 top-0.5 size-4 bg-surface transition-transform"
                  :class="showSuggested ? 'translate-x-[16px]' : 'translate-x-0'"
                />
              </span>
            </button>
            <button
              v-if="filtersActive"
              type="button"
              class="knowledge-text-action"
              data-test="clear-filters"
              @click="clearFilters"
            >
              清空筛选
            </button>
          </AppFilterBar>
        </section>

        <p
          v-if="graph.truncated"
          class="mb-4 bg-warning-tint p-4 text-subhead text-warning-strong"
          role="status"
        >
          节点较多，已截断为连接度最高的 100 个节点。可以缩小范围，以某条知识为中心查看局部图谱。
        </p>
        <p
          v-if="error"
          class="mb-4 bg-warning-tint p-4 text-subhead text-warning-strong"
          role="alert"
        >
          {{ error }}
        </p>
        <p
          v-if="loading && graph.nodes.length === 0"
          class="border border-hairline bg-surface p-6 text-subhead text-label-secondary"
        >
          正在装载图谱…
        </p>
        <p
          v-else-if="loading"
          class="mb-4 border border-hairline bg-surface p-4 text-subhead text-label-secondary"
          role="status"
        >
          正在更新图谱…
        </p>

        <div
          v-if="!loading && !error && graph.nodes.length === 0"
          class="border border-hairline bg-surface px-6 py-16 text-center"
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
            class="mb-4 border border-hairline bg-surface p-4 text-subhead text-label-secondary"
          >
            {{
              showSuggested
                ? '这些知识之间还没有已确认或建议关系。打开任意知识详情页，在「相关知识」面板中建立或确认关系后再回来查看。'
                : '这些知识之间还没有已确认关系。打开任意知识详情页，在「相关知识」面板中建立或确认关系后再回来查看。'
            }}
          </p>
          <p
            v-if="graph.edges.length > 0 && visibleEdges.length === 0"
            class="mb-4 border border-hairline bg-surface p-4 text-subhead text-label-secondary"
            data-test="relation-filter-empty"
          >
            当前图谱里没有「{{ relationFilterLabel }}」的边，换个关系类型或清空筛选看看。
          </p>
          <section class="border border-hairline bg-surface p-2 shadow-card">
            <KnowledgeGraphChart
              :nodes="graph.nodes"
              :edges="visibleEdges"
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
                <span class="size-2 bg-info" />
                AI 关联
              </span>
              <span class="flex items-center gap-2">
                <span class="size-2 bg-label-tertiary" />
                手动关联
              </span>
              <span
                v-if="hasContradiction"
                class="flex items-center gap-2"
                data-test="legend-contradicts"
              >
                <span class="h-0 w-6 border-t-2 border-solid border-danger" />
                矛盾关系
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
                class="border border-hairline p-3"
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
            <RouterLink
              :to="`/home/knowledge/${selectedNode.id}`"
              class="knowledge-button-secondary"
            >
              查看详情
            </RouterLink>
          </div>
        </Motion>
      </div>
    </main>
  </div>
</template>
