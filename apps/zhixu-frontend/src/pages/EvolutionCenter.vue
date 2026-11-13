<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Motion } from 'motion-v'
import FilterMenu from '../components/ui/FilterMenu.vue'
import AppFilterBar from '../components/ui/AppFilterBar.vue'
import FilePagination from '../components/file-management/FilePagination.vue'
import { EVOLUTION_PAGE_SIZE, useEvolutionAlerts } from '../composables/useEvolutionAlerts'
import { pendingTotal } from '../services/apis/evolution'
import type {
  EvolutionAction,
  EvolutionAlert,
  EvolutionAlertStatus,
  EvolutionAlertType,
} from '../services/apis/evolution'
import { evolutionActionLabel } from '../utils/operationLog'
import { highlightQuote, longestCommonPhrase } from '../utils/evolution'
import { knowledgeDate, knowledgeSourceLabel } from '../utils/knowledge'
import { springSnappy } from '../utils/motion'

const {
  tab,
  type,
  status,
  page,
  items,
  total,
  totalPages,
  loading,
  error,
  resolveError,
  resolving,
  summary,
  load,
  loadSummary,
  resolveAlert,
  changePage,
} = useEvolutionAlerts()

const TYPE_LABELS: Record<EvolutionAlertType, string> = {
  duplicate: '重复',
  outdated: '过时',
  conflict: '冲突',
}
const STATUS_LABELS: Record<EvolutionAlertStatus, string> = {
  pending: '待处理',
  deferred: '暂不处理',
  resolved: '已处理',
  dismissed: '已忽略',
}

const typeOptions = [
  { key: '', label: '全部类型' },
  { key: 'duplicate', label: '重复' },
  { key: 'outdated', label: '过时' },
  { key: 'conflict', label: '冲突' },
] as const

/** 状态筛选随页签变化：待办视野只有待处理与暂不处理，记录页签只有已处理与已忽略。 */
const statusOptions = computed(() =>
  tab.value === 'pending'
    ? [
        { key: '', label: '全部待办' },
        { key: 'pending', label: '待处理' },
        { key: 'deferred', label: '暂不处理' },
      ]
    : [
        { key: '', label: '全部记录' },
        { key: 'resolved', label: '已处理' },
        { key: 'dismissed', label: '已忽略' },
      ],
)

/** 每种提醒可用的裁决动作，顺序即面板里的按钮顺序，与后端 AllowedFor 一致。 */
const TYPE_ACTIONS: Record<EvolutionAlertType, EvolutionAction[]> = {
  duplicate: ['archive_one', 'keep_both', 'mark_verified', 'dismiss', 'defer'],
  outdated: ['mark_outdated', 'mark_verified', 'dismiss', 'defer'],
  conflict: [
    'accept_revision',
    'keep_both',
    'create_relation',
    'mark_verified',
    'dismiss',
    'defer',
  ],
}

const selectedId = ref<number | null>(null)
const note = ref('')

const selectedAlert = computed(() => items.value.find(item => item.id === selectedId.value) ?? null)
const filtered = computed(() => Boolean(type.value || status.value))
/** 页头展示待办视野总数（待处理 + 暂不处理）；首页卡片的「待处理」只算 pending。 */
const pendingCount = computed(() => pendingTotal(summary.value))

const archived = (alert: EvolutionAlert) => alert.subjects.some(subject => subject.archived)
const hasRevision = (alert: EvolutionAlert) => Boolean(alert.payload.revised_content?.trim())

const actionsOf = (alert: EvolutionAlert) => TYPE_ACTIONS[alert.alert_type]

/** 置灰原因：归档知识只能忽略或标记已核实，无修订稿不能采纳修订。 */
const disabledReason = (alert: EvolutionAlert, action: EvolutionAction): string => {
  // 归档知识只允许「忽略」与「标记已核实」；暂不处理也会让提醒继续挂着，一并禁用。
  if (archived(alert) && action !== 'dismiss' && action !== 'mark_verified')
    return '涉及知识已归档，只能忽略或标记已核实'
  if (action === 'accept_revision' && !hasRevision(alert)) return '该提醒没有修订稿，暂不能采纳修订'
  return ''
}

