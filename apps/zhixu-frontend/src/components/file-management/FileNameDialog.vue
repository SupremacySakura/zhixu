<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import AppButton from '../ui/AppButton.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { fadeTransition, springSnappy } from '../../utils/motion'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title: string
    value: string
    label: string
    placeholder?: string
    description?: string | null
    loading?: boolean
    confirmText?: string
    loadingText?: string
    cancelText?: string
  }>(),
  {
    placeholder: '',
    description: null,
    loading: false,
    confirmText: '确认',
    loadingText: '处理中...',
    cancelText: '取消',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:value': [value: string]
  confirm: []
  cancel: []
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const titleId = `file-name-dialog-title-${useId()}`
const isConfirmDisabled = computed(() => props.loading || props.value.trim().length === 0)

const handleInput = (event: Event) => {
  const input = event.currentTarget
  if (input instanceof HTMLInputElement) emit('update:value', input.value)
}

const handleCancel = () => {
  if (props.loading) return
  emit('update:modelValue', false)
  emit('cancel')
}

const handleConfirm = () => {
  if (isConfirmDisabled.value) return
  emit('confirm')
}

watch(
  () => props.modelValue,
  async visible => {
    if (!visible) return
    await nextTick()
    inputRef.value?.focus()
    inputRef.value?.select()
  },
)
</script>

<template>
  <Teleport to="body">
    <!-- 遮罩淡入；loading 期间点击遮罩 / Esc 不关闭 -->
    <AnimatePresence>
      <Motion
        v-if="modelValue"
        key="file-name-overlay"
        :initial="{ opacity: 0 }"
        :animate="{ opacity: 1 }"
        :exit="{ opacity: 0 }"
        :transition="fadeTransition"
        class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @click.self="handleCancel"
        @keydown.esc="handleCancel"
      >
        <!-- 面板：scale 0.96→1 + 淡入（motion-v 弹簧，第 8 步） -->
        <Motion
          :initial="{ opacity: 0, scale: 0.96 }"
          :animate="{ opacity: 1, scale: 1 }"
          :exit="{ opacity: 0, scale: 0.98 }"
          :transition="springSnappy"
          class="w-full max-w-md bg-surface p-6 shadow-popover"
        >
          <h3 :id="titleId" class="mb-2 text-title">{{ title }}</h3>
          <slot name="description">
            <p v-if="description" class="mb-4 text-note text-label-secondary">{{ description }}</p>
          </slot>

          <div class="mb-6" :class="description ? '' : 'mt-4'">
            <label class="mb-1.5 block text-note font-semibold text-label-secondary">
              {{ label }}
            </label>
            <input
              ref="inputRef"
              :value="value"
              type="text"
              class="h-9 w-full border border-hairline bg-surface-secondary px-3 text-body text-label transition-[border-color,background-color,box-shadow] placeholder:text-label-tertiary focus:border-primary focus:bg-surface disabled:opacity-60"
              :placeholder="placeholder"
              :disabled="loading"
              @input="handleInput"
              @keyup.enter="handleConfirm"
            />
          </div>

          <div class="flex justify-end gap-3">
            <AppButton :disabled="loading" @click="handleCancel">{{ cancelText }}</AppButton>
            <AppButton
              tone="primary"
              :disabled="isConfirmDisabled"
              :loading="loading"
              @click="handleConfirm"
            >
              {{ loading ? loadingText : confirmText }}
            </AppButton>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
