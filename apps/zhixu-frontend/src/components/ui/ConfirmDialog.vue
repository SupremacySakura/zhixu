<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../../utils/motion'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title?: string
    message?: string
    confirmText?: string
    cancelText?: string
    loading?: boolean
    danger?: boolean
  }>(),
  {
    title: '确认操作',
    message: '',
    confirmText: '确认',
    cancelText: '取消',
    loading: false,
    danger: false,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'cancel'): void
  (e: 'confirm'): void
}>()

const confirmButtonRef = ref<HTMLButtonElement | null>(null)

const handleClose = () => {
  if (props.loading) return
  emit('update:modelValue', false)
  emit('cancel')
}

const handleConfirm = () => {
  if (props.loading) return
  emit('confirm')
}

// 弹窗打开时聚焦确认按钮
watch(
  () => props.modelValue,
  visible => {
    if (visible) {
      nextTick(() => {
        confirmButtonRef.value?.focus()
      })
    }
  },
)
</script>

<template>
  <Teleport to="body">
    <!-- 遮罩淡入淡出；loading 期间点击遮罩不关闭；弹簧由 motion-v 驱动（第 8 步） -->
    <AnimatePresence>
      <Motion
        v-if="modelValue"
        key="confirm-overlay"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="'dialog-title-' + title"
        @click="handleClose"
      >
        <!-- 面板：scale 0.96→1 + 淡入（DESIGN.md §5.3） -->
        <Motion
          :initial="{ opacity: 0, scale: 0.96 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.98 }"
          :transition="springSnappy"
          class="w-full max-w-md bg-surface p-6 shadow-popover"
          @click.stop
        >
          <h3 :id="'dialog-title-' + title" class="text-title-3 mb-2">{{ title }}</h3>
          <p v-if="message" class="text-body mb-6 text-label-secondary">{{ message }}</p>
          <slot />

          <div class="flex justify-end gap-3">
            <button
              type="button"
              :disabled="loading"
              class="h-10 bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-colors hover:bg-surface-secondary disabled:opacity-60"
              @click="handleClose"
            >
              {{ cancelText }}
            </button>
            <button
              ref="confirmButtonRef"
              type="button"
              :disabled="loading"
              class="flex h-10 items-center gap-2 px-5 text-sm font-semibold transition-colors disabled:opacity-60"
              :class="danger ? 'atom-btn atom-tone-danger' : 'knowledge-button-primary'"
              @click="handleConfirm"
            >
              <slot name="confirm-icon" />
              {{ loading ? '处理中...' : confirmText }}
            </button>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
