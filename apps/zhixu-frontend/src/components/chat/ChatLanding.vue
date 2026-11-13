<script setup lang="ts">
/**
 * 对话空态：第一句话的邀请与三条起手建议。
 *
 * 为什么单独成组件：这段是**连续正文段落**，按 DESIGN §Layout 必须收进
 * measure（40em / 640px）。而 ShellContract.test.ts:130 要求每页恰好一个容器档，
 * ChatPage 的档是 shell-wide（1152）。两个约束同时成立只有一条路 ——
 * 正文列下沉到子组件。所以这个文件和 ChatTranscript 是同一类约束的产物。
 *
 * 为什么 measure 上再挂 text-read：max-w-measure 是 40em，em 按元素自身
 * font-size 算。挂在 13px 上只有 520px，挂到 16px 才是 640px（R5 的 em 陷阱）。
 */
const emit = defineEmits<{ pick: [text: string] }>()

/** 三条起手建议：第一句测连通性，第二句测解释能力，第三句测上下文记忆。 */
const SUGGESTIONS = [
  '你好，请用一句话介绍你能做什么。',
  '用简单的语言解释什么是向量数据库。',
  '我的项目叫知序，请在本次对话中记住。',
] as const
</script>

<template>
  <div class="shell-measure text-read">
    <p class="mb-5 text-note tracking-widest text-primary-pressed">从一句话开始</p>
    <h2 class="text-display font-semibold leading-snug tracking-tight">
      先聊聊，<br />看看模型是否回应。
    </h2>
    <p class="mt-5 text-body leading-7 text-label-secondary">
      使用你在设置中连接的模型。<br />这里只发送本次会话中的文字，不调用任何工具。
    </p>
    <!-- 建议行：世界的「行」——行间一条 hairline，行内左文右箭头。
         箭头用 AppButton 的 icon-only 会让整行变成两个热区，所以保持整行可点。 -->
    <div class="mt-8 border-t border-hairline">
      <button
        v-for="suggestion in SUGGESTIONS"
        :key="suggestion"
        type="button"
        class="flex w-full items-center justify-between gap-4 border-b border-hairline py-4 text-left text-body transition-colors hover:text-primary-pressed"
        @click="emit('pick', suggestion)"
      >
        <span class="min-w-0">{{ suggestion }}</span>
        <span class="flex-none text-label-tertiary" aria-hidden="true">↗</span>
      </button>
    </div>
    <p class="mt-5 text-note text-label-secondary">对话自动保存，可在左侧历史记录中回看</p>
  </div>
</template>
