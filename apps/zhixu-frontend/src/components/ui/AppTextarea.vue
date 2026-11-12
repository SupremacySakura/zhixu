<script setup lang="ts">
/**
 * 多行输入零件。
 *
 * 为什么高度不写死三档：多行框的高度由 rows 决定，钉死 26/32/36 会让
 * 「三行备注框」变成三行挤在 36px 里。这里三档只决定**字号与内边距**，
 * 高度交给调用方用 rows 说清楚。
 *
 * 为什么不写 resize-none：世界里纸是可以被拉长的，备注框尤其需要。
 * 默认 resize-y 允许纵向拉伸，横向拉伸会破坏版面，所以禁掉。
 */
import { computed, useAttrs, useId } from 'vue'
import AppLoading from './AppLoading.vue'

const props = withDefaults(
  defineProps<{
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    rows?: number
    disabled?: boolean
    loading?: boolean
    label?: string
    hint?: string
    error?: string
    placeholder?: string
  }>(),
  {
    tone: 'secondary',
    size: 'md',
    rows: 3,
    disabled: false,
    loading: false,
    label: '',
    hint: '',
    error: '',
    placeholder: '',
  },
)

const model = defineModel<string>({ default: '' })
const attrs = useAttrs()
const id = useId()

const inputId = computed(() => (attrs.id as string | undefined) || id)

const SCALES = {
  sm: { height: 26, padding: 8, font: 'var(--text-note)' },
  md: { height: 32, padding: 12, font: 'var(--text-body)' },
  lg: { height: 36, padding: 14, font: 'var(--text-lead)' },
} as const

const TONE_CLASS = {
  primary: 'atom-tone-primary',
  secondary: 'atom-tone-secondary',
  quiet: 'atom-tone-quiet',
  danger: 'atom-tone-danger',
} as const

const scale = computed(() => SCALES[props.size])
const toneClass = computed(() => (props.error ? 'atom-tone-danger' : TONE_CLASS[props.tone]))

const rootStyle = computed(() => ({
  '--atom-control-height': `${scale.value.height}px`,
  '--atom-control-padding': `${scale.value.padding}px`,
  '--atom-control-font-size': scale.value.font,
}))

const describedBy = computed(() => {
  if (!props.hint && !props.error) return undefined
  return `${inputId.value}-description`
})
</script>

<template>
  <div class="atom-field-shell" :style="rootStyle">
    <label v-if="label" :for="inputId" class="atom-field-label">{{ label }}</label>
    <textarea
      :id="inputId"
      v-model="model"
      :rows="rows"
      :placeholder="placeholder"
      :disabled="disabled || undefined"
      :readonly="loading || undefined"
      :aria-busy="loading ? 'true' : undefined"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
      class="atom-field atom-field-multiline"
      :class="toneClass"
      v-bind="attrs"
    />
    <AppLoading v-if="loading" size="md" class="atom-field-spinner atom-field-spinner-top" />
    <p v-if="error || hint" :id="describedBy" class="atom-field-description">
      {{ error || hint }}
    </p>
  </div>
</template>
