<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppIcon from '../components/ui/AppIcon.vue'
import AppLoading from '../components/ui/AppLoading.vue'
import {
  archiveKnowledge,
  favoriteKnowledge,
  getKnowledge,
  listKnowledgeRelations,
  regenerateKnowledgeEvolve,
  unarchiveKnowledge,
  type KnowledgeItem,
  type KnowledgeRelation,
} from '../services/apis/knowledge'
import {
  isKnowledgeActive,
  isKnowledgeVectorActive,
  knowledgeDate,
  knowledgeError,
  knowledgeSourceLabel,
  knowledgeVectorLabels,
} from '../utils/knowledge'
import { renderKnowledgeContent } from '../utils/markdown'
import { relationTypeLabel } from '../utils/relation'
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
/**
 * 页边注里的交叉引用摘要。
 *
 * 为什么另拉一次 `listKnowledgeRelations` 而不复用下方面板的数据：
 * 面板是 587 行的管理界面（查找 / 添加 / 确认 / 拒绝 / 轮询），数据在它内部；
 * 把页边注塞进面板等于让「摘要」依赖「管理器」的生命周期，改一处会牵两处。
 * 摘要只要一份只读列表，重复一次 GET 比共享状态便宜。
 */
const noteRelations = ref<KnowledgeRelation[]>([])
/** 页边注只放得下几条；多出来的交给下方面板。 */
const NOTE_LIMIT = 4
const noteVisible = computed(() => noteRelations.value.slice(0, NOTE_LIMIT))
const noteRest = computed(() => Math.max(0, noteRelations.value.length - NOTE_LIMIT))
let controller: AbortController | null = null
let noteController: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined
/**
 * 去掉正文开头那个与页头标题重复的一级标题。
 *
 * 为什么在这里做而不是改 `renderKnowledgeContent`：那个渲染函数被三处共用
 * （本页 / 编辑页预览 / 版本历史），那两处要的是原文，剥掉会误导用户。
 *
 * 为什么按「渲染后的首个元素是 h1 且文字等于标题」判定，而不是按源码首行：
 * 1. 文本相等才叫「重复」，正文里合法的章节 h1 必须留着；
 * 2. `source_type === 'text'` 走的是 breaks 渲染，`# 标题` 不是 h1，本来就不重复，
 *    按源码删会把用户真正的第一行吃掉。
 */
