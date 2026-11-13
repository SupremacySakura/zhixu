<script setup lang="ts">
/**
 * 对话消息流：世界的「长文阅读」原型（②）在对话页的落点。
 *
 * 为什么它必须是子组件而不是 ChatPage 里的一段模板：
 * `ShellContract.test.ts:130` 要求每页**恰好一个**容器档，ChatPage 的是
 * `shell-wide`（1152px）。而 DESIGN §Layout 规定「页内一旦出现连续正文段落，
 * 那段必须收进 measure」。两个约束同时成立只有一条路——把正文列下沉到子组件，
 * 页面级容器档保持唯一。所以这个文件是约束的产物，不是重构的副产物。
 *
 * 为什么 measure 上还挂 `text-read`：`max-w-measure` 是 `40em`，em 按**元素自身**
 * font-size 算。挂在 13px 的容器上只有 520px，挂在 16px 上才是 640px（R5 的
 * em 陷阱）。所以正文列的外框必须是 16px —— 里面的 caption/note 各自覆盖回小字。
 *
 * 为什么引文带用 `border-rule` 而不是 `border-hairline`：世界只有两种线，
 * hairline（墨 26%）是装饰纹理，rule（墨 48%）才够 3:1。引文带是「一段被引用
 * 的别人的话」，它需要能被读出来是一块独立内容，所以用 rule 的上下短横
 * （DESIGN §视觉语法「规则线母题」：引文带上下各一条 1px 短横）。
 */
import { RouterLink } from 'vue-router'
import AppButton from '../ui/AppButton.vue'
import type { ChatMessage } from '../../composables/useChat'
import type { ChatSource } from '../../services/apis/chat'

defineProps<{
  messages: ChatMessage[]
  running: boolean
  loadingHistory: boolean
  /** 当前选中的模型标签，重试时要带上，否则重试会记成别的模型。 */
  modelLabel: string
}>()

const emit = defineEmits<{ copy: [text: string]; retry: [index: number] }>()

/**
 * 引用跳转：记了分块号就落到那一块，否则落到文档的分块列表。
 * 这是唯一把「回答」和「原文」连起来的地方，不能让调用方各写一份。
 */
function citationTarget(source: ChatSource) {
  const path = `/home/knowledge/${source.knowledge_id}/chunks`
  return source.seq ? { path, query: { chunk: String(source.seq) } } : path
}

/** 一条助手回复的状态词。四个状态一个都不能少，它们是「这句话到底说完没」的全部信息。 */
function statusText(message: ChatMessage) {
  if (message.status === 'generating') return '正在生成…'
  if (message.status === 'completed') return '回复完成'
  if (message.status === 'stopped') return '已停止 · 部分内容'
  return '调用未完成'
}
</script>

<template>
  <div class="shell-measure text-read">
    <div class="space-y-8 py-7">
      <article
        v-for="(message, index) in messages"
        :key="index"
        :class="message.role === 'user' ? 'flex flex-col items-end' : ''"
      >
        <p
          class="mb-3 text-note"
          :class="message.role === 'user' ? 'text-label-secondary' : 'text-primary-pressed'"
        >
          {{ message.role === 'user' ? '你' : message.model }}
        </p>
        <div
          class="whitespace-pre-wrap break-words text-read"
          :class="message.role === 'user' ? 'max-w-[90%] bg-surface-secondary px-5 py-3' : ''"
        >
          {{
            message.content ||
            (message.status === 'generating' ? '等待首段回复…' : '未收到文字回复')
          }}
        </div>
        <div
          v-if="message.role === 'assistant'"
          class="mt-3 flex flex-wrap items-center gap-4 text-note text-label-secondary"
        >
          <span>{{ statusText(message) }}</span>
          <AppButton
            v-if="message.content && message.status !== 'generating'"
            tone="quiet"
            size="sm"
            @click="emit('copy', message.content)"
          >
            复制回复
          </AppButton>
          <AppButton
            v-if="['failed', 'stopped'].includes(message.status || '')"
            tone="quiet"
            size="sm"
            :disabled="running || loadingHistory"
            @click="emit('retry', index)"
          >
            重试这条消息
          </AppButton>
        </div>
        <!-- 引文带：上下各一条 rule 短横，中间是被引用的原文。
             折叠行为不变（<details>），摘要文案一字未改。 -->
        <details v-if="message.sources?.length" class="mt-3 border-y border-rule py-3 text-note">
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
              <RouterLink :to="citationTarget(source)" class="text-primary-pressed"
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
          class="mt-3 border-l-2 border-danger bg-danger-tint p-4 text-body leading-7 text-danger"
        >
          {{ message.error
          }}<RouterLink to="/home/settings" class="ml-3 underline">检查模型设置</RouterLink>
        </p>
      </article>
    </div>
  </div>
</template>
