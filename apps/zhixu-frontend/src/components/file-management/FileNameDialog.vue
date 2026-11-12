<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
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
        class="fixed inset-0 z-50 flex items-center justify-center bg-scrim p-4 backdrop-blur-sm"
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
          class="w-full max-w-md rounded-lg bg-surface p-6 shadow-popover"
        >
          <h3 :id="titleId" class="text-title-3 mb-2">{{ title }}</h3>
          <slot name="description">
            <p v-if="description" class="text-body mb-4 text-label-secondary">{{ description }}</p>
          </slot>

          <div class="mb-6" :class="description ? '' : 'mt-4'">
            <label class="mb-1.5 block text-[13px] font-semibold text-label-secondary">
              {{ label }}
            </label>
            <input
              ref="inputRef"
              :value="value"
              type="text"
              class="h-11 w-full rounded-sm border border-hairline bg-surface-secondary px-3.5 text-[15px] text-label transition-[border-color,background-color,box-shadow] duration-150 placeholder:text-label-tertiary focus:border-primary focus:bg-surface focus:outline-none focus:ring-[3px] focus:ring-primary-tint disabled:opacity-60"
              :placeholder="placeholder"
              :disabled="loading"
              @input="handleInput"
              @keyup.enter="handleConfirm"
            />
          </div>

          <div class="flex justify-end gap-3">
            <button
              class="h-10 rounded-full bg-surface px-5 text-sm font-semibold text-label-secondary ring-1 ring-hairline transition-[background-color,scale] duration-150 hover:bg-surface-secondary active:scale-[0.97] disabled:opacity-60"
              type="button"
              :disabled="loading"
              @click="handleCancel"
            >
              {{ cancelText }}
            </button>
            <button
              class="flex h-10 items-center gap-2 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed disabled:opacity-60"
              type="button"
              :disabled="isConfirmDisabled"
              @click="handleConfirm"
            >
              <AppIcon
                v-if="loading"
                icon="material-symbols:progress-activity"
                class="animate-spin"
              />
              {{ loading ? loadingText : confirmText }}
            </button>
          </div>
        </Motion>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
