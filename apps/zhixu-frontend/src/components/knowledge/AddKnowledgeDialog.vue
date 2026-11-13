<script setup lang="ts">
import { nextTick, onMounted, onBeforeUnmount, ref, useId } from 'vue'
import { AnimatePresence, Motion } from 'motion-v'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import type { FileDisplayItem } from '../../types/file'
import { formatBytes } from '../../utils/file'
import { fadeTransition, springSnappy } from '../../utils/motion'
const props = defineProps<{ file: FileDisplayItem; title: string; busy: boolean; error: string }>()
const emit = defineEmits<{ 'update:title': [value: string]; close: []; submit: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const input = ref<HTMLInputElement | null>(null)
const id = useId()
let previous: HTMLElement | null = null
onMounted(async () => {
  previous = document.activeElement as HTMLElement | null
  dialog.value?.showModal()
  await nextTick()
  input.value?.focus()
})
onBeforeUnmount(() => {
  dialog.value?.close()
  previous?.focus()
})
function close() {
  if (!props.busy) emit('close')
}
</script>
<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="atom-dialog-shell"
      :aria-labelledby="id"
      @cancel.prevent="close"
      @click.self="close"
    >
      <AnimatePresence>
        <Motion :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :transition="fadeTransition">
          <Motion
            :initial="{ opacity: 0, scale: 0.96 }"
            :animate="{ opacity: 1, scale: 1 }"
            :transition="springSnappy"
            class="bg-surface shadow-popover"
          >
            <form @submit.prevent="emit('submit')">
              <div class="p-6 sm:p-7">
                <div class="mb-5 flex items-center justify-between gap-3">
                  <h2 :id="id" class="text-title">加入知识库</h2>
                  <button
                    type="button"
                    class="atom-icon-btn"
                    :disabled="busy"
                    aria-label="关闭弹窗"
                    @click="close"
                  >
                    <AppIcon icon="material-symbols:close-rounded" />
                  </button>
                </div>
                <div class="mb-5 flex items-center gap-3 border border-hairline p-4">
                  <span
                    class="flex h-11 w-10 shrink-0 items-center justify-center bg-primary-tint text-note font-bold text-primary-pressed"
                    >MD</span
                  >
                  <div class="min-w-0">
                    <p class="break-all text-body font-semibold">{{ file.name }}</p>
                    <p class="mt-1 text-note text-label-secondary">
                      {{ formatBytes(file.size) }} · Markdown 文件
                    </p>
                  </div>
                </div>
                <label :for="`${id}-input`" class="mb-2 block text-body font-semibold"
                  >知识标题</label
                >
                <input
                  :id="`${id}-input`"
                  ref="input"
                  :value="title"
                  class="atom-field min-h-9 w-full"
                  :disabled="busy"
                  :aria-invalid="Boolean(error)"
                  :aria-describedby="`${id}-help`"
                  @input="emit('update:title', ($event.target as HTMLInputElement).value)"
                />
                <p :id="`${id}-help`" class="mt-2 text-note text-label-secondary">
                  默认使用文件名，也可以换成更方便查找的标题。
                </p>
                <p v-if="error" role="alert" class="mt-3 text-body text-warning-strong">
                  {{ error }}
                </p>
                <p class="mt-5 text-body leading-7 text-label-secondary">
                  系统将提取 Markdown 正文并创建知识条目。<br />原文件仍保留，不会被修改或移动。<br />本次不执行
                  AI 摘要或向量化。
                </p>
              </div>
              <div class="flex justify-end gap-2 border-t border-hairline p-5">
                <button
                  type="button"
                  class="atom-btn atom-tone-secondary atom-btn-wide"
                  :disabled="busy"
                  @click="close"
                >
                  取消
                </button>
                <button
                  type="submit"
                  class="atom-btn atom-tone-primary atom-btn-lg"
                  :disabled="busy || !title.trim()"
                >
                  <AppLoading v-if="busy" size="sm" />{{ busy ? '提交中…' : '加入知识库' }}
                </button>
              </div>
            </form>
          </Motion>
        </Motion>
      </AnimatePresence>
    </dialog>
  </Teleport>
</template>
