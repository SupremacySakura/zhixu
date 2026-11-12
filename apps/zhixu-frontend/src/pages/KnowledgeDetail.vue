<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Icon } from '@iconify/vue'
import {
  archiveKnowledge,
  favoriteKnowledge,
  getKnowledge,
  regenerateKnowledgeEvolve,
  unarchiveKnowledge,
  type KnowledgeItem,
} from '../services/apis/knowledge'
import {
  isKnowledgeActive,
  isKnowledgeVectorActive,
  knowledgeDate,
  knowledgeError,
  knowledgeSourceLabel,
} from '../utils/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'
import KnowledgeBadge from '../components/knowledge/KnowledgeBadge.vue'
import KnowledgeHistoryDialog from '../components/knowledge/KnowledgeHistoryDialog.vue'
import KnowledgeOrganizePanel from '../components/knowledge/KnowledgeOrganizePanel.vue'
import KnowledgeRelationsPanel from '../components/knowledge/KnowledgeRelationsPanel.vue'
import KnowledgeTagEditor from '../components/knowledge/KnowledgeTagEditor.vue'
import KnowledgeVectorPanel from '../components/knowledge/KnowledgeVectorPanel.vue'
const route = useRoute()
const item = ref<KnowledgeItem | null>(null)
const error = ref('')
const loading = ref(false)
const showHistory = ref(false)
const actionBusy = ref('')
// 演化检查是异步任务：提交后只给一句提示，结果由列表页/下次刷新体现。
const evolveBusy = ref(false)
const evolveHint = ref('')
// 采纳 AI 整理结果会追加标签，用 key 重挂标签面板让它重新拉取。
const tagVersion = ref(0)
let controller: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const html = computed(() =>
  renderKnowledgeContent(item.value?.content || '', item.value?.source_type || ''),
)
// 空字符串（历史/异常数据）也要落到 Markdown 标签，否则来源徽章渲染为空。
const sourceLabel = computed(() => knowledgeSourceLabel(item.value?.source_type || 'markdown'))
async function load(background = false) {
  controller?.abort()
  clearTimeout(timer)
  const current = new AbortController()
  controller = current
  if (!background) {
    item.value = null
    loading.value = true
  }
  error.value = ''
  try {
    const data = await getKnowledge(Number(route.params.id), current.signal)
    if (current.signal.aborted) return
    item.value = data
    if (isKnowledgeActive(data.status) || isKnowledgeVectorActive(data.vector_state))
      timer = setTimeout(() => load(true), 3000)
  } catch (e) {
    if (!current.signal.aborted) error.value = knowledgeError(e)
  } finally {
    if (controller === current) loading.value = false
  }
}
watch(
  () => route.params.id,
  () => load(),
  { immediate: true },
)
onScopeDispose(() => {
  controller?.abort()
  clearTimeout(timer)
})
async function toggleFavorite() {
  if (!item.value || actionBusy.value) return
  actionBusy.value = 'favorite'
  try {
    const state = await favoriteKnowledge(item.value.id, !item.value.is_favorite)
    item.value.is_favorite = state.is_favorite
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    actionBusy.value = ''
  }
}
async function toggleArchive() {
  if (!item.value || actionBusy.value) return
  actionBusy.value = 'archive'
  try {
    const state = item.value.archived_at
      ? await unarchiveKnowledge(item.value.id)
      : await archiveKnowledge(item.value.id)
    item.value.archived_at = state.archived_at
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    actionBusy.value = ''
  }
}
async function recheckEvolution() {
  if (!item.value || evolveBusy.value) return
  evolveBusy.value = true
  evolveHint.value = ''
  try {
    const task = await regenerateKnowledgeEvolve(item.value.id)
    evolveHint.value =
      task.status === 'failed'
        ? task.error_message || '演化检查失败，请稍后重试'
        : '已提交演化检查，结果稍后可在演化中心查看。'
  } catch (e) {
    evolveHint.value = knowledgeError(e)
  } finally {
    evolveBusy.value = false
  }
}
function onRestored(restored: KnowledgeItem) {
  item.value = restored
  showHistory.value = false
  load(true)
}
function onOrganized(updated: KnowledgeItem) {
  item.value = updated
  tagVersion.value += 1
}
</script>
<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <div class="mb-5 flex gap-3 text-subhead text-label-secondary">
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink
        ><span>/</span><span>知识详情</span>
      </div>
      <p v-if="loading" class="py-12 text-center" role="status">正在读取知识正文…</p>
      <div
        v-if="error"
        class="mb-5 rounded-lg bg-warning-tint p-4 text-warning-strong"
        role="alert"
      >
        {{ error }} <button class="knowledge-text-action" @click="load()">重新加载</button>
      </div>
      <template v-if="item">
        <header class="mb-7 flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0">
            <h1 class="break-all text-title-1">{{ item.title }}</h1>
            <p class="mt-2 text-subhead text-label-secondary">
              {{
                item.source_url
                  ? '来源于网页'
                  : item.source_file_id
                    ? `来源于${sourceLabel}文件`
                    : '手动创建'
              }}
              · 原始正文<template v-if="item.archived_at"> · 已归档</template>
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <button
              type="button"
              class="knowledge-button-secondary"
              :aria-pressed="item.is_favorite"
              @click="toggleFavorite"
            >
              <Icon
                :icon="
                  item.is_favorite
                    ? 'material-symbols:star-rounded'
                    : 'material-symbols:star-outline'
                "
                class="text-lg"
                aria-hidden="true"
              />{{ item.is_favorite ? '已收藏' : '收藏' }}
            </button>
            <button type="button" class="knowledge-button-secondary" @click="toggleArchive">
              <Icon
                :icon="
                  item.archived_at
                    ? 'material-symbols:unarchive-outline-rounded'
                    : 'material-symbols:archive-outline-rounded'
                "
                class="text-lg"
                aria-hidden="true"
              />{{ item.archived_at ? '恢复归档' : '归档' }}
            </button>
            <button
              v-if="item.status === 'ready'"
              type="button"
              class="knowledge-button-secondary"
              @click="showHistory = true"
            >
              版本历史
            </button>
            <RouterLink
              v-if="item.status === 'ready'"
              :to="`/home/knowledge/${item.id}/edit`"
              class="knowledge-button-secondary"
              >编辑</RouterLink
            >
            <RouterLink
              v-if="item.source_file_id"
              :to="{
                path: '/home/files',
                query: { folder: item.source_folder_id, folder_name: item.source_folder_name },
              }"
              class="knowledge-button-secondary"
              >查看来源目录 →</RouterLink
            >
            <button
              v-if="item.status === 'ready' && !item.archived_at"
              type="button"
              class="knowledge-button-secondary"
              :disabled="evolveBusy"
              @click="recheckEvolution"
            >
              <Icon
                :icon="
                  evolveBusy
                    ? 'material-symbols:progress-activity'
                    : 'material-symbols:refresh-rounded'
                "
                class="text-lg"
                :class="{ 'animate-spin': evolveBusy }"
                aria-hidden="true"
              />{{ evolveBusy ? '检查中…' : '重新检查' }}
            </button>
          </div>
        </header>
        <div
          v-if="item.archived_at"
          class="mb-5 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
          role="status"
        >
          该知识已归档，不会出现在默认列表与问答检索中。恢复后重新生效。
        </div>
        <div
          v-if="item.outdated_at"
          class="mb-5 rounded-lg bg-warning-tint p-4 text-subhead text-warning-strong"
          role="status"
        >
          <p class="flex items-center gap-2 font-semibold">
            <Icon icon="material-symbols:history-rounded" class="text-lg" aria-hidden="true" />
            这条知识可能已过时
          </p>
          <p class="mt-2 leading-6">
            {{ item.outdated_reason || '已被后续导入的一条知识取代，建议核对后更新。' }}
          </p>
          <p class="mt-2 text-caption">
            判定时间：{{
              knowledgeDate(item.outdated_at)
            }}。内容仍可正常阅读与检索，改写正文或标记已核实后该提示消失。
          </p>
        </div>
        <p v-if="evolveHint" class="mb-5 text-subhead text-label-secondary" role="status">
          {{ evolveHint }}
        </p>
        <div class="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_15rem]">
          <article
            class="min-w-0 rounded-lg border border-hairline bg-surface p-6 sm:p-9"
            aria-label="知识正文"
          >
            <div
              class="mb-7 flex flex-wrap items-center justify-between gap-3 border-b border-hairline pb-4"
            >
              <span class="text-caption text-label-secondary">正文 · {{ sourceLabel }}</span
              ><KnowledgeBadge :status="item.status" />
            </div>
            <!-- Parser disables HTML, unsafe protocols, and all image requests. -->
            <div v-if="item.status === 'ready'" class="knowledge-markdown" v-html="html" />
            <p v-else class="text-subhead leading-7 text-label-secondary" role="status">
              {{
                item.status === 'failed' ? item.error_message : '正文仍在处理，完成后会自动展示。'
              }}
            </p>
            <RouterLink
              v-if="item.status === 'failed'"
              to="/home/knowledge"
              class="knowledge-text-action mt-4"
              >返回知识库重试</RouterLink
            >
          </article>
          <div class="space-y-5">
            <KnowledgeRelationsPanel :item="item" />
            <KnowledgeOrganizePanel :item="item" @accepted="onOrganized" />
            <KnowledgeVectorPanel :item="item" @retried="item = $event" />
            <KnowledgeTagEditor :key="tagVersion" :knowledge-id="item.id" />
            <aside class="rounded-lg border border-hairline bg-surface p-5">
              <h2 class="mb-5 text-body font-semibold">来源信息</h2>
              <dl class="space-y-5">
                <div v-if="item.source_file_id">
                  <dt class="text-caption text-label-secondary">原文件</dt>
                  <dd class="mt-1.5 break-all text-subhead">{{ item.source_file_name }}</dd>
                </div>
                <div v-if="item.source_file_id">
                  <dt class="text-caption text-label-secondary">来源目录（加入时）</dt>
                  <dd class="mt-1.5 text-subhead">{{ item.source_folder_name }}</dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">当前版本</dt>
                  <dd class="mt-1.5 text-subhead">第 {{ item.version }} 版</dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">来源类型</dt>
                  <dd class="mt-1.5 text-subhead">
                    {{ sourceLabel }}{{ item.source_file_id ? '文件' : '' }}
                  </dd>
                </div>
                <div v-if="item.source_url">
                  <dt class="text-caption text-label-secondary">来源链接</dt>
                  <dd class="mt-1.5 break-all text-subhead">
                    <a
                      :href="item.source_url"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-primary-pressed"
                      >{{ item.source_url }}</a
                    >
                  </dd>
                </div>
                <div>
                  <dt class="text-caption text-label-secondary">加入时间</dt>
                  <dd class="mt-1.5 text-subhead">{{ knowledgeDate(item.imported_at) }}</dd>
                </div>
              </dl>
              <p
                class="mt-6 border-t border-hairline pt-4 text-caption leading-6 text-label-secondary"
              >
                来源信息为加入时的快照。原文件改名、移动或删除不影响已保存正文。
              </p>
            </aside>
          </div>
        </div>
        <KnowledgeHistoryDialog
          v-if="showHistory"
          :knowledge-id="item.id"
          @close="showHistory = false"
          @restored="onRestored"
        />
      </template>
    </div>
  </main>
</template>