const subjectAt = (alert: EvolutionAlert, index: number) => alert.subjects[index] ?? null
const quoteAt = (alert: EvolutionAlert, index: number) =>
  alert.payload.evidence.find(item => item.knowledge_id === subjectAt(alert, index)?.id)?.quote ??
  alert.payload.evidence[index]?.quote ??
  ''

const sharedPhrase = (alert: EvolutionAlert) =>
  longestCommonPhrase(quoteAt(alert, 0), quoteAt(alert, 1))

const openDetail = (alert: EvolutionAlert) => {
  selectedId.value = alert.id
  note.value = ''
}

/** 清空筛选回到全部提醒；两个筛选都置空后 watch 会自动重拉。 */
const clearFilters = () => {
  type.value = ''
  status.value = ''
}

const closeDetail = () => {
  selectedId.value = null
  note.value = ''
}

const submit = async (alert: EvolutionAlert, action: EvolutionAction) => {
  const ok = await resolveAlert(alert, action, note.value)
  if (ok) closeDetail()
}

watch(selectedAlert, alert => {
  if (!alert) note.value = ''
})

onMounted(() => {
  void load()
  void loadSummary()
})
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <!-- 页头带 64px：左标题 + 等宽计数行（待处理数 = pending + deferred，
         与首页提醒卡片只算 pending 是两套口径，这里沿用页头的待办视野）。
         面包屑已取消 —— 索引页不再重复侧栏已经说明的位置。 -->
    <header class="shell-band">
      <div class="min-w-0 flex-1">
        <h1 class="text-title">演化中心</h1>
        <p class="font-type text-micro text-label-tertiary">
          待处理 <span class="font-semibold text-label">{{ pendingCount }}</span> 条
        </p>
      </div>
    </header>

    <main class="shell-page">
      <div class="shell-wide">
        <p class="mb-4 text-subhead text-label-secondary">
          逐条处理重复、过时与冲突提醒；每条裁决只影响提醒涉及的知识。
        </p>
        <div class="mb-4 flex gap-1" role="tablist" aria-label="演化提醒视图">
          <button
            type="button"
            role="tab"
            :aria-selected="tab === 'pending'"
            :class="[
              ' px-4 py-1.5 text-subhead transition-colors',
              tab === 'pending'
                ? 'bg-primary-tint font-semibold text-primary-pressed'
                : 'text-label-secondary hover:bg-surface-secondary',
            ]"
            @click="tab = 'pending'"
          >
            待处理
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="tab === 'records'"
            :class="[
              ' px-4 py-1.5 text-subhead transition-colors',
              tab === 'records'
                ? 'bg-primary-tint font-semibold text-primary-pressed'
                : 'text-label-secondary hover:bg-surface-secondary',
            ]"
            @click="tab = 'records'"
          >
            处理记录
          </button>
        </div>

        <section class="mb-4 flex flex-wrap items-center gap-3">
          <!-- 角标只数两个下拉：那才是「筛选条件」。共 N 条是结果计数，不是筛选。 -->
          <AppFilterBar :active-count="(type ? 1 : 0) + (status ? 1 : 0)">
            <FilterMenu
              v-model="type"
              :options="typeOptions"
              label="提醒类型"
              data-test="type-filter"
            />
            <FilterMenu v-model="status" :options="statusOptions" label="提醒状态" />
          </AppFilterBar>
          <span class="text-caption text-label-secondary">共 {{ total }} 条</span>
        </section>

        <p
          v-if="error"
          class="mb-4 bg-warning-tint p-4 text-subhead text-warning-strong"
          role="alert"
        >
          {{ error }}
        </p>

        <p v-if="loading" class="mb-4 text-subhead text-label-secondary" role="status">
          正在加载提醒…
        </p>

        <div
          v-else-if="!error && items.length === 0 && filtered"
          class="border border-hairline bg-surface px-6 py-16 text-center"
          data-test="empty-filtered"
        >
          <h2 class="text-title-3">没有符合筛选条件的提醒</h2>
          <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
            换一个类型或状态试试，也可以清空筛选看全部提醒。
          </p>
          <button type="button" class="knowledge-button-secondary mt-6" @click="clearFilters">
            清空筛选
          </button>
        </div>

        <div
          v-else-if="!error && items.length === 0"
          class="border border-hairline bg-surface px-6 py-16 text-center"
          data-test="empty-none"
        >
          <h2 class="text-title-3">
            {{ tab === 'pending' ? '当前没有需要处理的演化提醒' : '还没有处理记录' }}
          </h2>
          <p class="mx-auto mt-3 max-w-sm text-subhead leading-7 text-label-secondary">
            {{
              tab === 'pending'
                ? '导入或编辑知识后，系统会自动检查重复、过时与冲突，发现问题会出现在这里。'
                : '处理过的提醒会按时间倒序留在这里，方便回看当时的裁决与备注。'
            }}
          </p>
        </div>

        <template v-else-if="!error">
          <ul class="space-y-3">
            <li
              v-for="alert in items"
              :key="alert.id"
              class="cursor-pointer border border-hairline bg-surface p-5 transition-shadow hover:shadow-popover"
              data-test="alert-card"
              tabindex="0"
              @click="openDetail(alert)"
              @keydown.enter="openDetail(alert)"
            >
              <div class="flex flex-wrap items-center gap-2 text-caption">
                <span class="bg-primary-tint px-2.5 py-1 font-medium text-primary-pressed">
                  {{ TYPE_LABELS[alert.alert_type] }}
                </span>
                <span class="text-label-secondary">{{ STATUS_LABELS[alert.status] }}</span>
                <span v-if="alert.severity === 'strong'" class="text-warning-strong"
                  >需要优先处理</span
                >
                <span class="text-label-tertiary">{{ knowledgeDate(alert.created_at) }}</span>
              </div>
              <h3 class="mt-2 truncate text-headline">
                {{ alert.subjects[0]?.title || '演化提醒' }}
              </h3>
              <p class="mt-1 text-subhead leading-7 text-label-secondary">
                {{ alert.payload.reason }}
              </p>
              <p
                v-if="archived(alert)"
                class="mt-2 bg-warning-tint px-3 py-1.5 text-caption text-warning-strong"
              >
                涉及知识已归档
              </p>
              <div
                v-if="tab === 'records'"
                class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-caption text-label-secondary"
              >
                <span>{{ evolutionActionLabel(alert.resolved_action) }}</span>
                <span>{{ knowledgeDate(alert.resolved_at) }}</span>
                <span v-if="alert.resolution_note">备注：{{ alert.resolution_note }}</span>
              </div>
            </li>
          </ul>

          <FilePagination
            v-if="total > EVOLUTION_PAGE_SIZE"
            class="mt-4"
            :page="page"
            :total-pages="totalPages"
            :start-index="(page - 1) * EVOLUTION_PAGE_SIZE + 1"
            :end-index="Math.min(page * EVOLUTION_PAGE_SIZE, total)"
            :total-count="total"
            :loading="loading"
            @change="changePage"
          />
        </template>
      </div>

      <div
        v-if="selectedAlert"
        class="fixed inset-0 z-40"
        role="dialog"
        aria-modal="true"
        aria-label="演化提醒详情"
      >
        <div class="absolute inset-0 bg-scrim" @click="closeDetail" />
        <Motion
          :initial="{ x: 32, opacity: 0 }"
          :animate="{ x: 0, opacity: 1 }"
          :transition="springSnappy"
          class="absolute inset-y-0 right-0 flex w-full max-w-md flex-col overflow-y-auto border-l border-hairline bg-surface p-6 shadow-floating"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="text-caption text-label-secondary">
                {{ TYPE_LABELS[selectedAlert.alert_type] }}提醒 ·
                {{ STATUS_LABELS[selectedAlert.status] }}
              </p>
              <h2 class="mt-1 text-title-3">
                {{ selectedAlert.subjects[0]?.title || '演化提醒' }}
              </h2>
            </div>
            <button
              type="button"
              class="knowledge-icon-button shrink-0"
              aria-label="关闭"
              @click="closeDetail"
            >
              ×
            </button>
          </div>

          <p class="mt-4 text-subhead leading-7 text-label-secondary">
            {{ selectedAlert.payload.reason }}
          </p>

          <p
            v-if="archived(selectedAlert)"
            class="mt-3 bg-warning-tint px-3 py-2 text-caption text-warning-strong"
          >
            涉及知识已归档，只能忽略或标记已核实
          </p>
          <p
            v-if="selectedAlert.alert_type === 'conflict' && !hasRevision(selectedAlert)"
            class="mt-3 bg-surface-secondary px-3 py-2 text-caption text-label-secondary"
          >
            该提醒没有修订稿，暂不能采纳修订
          </p>

          <section class="mt-5">
            <h3 class="text-headline">证据对照</h3>
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div
                v-for="index in [0, 1]"
                :key="index"
                class="border border-hairline p-3"
                :data-test="`evidence-${index}`"
              >
                <p class="text-caption text-label-secondary">
                  {{ index === 0 ? '本次检查的知识' : '被比对的知识' }}
                </p>
                <RouterLink
                  v-if="subjectAt(selectedAlert, index)"
                  :to="`/home/knowledge/${subjectAt(selectedAlert, index)!.id}`"
                  class="mt-1 block truncate text-subhead font-medium text-primary hover:underline"
                >
                  {{ subjectAt(selectedAlert, index)!.title }}
                </RouterLink>
                <p class="mt-1 text-caption text-label-tertiary">
                  {{ knowledgeSourceLabel(subjectAt(selectedAlert, index)?.source_type || '') }}
                </p>
                <blockquote class="mt-2 text-subhead leading-7 text-label">
                  <template
                    v-for="(segment, segmentIndex) in highlightQuote(
                      quoteAt(selectedAlert, index),
                      sharedPhrase(selectedAlert),
                    )"
                    :key="segmentIndex"
                  >
                    <mark v-if="segment.hit" class="bg-warning-tint text-warning-strong">{{
                      segment.text
                    }}</mark>
                    <template v-else>{{ segment.text }}</template>
                  </template>
                </blockquote>
              </div>
            </div>
          </section>

          <section v-if="hasRevision(selectedAlert)" class="mt-5">
            <h3 class="text-headline">修订稿</h3>
            <p class="mt-2 text-subhead font-medium text-label">
              {{ selectedAlert.payload.revised_title }}
            </p>
            <p class="mt-1 text-subhead leading-7 text-label-secondary">
              {{ selectedAlert.payload.revised_content }}
            </p>
          </section>

          <section v-if="tab === 'pending'" class="mt-5">
            <label class="text-caption text-label-secondary" for="evolution-note"
              >备注（随裁决一并记录）</label
            >
            <textarea
              id="evolution-note"
              v-model="note"
              class="knowledge-textarea mt-2"
              rows="3"
              placeholder="可选，例如：两条内容重复但来源不同，先保留"
            />
          </section>

          <p
            v-if="resolveError"
            class="mt-3 bg-warning-tint p-3 text-subhead text-warning-strong"
            role="alert"
          >
            {{ resolveError }}
          </p>

          <div v-if="tab === 'pending'" class="mt-5 flex flex-wrap gap-3">
            <button
              v-for="action in actionsOf(selectedAlert)"
              :key="action"
              type="button"
              :data-action="action"
              :disabled="Boolean(disabledReason(selectedAlert, action)) || resolving"
              :title="disabledReason(selectedAlert, action) || undefined"
              :class="
                action === 'dismiss' ? 'knowledge-button-secondary' : 'knowledge-button-primary'
              "
              @click="submit(selectedAlert, action)"
            >
              {{ evolutionActionLabel(action) }}
            </button>
          </div>

          <div v-else class="mt-5 flex flex-wrap gap-3">
            <RouterLink
              v-if="subjectAt(selectedAlert, 0)"
              :to="`/home/knowledge/${subjectAt(selectedAlert, 0)!.id}`"
              class="knowledge-button-secondary"
            >
              查看知识
            </RouterLink>
          </div>
        </Motion>
      </div>
    </main>
  </div>
</template>
