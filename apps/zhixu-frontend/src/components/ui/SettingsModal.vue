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
.settings-dialog {
  scrollbar-width: none;
}

.settings-dialog::-webkit-scrollbar {
  display: none;
}
</style>
