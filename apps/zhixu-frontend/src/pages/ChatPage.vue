<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import SettingsModal from '../components/ui/SettingsModal.vue'
import ConversationSidebar from '../components/chat/ConversationSidebar.vue'
import { useAISettings } from '../composables/useAISettings'
import { useChat } from '../composables/useChat'
const { configuration, loading, error, load } = useAISettings()
const {
  messages,
  running,
  conversationId,
  loadingHistory,
  stop,
  reset,
  loadConversation,
  send,
  retry,
} = useChat()
const model = ref('0')
const prompt = ref('')
const ragEnabled = ref(true)
const sidebar = ref<InstanceType<typeof ConversationSidebar> | null>(null)
const hasEmbedding = computed(() => (configuration.value?.settings.embedding_model_id || 0) > 0)
const transcript = ref<HTMLElement | null>(null)
const input = ref<HTMLTextAreaElement | null>(null)
const confirmation = ref<string | null>(null)
const copyStatus = ref('')
const options = computed(
  () =>
    configuration.value?.providers.flatMap(p =>
      p.models
        .filter(m => m.kind === 'chat' && (p.auth_type === 'none' || p.credential_configured))
        .map(m => ({ key: String(m.id), label: `${p.name} / ${m.model_id}` })),
    ) || [],
)
const selected = computed(() => options.value.find(o => o.key === model.value))
const incomplete = computed(() => {
  const last = messages.value.at(-1)
  return last?.role === 'assistant' && last.status !== 'completed' && !running.value
})
watch(configuration, value => {
  model.value = String(value?.settings.chat_model_id || 0)
})
watch(
  () => messages.value.map(m => m.content).join(''),
  async () => {
    const el = transcript.value
    const follow = el && el.scrollHeight - el.scrollTop - el.clientHeight < 120
    await nextTick()
    if (follow && el) el.scrollTop = el.scrollHeight
  },
)
function changeModel(value: string) {
  if (messages.value.length || prompt.value) confirmation.value = value
  else model.value = value
}
function newChat() {
  if (messages.value.length || prompt.value) confirmation.value = 'new'
  else reset()
}
async function openConversation(id: string) {
  if (running.value) return
  try {
    await loadConversation(id)
    await nextTick()
    if (transcript.value) transcript.value.scrollTop = transcript.value.scrollHeight
  } catch {
    /* detail load failure surfaced by global handler */
  }
}
function confirm() {
  reset()
  prompt.value = ''
  if (confirmation.value !== 'new') model.value = confirmation.value!
  confirmation.value = null
}
async function submit() {
  if (
    !selected.value ||
    !prompt.value.trim() ||
    running.value ||
    loadingHistory.value ||
    incomplete.value
  )
    return
  const text = prompt.value
  prompt.value = ''
  const task = send(
    text,
    Number(model.value),
    selected.value.label,
    ragEnabled.value && hasEmbedding.value,
  )
  await nextTick()
  if (transcript.value) transcript.value.scrollTop = transcript.value.scrollHeight
  await task
  // A finished first turn creates the thread server-side; refresh the history list.
  sidebar.value?.reload()
  input.value?.focus()
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    void submit()
  }
}
function fillPrompt(text: string) {
  prompt.value = text
  input.value?.focus()
}
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copyStatus.value = '回复已复制'
  } catch {
    copyStatus.value = '复制失败，请手动选中文字复制'
  }
}
onMounted(() => {
  void load()
})
</script>
<template>
  <div class="flex h-[calc(100dvh-60px)] min-h-0 lg:h-auto">
    <ConversationSidebar
      ref="sidebar"
      :active-id="conversationId"
      @select="openConversation"
      @new="newChat"
    />
    <main class="flex min-w-0 flex-1 flex-col bg-surface">
      <header class="flex flex-wrap items-center justify-between gap-3 px-5 py-5 sm:px-8">
        <div>
          <p class="mb-1 text-caption text-label-secondary">工作空间 / 对话</p>
          <h1 class="text-title-3">普通对话</h1>
        </div>
        <div class="flex items-center gap-4 text-caption">
          <RouterLink to="/home/settings" class="text-primary-pressed">模型设置 ↗</RouterLink
          ><button type="button" class="knowledge-button-secondary" @click="newChat">
            ＋ 新对话
          </button>
        </div>
      </header>
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-y border-hairline px-5 py-3 sm:px-8"
      >
        <fieldset :disabled="running || loading" class="min-w-0">
          <FilterMenu
            :model-value="model"
            :options="options"
            label="选择对话模型"
            @update:model-value="changeModel"
          />
        </fieldset>
        <label
          class="flex items-center gap-2 text-caption"
          :class="hasEmbedding ? 'text-label-primary' : 'text-label-secondary'"
        >
          <input
            v-model="ragEnabled"
            type="checkbox"
            :disabled="!hasEmbedding || running"
            class="h-4 w-4 accent-primary-pressed"
          />
          知识库检索{{ hasEmbedding ? '' : '（需先配置 Embedding 模型）' }}
        </label>
      </div>
      <section
        ref="transcript"
        class="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        aria-label="对话消息"
      >
        <div v-if="loading" role="status" class="p-8 text-subhead text-label-secondary">
          正在读取模型配置…
        </div>
        <div v-else-if="loadingHistory" role="status" class="p-8 text-subhead text-label-secondary">
          正在读取对话历史…
        </div>
        <div v-else-if="error" role="alert" class="p-8 text-subhead text-danger">
          {{ error
          }}<button type="button" class="knowledge-text-action ml-3" @click="load">重新加载</button>
        </div>
        <div v-else-if="!selected" class="mx-auto max-w-2xl px-6 py-16 text-center">
          <h2 class="text-title-2">先连接一个对话模型</h2>
          <p class="my-5 text-subhead leading-7 text-label-secondary">
            请选择可用对话模型，或前往设置添加模型与凭据。<br />普通对话不需要 Embedding 配置。
          </p>
          <RouterLink to="/home/settings" class="knowledge-button-primary">前往模型设置</RouterLink>
        </div>
        <div v-else-if="!messages.length" class="mx-auto max-w-3xl px-6 py-12 sm:px-10">
          <p class="mb-5 text-caption tracking-widest text-primary-pressed">从一句话开始</p>
          <h2 class="text-[32px] font-semibold leading-snug tracking-tight sm:text-[42px]">
            先聊聊，<br />看看模型是否回应。
          </h2>
          <p class="mt-5 text-subhead leading-7 text-label-secondary">
            使用你在设置中连接的模型。<br />这里只发送本次会话中的文字，不调用任何工具。
          </p>
          <div class="mt-8 max-w-lg border-t border-hairline">
            <button
              v-for="suggestion in [
                '你好，请用一句话介绍你能做什么。',
                '用简单的语言解释什么是向量数据库。',
                '我的项目叫知序，请在本次对话中记住。',
              ]"
              :key="suggestion"
              type="button"
              class="flex w-full justify-between gap-4 border-b border-hairline py-4 text-left text-subhead hover:text-primary-pressed"
              @click="fillPrompt(suggestion)"
            >
              {{ suggestion }}<span>↗</span>
            </button>
          </div>
          <p class="mt-5 text-caption text-label-secondary">对话自动保存，可在左侧历史记录中回看</p>
        </div>
        <div v-else class="mx-auto max-w-3xl space-y-8 px-6 py-7 sm:px-10">
          <article
            v-for="(message, index) in messages"
            :key="index"
            :class="message.role === 'user' ? 'flex flex-col items-end' : ''"
          >
            <p
              class="mb-3 text-caption"
              :class="message.role === 'user' ? 'text-label-secondary' : 'text-primary-pressed'"
            >
              {{ message.role === 'user' ? '你' : message.model }}
            </p>
            <div
              class="whitespace-pre-wrap break-words text-body leading-8"
              :class="
                message.role === 'user'
                  ? 'max-w-[90%] rounded-lg bg-surface-secondary px-5 py-3'
                  : ''
              "
            >
              {{
                message.content ||
                (message.status === 'generating' ? '等待首段回复…' : '未收到文字回复')
              }}
            </div>
            <div
              v-if="message.role === 'assistant'"
              class="mt-3 flex flex-wrap items-center gap-4 text-caption text-label-secondary"
            >
              <span>{{
                message.status === 'generating'
                  ? '正在生成…'
                  : message.status === 'completed'
                    ? '回复完成'
                    : message.status === 'stopped'
                      ? '已停止 · 部分内容'
                      : '调用未完成'
              }}</span
              ><button
                v-if="message.content && message.status !== 'generating'"
                type="button"
                @click="copy(message.content)"
              >
                复制回复</button
              ><button
                v-if="['failed', 'stopped'].includes(message.status || '')"
                type="button"
                :disabled="running || loadingHistory"
                class="text-primary-pressed"
                @click="retry(index, Number(model), selected!.label)"
              >
                重试这条消息
              </button>
            </div>
            <details
              v-if="message.sources?.length"
              class="mt-3 rounded-md border border-hairline bg-surface-secondary px-4 py-3 text-caption"
            >
              <summary class="cursor-pointer text-label-primary">
                参考来源（{{ message.sources.length }}）<span
                  v-if="message.query"
                  class="text-label-secondary"
                >
                  · 检索词「{{ message.query }}」</span
                >
              </summary>
              <ol class="mt-3 space-y-3">
                <li v-for="(source, order) in message.sources" :key="order">
                  <RouterLink
                    :to="`/home/knowledge/${source.knowledge_id}/chunks`"
                    class="text-primary-pressed"
                    >[{{ order + 1 }}] {{ source.title || '未命名文档' }}</RouterLink
                  >
                  <span class="ml-2 text-label-secondary">{{ source.section }}</span>
                  <p class="mt-1 whitespace-pre-wrap break-words leading-6 text-label-secondary">
                    {{ source.content }}
                  </p>
                </li>
              </ol>
            </details>
            <p
              v-if="message.error"
              role="alert"
              class="mt-3 rounded-sm border-l-2 border-danger bg-danger-tint p-4 text-subhead leading-7 text-danger"
            >
              {{ message.error
              }}<RouterLink to="/home/settings" class="ml-3 underline">检查模型设置</RouterLink>
            </p>
          </article>
        </div>
      </section>
      <div class="mx-auto w-full max-w-3xl shrink-0 px-4 pb-4 sm:px-8">
        <p role="status" class="min-h-7 text-caption text-label-secondary">
          {{
            running
              ? '正在生成 · 可停止'
              : incomplete
                ? '回复未完成，请重试或新建对话。重试可能产生调用费用。'
                : copyStatus
          }}
        </p>
        <form
          class="rounded-xl border border-hairline bg-canvas p-4 focus-within:border-primary focus-within:ring-2 focus-within:ring-primary-tint"
          @submit.prevent="submit"
        >
          <label for="chat-prompt" class="sr-only">发送给模型的消息</label
          ><textarea
            id="chat-prompt"
            ref="input"
            v-model="prompt"
            maxlength="8000"
            rows="2"
            class="w-full resize-none bg-transparent text-subhead leading-7 outline-none"
            :disabled="!selected || running || loadingHistory || incomplete"
            placeholder="发送一条消息，测试你的模型连接…"
            @keydown="keydown"
          />
          <div
            class="mt-2 flex items-center justify-between gap-2 text-caption text-label-secondary"
          >
            <span>Enter 发送 · Shift + Enter 换行</span>
            <div class="flex items-center gap-3">
              <span>{{ prompt.length }} / 8000</span
              ><button
                v-if="running"
                type="button"
                class="knowledge-button-secondary"
                @click="stop"
              >
                ■ 停止生成</button
              ><button
                v-else
                type="submit"
                aria-label="发送消息"
                class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-pressed text-xl text-white disabled:opacity-40"
                :disabled="!selected || !prompt.trim() || loadingHistory || incomplete"
              >
                ↑
              </button>
            </div>
          </div>
        </form>
        <p class="mt-3 text-center text-[10px] leading-5 text-label-secondary">
          文字将发送至当前模型供应商；请勿输入敏感信息。模型回复可能不准确。
        </p>
      </div>
      <SettingsModal v-if="confirmation" title="开始新对话？" @close="confirmation = null"
        ><p class="mb-6 text-subhead leading-7 text-label-secondary">
          当前消息和草稿将清空，正在生成的回复会停止，无法恢复。切换模型只影响本次会话，不修改默认模型。
        </p>
        <div class="flex justify-end gap-3">
          <button type="button" class="knowledge-button-secondary" @click="confirmation = null">
            取消</button
          ><button type="button" class="knowledge-button-primary" @click="confirm">确认新建</button>
        </div></SettingsModal
      >
    </main>
  </div>
</template>
