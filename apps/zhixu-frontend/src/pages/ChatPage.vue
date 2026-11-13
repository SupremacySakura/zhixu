<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import AppButton from '../components/ui/AppButton.vue'
import AppEmptyState from '../components/ui/AppEmptyState.vue'
import AppLoading from '../components/ui/AppLoading.vue'
import AppPanel from '../components/ui/AppPanel.vue'
import ChatLanding from '../components/chat/ChatLanding.vue'
import ChatTranscript from '../components/chat/ChatTranscript.vue'
import { AnimatePresence, Motion, animate, type PanInfo } from 'motion-v'
import { springBouncy, springSnappy, fadeTransition } from '../utils/motion'
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
// 桌面与移动端抽屉各持有一个会话列表实例，首轮对话完成后两个列表都要刷新
const sidebarDesktop = ref<InstanceType<typeof ConversationSidebar> | null>(null)
const sidebarMobile = ref<InstanceType<typeof ConversationSidebar> | null>(null)
const historyOpen = ref(false)
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
// 移动端抽屉内的选择：先关抽屉再走原有打开/新建流程
function selectConversation(id: string) {
  historyOpen.value = false
  void openConversation(id)
}
function newChatFromDrawer() {
  historyOpen.value = false
  newChat()
}
// 抽屉拖拽关闭（DESIGN.md §2.4 手势：速度方向优先，动量投影辅助判断，同 HomePage 导航抽屉）
const drawerPanel = ref<HTMLElement | null>(null)
const setDrawerPanelRef = (instance: unknown) => {
  drawerPanel.value =
    (instance as { $el?: HTMLElement } | null)?.$el ?? (instance as HTMLElement) ?? null
}
const handleDrawerPanEnd = (_event: unknown, info: PanInfo) => {
  const shouldClose = info.velocity.x < -400 || (info.velocity.x <= 0 && info.offset.x < -80)
  if (shouldClose) {
    historyOpen.value = false
    return
  }
  if (drawerPanel.value) animate(drawerPanel.value, { x: 0 }, springSnappy)
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
  const task = send(text, Number(model.value), selected.value.label)
  await nextTick()
  if (transcript.value) transcript.value.scrollTop = transcript.value.scrollHeight
  await task
  // A finished first turn creates the thread server-side; refresh the history list.
  sidebarDesktop.value?.reload()
  sidebarMobile.value?.reload()
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
  <!-- 外壳高度：100dvh 减掉移动端全局条。这里用 --spacing-topbar 令牌而不是写死 60，
       外壳高度一变，这一处跟着变。桌面端没有全局条，lg:h-auto 交还给父级 flex。 -->
  <div class="flex h-[calc(100dvh-var(--spacing-topbar))] min-h-0 lg:h-auto">
    <div class="hidden h-full lg:block">
      <ConversationSidebar
        ref="sidebarDesktop"
        :active-id="conversationId"
        @select="openConversation"
        @new="newChat"
      />
    </div>
    <main class="flex min-w-0 flex-1 flex-col bg-surface">
      <!-- 页头带 64px（与其它九页统一）。面包屑已取消：对话页不在
           「三层以上」那三处里，位置由侧栏的「对话」组说明。 -->
      <header class="shell-band">
        <div class="flex min-w-0 flex-1 items-center gap-2">
          <AppButton
            icon-only
            tone="quiet"
            icon="material-symbols:history-rounded"
            aria-label="打开会话历史"
            class="lg:hidden"
            @click="historyOpen = true"
          />
          <div class="min-w-0 flex-1">
            <h1 class="text-title">普通对话</h1>
            <p class="font-type text-micro text-label-tertiary">只发送本次会话中的文字</p>
          </div>
        </div>
        <div class="flex flex-none items-center gap-2">
          <RouterLink to="/home/settings" class="shell-band-action" title="模型设置"
            >模型设置<span class="hidden sm:inline"> ↗</span></RouterLink
          ><button type="button" class="shell-band-action-primary" @click="newChat">
            ＋<span class="hidden sm:inline"> 新对话</span>
          </button>
        </div>
      </header>
      <div class="shell-toolbar-gutter">
        <fieldset :disabled="running || loading" class="min-w-0">
          <FilterMenu
            :model-value="model"
            :options="options"
            label="选择对话模型"
            @update:model-value="changeModel"
          />
        </fieldset>
      </div>
      <!-- 消息区：滚动容器留在页面（滚动跟随与 ref 都指这里），
           正文列由 ChatTranscript 承载 measure —— ShellContract 要求每页
           恰好一个容器档，ChatPage 的是 shell-wide，所以 40em 必须下沉。 -->
      <section
        ref="transcript"
        class="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        aria-label="对话消息"
      >
        <div
          v-if="loading"
          role="status"
          class="flex items-center gap-2 p-8 text-subhead text-label-secondary"
        >
          <AppLoading size="sm" />正在读取模型配置…
        </div>
        <div
          v-else-if="loadingHistory"
          role="status"
          class="flex items-center gap-2 p-8 text-subhead text-label-secondary"
        >
          <AppLoading size="sm" />正在读取对话历史…
        </div>
        <div v-else-if="error" role="alert" class="p-8 text-subhead text-danger">
          {{ error }}<AppButton size="sm" class="ml-3" @click="load">重新加载</AppButton>
        </div>
        <div v-else-if="!selected" class="shell-wide px-6 py-16">
          <AppEmptyState
            title="先连接一个对话模型"
            description="请选择可用对话模型，或前往设置添加模型与凭据。普通对话不需要 Embedding 配置。"
            icon="material-symbols:link-rounded"
            size="lg"
          >
            <template #actions>
              <RouterLink to="/home/settings" class="knowledge-button-primary"
                >前往模型设置</RouterLink
              >
            </template>
          </AppEmptyState>
        </div>
        <!-- 空态：正文列同样下沉到子组件（measure 只能有一处，且页面档是 wide） -->
        <div v-else-if="!messages.length" class="shell-wide px-6 py-12 sm:px-10">
          <ChatLanding @pick="fillPrompt" />
        </div>
        <div v-else class="shell-wide px-6 py-7 sm:px-10">
          <ChatTranscript
            :messages="messages"
            :running="running"
            :loading-history="loadingHistory"
            :model-label="selected.label"
            @copy="copy"
            @retry="index => retry(index, Number(model), selected!.label)"
          />
        </div>
      </section>
      <div class="shell-wide shrink-0 px-4 pb-[max(1rem,env(safe-area-inset-bottom))] sm:px-8">
        <p role="status" class="min-h-7 text-caption text-label-secondary">
          {{
            running
              ? '正在生成 · 可停止'
              : incomplete
                ? '回复未完成，请重试或新建对话。重试可能产生调用费用。'
                : copyStatus
          }}
        </p>
        <!-- 输入区：世界里是一张「可写的纸」。用 AppPanel 的 quiet 档承载，
             只覆写两个变量 —— 底色换成 canvas（纸面比卡面更浅一档），
             描边换成 rule（48%，3:1，够当控件边界；hairline 只有 1.72，不行）。
             焦点时描边升到 primary。 -->
        <AppPanel
          tone="quiet"
          size="md"
          class="focus-within:border-primary"
          :style="{
            '--atom-tint-surface': 'var(--color-canvas)',
            '--atom-tint-border': 'var(--color-rule)',
          }"
        >
          <form @submit.prevent="submit">
            <label for="chat-prompt" class="sr-only">发送给模型的消息</label
            ><textarea
              id="chat-prompt"
              ref="input"
              v-model="prompt"
              maxlength="2000"
              rows="2"
              class="w-full resize-none bg-transparent text-subhead leading-7 outline-none"
              :disabled="!selected || running || loadingHistory || incomplete"
              placeholder="发送一条消息，测试你的模型连接…"
              @keydown="keydown"
            />
            <div
              class="mt-2 flex items-center justify-between gap-2 text-caption text-label-secondary"
            >
              <span class="hidden sm:inline">Enter 发送 · Shift + Enter 换行</span>
              <div class="ml-auto flex items-center gap-3">
                <span>{{ prompt.length }} / 2000</span>
                <AppButton v-if="running" @click="stop">■ 停止生成</AppButton>
                <!-- R12：这个按钮必须保持原生 <button disabled>。
                   AppButton 忙态用 aria-busy + click 早退，不设原生 disabled，
                   而 ChatPage.test.ts:82 断言这里的 disabled 是 defined。
                   禁用态用规则线降级 + 颜色降级，不用 opacity（DESIGN 明禁）。 -->
                <button
                  v-else
                  type="submit"
                  aria-label="发送消息"
                  class="flex h-9 w-9 items-center justify-center bg-primary-pressed text-xl text-white tap-target transition-colors hover:bg-primary disabled:border disabled:border-hairline disabled:bg-surface-secondary disabled:text-label-tertiary"
                  :disabled="!selected || !prompt.trim() || loadingHistory || incomplete"
                >
                  ↑
                </button>
              </div>
            </div>
          </form>
        </AppPanel>
        <p class="mt-3 text-center text-note leading-5 text-label-secondary">
          文字将发送至当前模型供应商；请勿输入敏感信息。模型回复可能不准确。
        </p>
      </div>
      <!-- 移动端会话历史抽屉：同 HomePage 导航抽屉模式（弹簧滑入 + 遮罩点击/拖拽左滑关闭） -->
      <AnimatePresence>
        <Motion
          v-if="historyOpen"
          key="chat-history-layer"
          :initial="{ opacity: 0 }"
          :animate="{ opacity: 1 }"
          :exit="{ opacity: 0 }"
          :transition="fadeTransition"
          class="fixed inset-0 z-menu flex lg:hidden"
          @click="historyOpen = false"
        >
          <div class="absolute inset-0 bg-scrim"></div>
          <Motion
            :ref="setDrawerPanelRef"
            :initial="{ x: '-100%' }"
            :animate="{ x: 0 }"
            :exit="{ x: '-100%' }"
            :transition="springBouncy"
            drag="x"
            :drag-constraints="{ left: -240, right: 0 }"
            :drag-elastic="{ left: 0.2, right: 0 }"
            :drag-momentum="false"
            role="dialog"
            aria-modal="true"
            aria-label="会话历史"
            class="relative flex h-full w-drawer flex-col bg-surface-secondary shadow-popover"
            @click.stop
            @pan-end="handleDrawerPanEnd"
          >
            <div
              class="flex flex-none items-center justify-between border-b border-hairline px-3 py-2.5"
            >
              <h2 class="px-1 text-caption font-semibold tracking-wide text-label-secondary">
                会话历史
              </h2>
              <AppButton
                icon-only
                tone="quiet"
                icon="material-symbols:close-rounded"
                aria-label="关闭会话历史"
                @click="historyOpen = false"
              />
            </div>
            <div class="min-h-0 flex-1">
              <ConversationSidebar
                ref="sidebarMobile"
                :active-id="conversationId"
                @select="selectConversation"
                @new="newChatFromDrawer"
              />
            </div>
          </Motion>
        </Motion>
      </AnimatePresence>
      <SettingsModal v-if="confirmation" title="开始新对话？" @close="confirmation = null"
        ><p class="mb-6 text-subhead leading-7 text-label-secondary">
          当前消息和草稿将清空，正在生成的回复会停止，无法恢复。切换模型只影响本次会话，不修改默认模型。
        </p>
        <div class="flex justify-end gap-3">
          <AppButton @click="confirmation = null">取消</AppButton>
          <AppButton tone="primary" @click="confirm">确认新建</AppButton>
        </div></SettingsModal
      >
    </main>
  </div>
</template>
