<script setup lang="ts">
/**
 * 单行输入零件。
 *
 * 为什么用 defineModel 而不是自己 emit('update:modelValue')：
 * 组件对外就是 v-model，defineModel 让 props 契约和 emit 只写一处，
 * 也不会出现「声明了 modelValue 但忘了 emit」这种半截实现。
 *
 * 为什么加载态不设原生 disabled：和 AppButton 同理——设了会掉焦点，
 * 用户正在输入时请求返回、输入框失焦，光标就没了。这里改用 readonly
 * （阻止改值但保住焦点与选区）+ aria-busy（让辅助技术知道在忙）。
 */
import { computed, useAttrs, useId } from 'vue'
import AppLoading from './AppLoading.vue'

const props = withDefaults(
  defineProps<{
    /** 四档语气；输入框通常只用 secondary，primary 用于「当前正在编辑」。 */
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档尺寸：26 / 32 / 36px。 */
    size?: 'sm' | 'md' | 'lg'
    type?: string
    disabled?: boolean
    loading?: boolean
    /** 可见标签文字；给了就自动与输入框建立 for/id 关联。 */
    label?: string
    /** 标签下方的说明文字。 */
    hint?: string
    /** 校验失败信息：一旦有值，语气强制切到 danger，并标 aria-invalid。 */
    error?: string
    placeholder?: string
  }>(),
  {
    tone: 'secondary',
    size: 'md',
    type: 'text',
    disabled: false,
    loading: false,
    label: '',
    hint: '',
    error: '',
    placeholder: '',
  },
)

/**
 * 关掉属性透传：本组件的根是 `<div class="atom-field-shell">`，而真正要接收
 * `id` / `name` / `autocomplete` 的是里面那层 `<input>`。不关的话 Vue 会把
 * 同一批属性**同时**落到 div 与 input 上，`document.querySelector('#id')`
 * 先命中的是 DIV —— 测试里 `wrapper.get('#xxx').setValue()` 会直接抛
 * （DIV 没有 setValue）。本组件已经在模板里显式 `v-bind="attrs"` 到 input，
 * 所以关掉透传不会丢任何属性。
 */
defineOptions({ inheritAttrs: false })

const model = defineModel<string | number>({ default: '' })
const attrs = useAttrs()
const id = useId()

/** 外部显式给了 id 就用外部的，否则用本组件生成的——两种情况标签都能对上。 */
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

/** 错误优先于语气：有 error 时一定是危险边界。 */
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
    <input
      :id="inputId"
      v-model="model"
      :type="type"
      :placeholder="placeholder"
      :disabled="disabled || undefined"
      :readonly="loading || undefined"
      :aria-busy="loading ? 'true' : undefined"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="describedBy"
      class="atom-field"
      :class="toneClass"
      v-bind="attrs"
    />
    <AppLoading v-if="loading" size="md" class="atom-field-spinner" />
    <p v-if="error || hint" :id="describedBy" class="atom-field-description">
      {{ error || hint }}
    </p>
  </div>
</template>
