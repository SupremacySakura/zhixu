<script setup lang="ts">
import { ref, useId, onMounted, onBeforeUnmount } from 'vue'
import { Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
defineProps<{ title: string; busy?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const id = useId()
let previous: HTMLElement | null = null
onMounted(() => {
  previous = document.activeElement as HTMLElement
  dialog.value?.showModal()
})
onBeforeUnmount(() => {
  dialog.value?.close()
  previous?.focus()
})
</script>
<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="atom-dialog-shell settings-dialog w-full max-w-3xl"
      :aria-labelledby="id"
      @cancel.prevent="!busy && emit('close')"
      @click.self="!busy && emit('close')"
    >
      <Motion
        :initial="{ opacity: 0, scale: 0.96 }"
        :animate="{ opacity: 1, scale: 1 }"
        :transition="springSnappy"
        class="bg-surface p-6 shadow-popover sm:p-7"
      >
        <header class="mb-5 flex items-center justify-between gap-4">
          <h2 :id="id" class="text-title">{{ title }}</h2>
          <button
            type="button"
            aria-label="关闭弹窗"
            class="atom-icon-btn"
            :disabled="busy"
            @click="emit('close')"
          >
            ×
          </button>
        </header>
        <slot />
      </Motion>
    </dialog>
  </Teleport>
</template>

<style scoped>
/* 这层壳自己会滚（atom-dialog-shell 带 overflow-y: auto），
   但它的滚动条挂在**遮罩**上、不在纸面上：全局的纸色 thumb 压在这层
   透明壳上会显得悬空，所以这里让它退回不可见。

   与全局方案的关系：全局样式在 @layer base，scoped 样式在 Vue 的
   [data-v-*] 属性选择器上，特异性更高，两者不打架；Firefox 那条
   scrollbar-width: none 也是同理覆盖 @supports not selector() 那段。
   这是全仓**唯一**允许隐藏滚动条的地方 —— 因为这里根本没有「纸面」
   可以承载纸条。 */
.settings-dialog {
  scrollbar-width: none;
}

.settings-dialog::-webkit-scrollbar {
  display: none;
}
</style>
