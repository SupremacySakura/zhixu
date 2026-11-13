<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { AnimatePresence, Motion } from 'motion-v'
import AppIcon from '../ui/AppIcon.vue'
import AppLoading from '../ui/AppLoading.vue'
import { importKnowledgeFromWeb, type KnowledgeItem } from '../../services/apis/knowledge'
import { knowledgeError } from '../../utils/knowledge'
import { fadeTransition, springSnappy } from '../../utils/motion'
const emit = defineEmits<{ close: []; imported: [item: KnowledgeItem] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const input = ref<HTMLInputElement | null>(null)
const id = useId()
const url = ref('')
const busy = ref(false)
const error = ref('')
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
  if (!busy.value) emit('close')
}
async function submit() {
  if (busy.value) return
  const value = url.value.trim()
  if (!value) {
    error.value = '请填写网页链接'
    return
  }
  busy.value = true
  error.value = ''
  try {
    emit('imported', await importKnowledgeFromWeb(value))
  } catch (e) {
    error.value = knowledgeError(e)
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="knowledge-dialog"
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
            <form @submit.prevent="submit">
              <div class="p-6 sm:p-7">
                <div class="mb-5 flex items-center justify-between gap-3">
                  <h2 :id="id" class="text-title-3">从网页导入</h2>
                  <button
                    type="button"
                    class="knowledge-icon-button"
                    :disabled="busy"
                    aria-label="关闭弹窗"
                    @click="close"
                  >
                    <AppIcon icon="material-symbols:close-rounded" />
                  </button>
                </div>
                <label :for="`${id}-url`" class="mb-2 block text-subhead font-semibold"
                  >网页链接</label
                >
                <input
                  :id="`${id}-url`"
                  ref="input"
                  v-model="url"
                  type="url"
                  class="knowledge-input w-full"
                  placeholder="https://example.com/article"
                  :disabled="busy"
                  :aria-invalid="Boolean(error)"
                  :aria-describedby="`${id}-help`"
                />
                <p :id="`${id}-help`" class="mt-2 text-caption text-label-secondary">
                  仅支持公开可访问的 http/https 网页；标题自动取自网页。
                </p>
                <p v-if="error" role="alert" class="mt-3 text-subhead text-warning-strong">
                  {{ error }}
                </p>
                <p class="mt-5 text-subhead leading-7 text-label-secondary">
                  系统会抓取网页正文并转为 Markdown，再自动分块与向量化。<br />内网地址、需要登录的页面与超过
                  10MB 的页面无法导入。
                </p>
              </div>
              <div class="flex justify-end gap-2 border-t border-hairline p-5">
                <button
                  type="button"
                  class="knowledge-button-secondary"
                  :disabled="busy"
                  @click="close"
                >
                  取消
                </button>
                <button
                  type="submit"
                  class="knowledge-button-primary"
                  :disabled="busy || !url.trim()"
                >
                  <AppLoading v-if="busy" size="sm" />{{ busy ? '抓取中…' : '开始导入' }}
                </button>
              </div>
            </form>
          </Motion>
        </Motion>
      </AnimatePresence>
    </dialog>
  </Teleport>
</template>
