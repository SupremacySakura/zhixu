<script setup lang="ts">
/**
 * 弹窗零件。
 *
 * 为什么用原生 <dialog> + showModal：焦点陷阱、Esc 关闭、惰性化背景
 * 都由浏览器实现，自己写一遍只会漏掉某个浏览器行为。role="dialog" 由
 * <dialog> 元素自动提供（ARIA 映射里 dialog 元素默认就是这个 role），
 * 但为了承接演化中心与图谱测试里那十几处 [role="dialog"] 断言，
 * 这里仍然显式写出来——显式的契约比依赖映射稳。
 *
 * 为什么 DOM 结构是「根节点 → 面板」两层而不是把 role 放在内层：
 * 测试用 wrapper.find('[role="dialog"] button[data-action="…"]') 从
 * 根往下找按钮，role 必须落在能包住全部按钮的那一层。
 *
 * 为什么关闭按钮是「×」而不是图标：世界里弹窗右上角是手写的一笔，
 * 且 × 是纯文本，不依赖图标族，断网也在。
 */
import { onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 四档语气：primary 给「正在处理的关键事务」，danger 给破坏性确认。 */
    tone?: 'primary' | 'secondary' | 'quiet' | 'danger'
    /** 三档宽度（sm 24rem / md 32rem / lg 48rem），不是三档高度。 */
    size?: 'sm' | 'md' | 'lg'
    title?: string
    description?: string
    /** 打开状态；v-model 双向绑定。 */
    modelValue?: boolean
    /** 忙态：挡住 Esc 与遮罩点击关闭，避免请求中途被关掉。 */
    loading?: boolean
    /** 是否显示右上角关闭按钮。 */
    closable?: boolean
    /** 关闭按钮的可访问名称，测试里按 aria-label="关闭" 找它。 */
    closeLabel?: string
  }>(),
  {
    tone: 'secondary',
    size: 'md',
    title: '',
    description: '',
    modelValue: false,
    loading: false,
    closable: true,
    closeLabel: '关闭',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: boolean]; close: [] }>()

const dialog = ref<HTMLDialogElement | null>(null)
const id = useId()

/** 打开前的焦点位置：关闭后要还回去，否则键盘用户会被扔到页面顶部。 */
let previousFocus: HTMLElement | null = null

const SIZE_CLASS = { sm: 'atom-dialog-sm', md: 'atom-dialog-md', lg: 'atom-dialog-lg' } as const
const TONE_CLASS = {
  primary: 'atom-panel-primary',
  secondary: '',
  quiet: 'atom-panel-quiet',
  danger: 'atom-panel-danger',
} as const

function close() {
  if (props.loading) return
  emit('update:modelValue', false)
  emit('close')
}

/** 原生 cancel（Esc）走同一条关闭路径，忙态时直接吞掉。 */
function onCancel(event: Event) {
  event.preventDefault()
  close()
}

/** 打开：记下当前焦点再 showModal；重复调用是安全的（已开就跳过）。 */
function openDialog() {
  if (dialog.value?.open) return
  previousFocus = document.activeElement as HTMLElement | null
  dialog.value?.showModal()
}

function closeDialog() {
  if (!dialog.value?.open) return
  dialog.value.close()
  previousFocus?.focus?.()
  previousFocus = null
}

onMounted(() => {
  if (props.modelValue) openDialog()
})

// 父组件把 modelValue 改回 false 时要真的关上——只读一次 props 的话，
// 外部 setState(false) 之后弹窗会留在屏幕上，这是 v-model 最容易漏的一半。
watch(
  () => props.modelValue,
  value => (value ? openDialog() : closeDialog()),
)

onBeforeUnmount(() => {
  dialog.value?.close()
  previousFocus?.focus?.()
})

defineSlots<{
  default?: () => unknown
  title?: () => unknown
  description?: () => unknown
  footer?: () => unknown
}>()
</script>

<template>
  <!-- 结构契约：[role="dialog"] 包住标题、正文与全部按钮。
       测试用 [role="dialog"] button[data-action] 从这里往下找。 -->
  <dialog
    ref="dialog"
    class="atom-dialog"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="title ? `${id}-title` : undefined"
    :aria-busy="loading ? 'true' : undefined"
    @cancel="onCancel"
    @click.self="close"
  >
    <div class="atom-dialog-panel" :class="[SIZE_CLASS[size], TONE_CLASS[tone]]">
      <header class="atom-dialog-header">
        <div class="min-w-0">
          <h2 v-if="title || $slots.title" :id="`${id}-title`" class="atom-dialog-title">
            <slot name="title">{{ title }}</slot>
          </h2>
          <p v-if="description || $slots.description" class="atom-dialog-description">
            <slot name="description">{{ description }}</slot>
          </p>
        </div>
        <button
          v-if="closable"
          type="button"
          class="atom-dialog-close"
          :aria-label="closeLabel"
          :disabled="loading || undefined"
          @click="close"
        >
          <AppIcon icon="material-symbols:close" class="atom-glyph" aria-hidden="true" />
        </button>
      </header>
      <div class="atom-dialog-body">
        <slot />
      </div>
      <footer v-if="$slots.footer" class="atom-dialog-footer">
        <slot name="footer" />
      </footer>
    </div>
  </dialog>
</template>