const ENTITIES: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', '#39': "'" }
function headingText(html: string) {
  return html
    .replace(/<[^>]*>/g, '')
    .replace(/&(amp|lt|gt|quot|#39);/g, (_, name: string) => ENTITIES[name] ?? '')
    .replace(/\s+/g, ' ')
    .trim()
}
function stripRepeatedHeading(html: string, title: string) {
  const match = html.match(/^\s*<h1\b[^>]*>([\s\S]*?)<\/h1>\s*/)
  if (!match) return html
  const heading = headingText(match[1] ?? '')
  const pageTitle = title.replace(/\s+/g, ' ').trim()
  if (!pageTitle || heading !== pageTitle) return html
  return html.slice(match[0].length)
}
const html = computed(() =>
  stripRepeatedHeading(
    renderKnowledgeContent(item.value?.content || '', item.value?.source_type || ''),
    item.value?.title || '',
  ),
)
// 空字符串（历史/异常数据）也要落到 Markdown 标签，否则来源徽章渲染为空。
const sourceLabel = computed(() => knowledgeSourceLabel(item.value?.source_type || 'markdown'))
const vectorLabel = computed(() =>
  item.value?.vector_state ? knowledgeVectorLabels[item.value.vector_state] : '待向量化',
)
async function loadNote(signal: AbortSignal) {
  noteController?.abort()
  const current = new AbortController()
  noteController = current
  try {
    const data = await listKnowledgeRelations(Number(route.params.id), current.signal)
    if (current.signal.aborted || signal.aborted) return
    noteRelations.value = data.confirmed
  } catch {
    // 页边注是摘要：拿不到就当作没有交叉引用，不打断正文阅读，
    // 真正的错误提示由下方的关系面板负责（它会把失败原因显示出来）。
    if (!current.signal.aborted) noteRelations.value = []
  }
}
async function load(background = false) {
  controller?.abort()
  clearTimeout(timer)
  const current = new AbortController()
  controller = current
  if (!background) {
    item.value = null
    loading.value = true
    noteRelations.value = []
  }
  error.value = ''
  try {
    const data = await getKnowledge(Number(route.params.id), current.signal)
    if (current.signal.aborted) return
    item.value = data
    if (!background) void loadNote(current.signal)
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
  noteController?.abort()
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
  <main class="shell-page">
    <!-- 原型 ②「长文阅读」：容器 measure（40em）、**没有页头带**。
         标题直接落在正文流里，操作行是 32px 的次操作，不占用一条 64px 的带子。 -->
    <div class="shell-measure">
      <!-- 面包屑：三层以上页面保留（知识库 → 知识详情），共 3 处之一。 -->
      <div data-test="breadcrumb" class="mb-5 flex gap-3 text-body text-label-secondary">
        <RouterLink to="/home/knowledge" class="text-primary-pressed">知识库</RouterLink
        ><span>/</span><span>知识详情</span>
      </div>
      <p v-if="loading" class="py-12 text-center" role="status">正在读取知识正文…</p>
      <div v-if="error" class="mb-5 bg-warning-tint p-4 text-warning-strong" role="alert">
        {{ error }} <button class="atom-text-action" @click="load()">重新加载</button>
      </div>
      <template v-if="item">
        <header class="mb-6">
          <p class="font-type text-micro text-label-tertiary">
            第 {{ item.version }} 版 · {{ item.chunks_total }} 分块
          </p>
          <h1 class="mt-2 break-all text-display">{{ item.title }}</h1>
          <p class="mt-3 text-body text-label-secondary">
            {{
              item.source_url
                ? '来源于网页'
                : item.source_file_id
                  ? `来源于${sourceLabel}文件`
                  : '手动创建'
            }}
            · 原始正文<template v-if="item.archived_at"> · 已归档</template>
          </p>
          <!-- 无页头带 ⇒ 动作落在正文流里。用 32px 的次操作（shell-band-action）：
               36px 的 atom-btn 六个并排会顶满整行，读起来像工具栏。 -->
          <div class="mt-4 flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="shell-band-action"
              :aria-pressed="item.is_favorite"
              @click="toggleFavorite"
            >
              <AppIcon
                :icon="
                  item.is_favorite
                    ? 'material-symbols:star-rounded'
                    : 'material-symbols:star-outline'
                "
                class="atom-glyph"
                aria-hidden="true"
                :style="{ '--atom-glyph-size': '16px' }"
              />{{ item.is_favorite ? '已收藏' : '收藏' }}
            </button>
            <button type="button" class="shell-band-action" @click="toggleArchive">
              <AppIcon
                :icon="
                  item.archived_at
                    ? 'material-symbols:unarchive-outline-rounded'
                    : 'material-symbols:archive-outline-rounded'
                "
                class="atom-glyph"
                aria-hidden="true"
                :style="{ '--atom-glyph-size': '16px' }"
              />{{ item.archived_at ? '恢复归档' : '归档' }}
            </button>
            <button
              v-if="item.status === 'ready'"
              type="button"
              class="shell-band-action"
              @click="showHistory = true"
            >
              版本历史
            </button>
            <RouterLink
              v-if="item.status === 'ready'"
              :to="`/home/knowledge/${item.id}/edit`"
              class="shell-band-action-primary"
              >编辑</RouterLink
            >
            <RouterLink
              v-if="item.source_file_id"
              :to="{
                path: '/home/files',
                query: { folder: item.source_folder_id, folder_name: item.source_folder_name },
              }"
              class="shell-band-action"
              >查看来源目录 →</RouterLink
            >
            <button
              v-if="item.status === 'ready' && !item.archived_at"
              type="button"
              class="shell-band-action"
              :disabled="evolveBusy"
              @click="recheckEvolution"
            >
              <AppLoading v-if="evolveBusy" size="sm" />
              <AppIcon
                v-else
                icon="material-symbols:refresh-rounded"
                class="atom-glyph"
                aria-hidden="true"
                :style="{ '--atom-glyph-size': '16px' }"
              />{{ evolveBusy ? '检查中…' : '重新检查' }}
            </button>
          </div>
        </header>
        <div
          v-if="item.archived_at"
          class="mb-5 bg-warning-tint p-4 text-body text-warning-strong"
          role="status"
        >
          该知识已归档，不会出现在默认列表与问答检索中。恢复后重新生效。
        </div>
        <div
          v-if="item.outdated_at"
          class="mb-5 bg-warning-tint p-4 text-body text-warning-strong"
          role="status"
        >
          <p class="flex items-center gap-2 font-semibold">
            <AppIcon
              icon="material-symbols:history-rounded"
              class="atom-glyph"
              aria-hidden="true"
              :style="{ '--atom-glyph-size': '16px' }"
            />
            这条知识可能已过时
          </p>
          <p class="mt-2 leading-6">
            {{ item.outdated_reason || '已被后续导入的一条知识取代，建议核对后更新。' }}
          </p>
          <p class="mt-2 text-note">
            判定时间：{{
              knowledgeDate(item.outdated_at)
            }}。内容仍可正常阅读与检索，改写正文或标记已核实后该提示消失。
          </p>
        </div>
        <p v-if="evolveHint" class="mb-5 text-body text-label-secondary" role="status">
          {{ evolveHint }}
        </p>
        <!-- 长文不套行框：正文直接落在画布上，标题下一条规则线就是它的边界。
             页边注在 ≥1280 落到正文右侧的页边（那里才有多余的横向留白），
             更窄时按 DESIGN.md:514 的结构性降级落到正文下方 —— 不是缩小，是换位置。 -->
        <div class="relative xl:min-h-72">
          <article class="min-w-0" aria-label="知识正文">
            <div class="border-t border-rule pt-4">
              <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
                <span class="text-note text-label-secondary">正文 · {{ sourceLabel }}</span
                ><KnowledgeBadge :status="item.status" />
              </div>
              <!-- Parser disables HTML, unsafe protocols, and all image requests. -->
              <div v-if="item.status === 'ready'" class="knowledge-markdown" v-html="html" />
              <p v-else class="text-body leading-7 text-label-secondary" role="status">
                {{
                  item.status === 'failed' ? item.error_message : '正文仍在处理，完成后会自动展示。'
                }}
              </p>
              <RouterLink
                v-if="item.status === 'failed'"
                to="/home/knowledge"
                class="atom-text-action mt-4"
                >返回知识库重试</RouterLink
              >
            </div>
          </article>
          <aside
            aria-label="页边注"
            class="mt-8 border-t border-rule pt-3 xl:absolute xl:left-full xl:top-0 xl:mt-0 xl:ml-8 xl:w-36"
          >
            <h2 class="text-body font-semibold">交叉引用</h2>
            <ul v-if="noteVisible.length" class="mt-2 space-y-1.5">
              <li v-for="relation in noteVisible" :key="relation.id" class="text-note leading-5">
                <span class="font-semibold text-primary-pressed">{{
                  relationTypeLabel(relation.relation_type)
                }}</span>
                <RouterLink
                  :to="`/home/knowledge/${relation.counterpart_id ?? relation.target_id}`"
                  class="text-label-secondary underline decoration-hairline hover:text-label"
                  >{{ relation.counterpart_title || `#${relation.target_id}` }}</RouterLink
                >
              </li>
            </ul>
            <p v-else class="mt-2 text-note leading-5 text-label-secondary">这篇还没有交叉引用。</p>
            <p v-if="noteRest" class="mt-1.5 text-note leading-5 text-label-tertiary">
              另有 {{ noteRest }} 条，见下方交叉引用面板。
            </p>
            <h2 class="mt-5 text-body font-semibold">这张卡</h2>
            <dl class="mt-2 space-y-1 text-note">
              <div class="flex items-baseline justify-between gap-2">
                <dt class="text-label-secondary">分块</dt>
                <dd class="font-type">{{ item.chunks_total }}</dd>
              </div>
              <div class="flex items-baseline justify-between gap-2">
                <dt class="text-label-secondary">向量</dt>
                <dd>{{ vectorLabel }}</dd>
              </div>
              <div class="flex items-baseline justify-between gap-2">
                <dt class="text-label-secondary">来源</dt>
                <dd class="truncate">{{ sourceLabel }}</dd>
              </div>
            </dl>
          </aside>
        </div>
        <div class="mt-8 space-y-5">
          <KnowledgeRelationsPanel :item="item" />
          <KnowledgeOrganizePanel :item="item" @accepted="onOrganized" />
          <KnowledgeVectorPanel :item="item" @retried="item = $event" />
          <KnowledgeTagEditor :key="tagVersion" :knowledge-id="item.id" />
          <aside class="border border-hairline bg-surface p-5">
            <h2 class="mb-5 text-body font-semibold">来源信息</h2>
            <dl class="space-y-5">
              <div v-if="item.source_file_id">
                <dt class="text-note text-label-secondary">原文件</dt>
                <dd class="mt-1.5 break-all text-body">{{ item.source_file_name }}</dd>
              </div>
              <div v-if="item.source_file_id">
                <dt class="text-note text-label-secondary">来源目录（加入时）</dt>
                <dd class="mt-1.5 text-body">{{ item.source_folder_name }}</dd>
              </div>
              <div>
                <dt class="text-note text-label-secondary">当前版本</dt>
                <dd class="mt-1.5 text-body">第 {{ item.version }} 版</dd>
              </div>
              <div>
                <dt class="text-note text-label-secondary">来源类型</dt>
                <dd class="mt-1.5 text-body">
                  {{ sourceLabel }}{{ item.source_file_id ? '文件' : '' }}
                </dd>
              </div>
              <div v-if="item.source_url">
                <dt class="text-note text-label-secondary">来源链接</dt>
                <dd class="mt-1.5 break-all text-body">
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
                <dt class="text-note text-label-secondary">加入时间</dt>
                <dd class="mt-1.5 text-body">{{ knowledgeDate(item.imported_at) }}</dd>
              </div>
            </dl>
            <p class="mt-6 border-t border-hairline pt-4 text-note leading-6 text-label-secondary">
              来源信息为加入时的快照。原文件改名、移动或删除不影响已保存正文。
            </p>
          </aside>
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
