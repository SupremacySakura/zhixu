<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, useId } from 'vue'
import { Icon } from '@iconify/vue'
import { AnimatePresence, Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
const props = withDefaults(
  defineProps<{
    modelValue: string
    options: readonly { key: string; label: string }[]
    label: string
    /** 表单式外观：占满整行、带边框，用于设置面板与内嵌表单。 */
    field?: boolean
    /** 紧凑高度（36px），用于详情页侧栏与列表行内的选择器。 */
    compact?: boolean
    /** 是否显示左侧的筛选图标；紧凑选择器与列表行里通常不需要。 */
    icon?: boolean
    /** 禁用触发器（请求进行中时阻止再次选择）。 */
    disabled?: boolean
  }>(),
  // 布尔 prop 缺省会被 Vue 转成 false，这里显式恢复“默认带筛选图标”的历史外观。
  { icon: true },
)
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const open = ref(false)
const id = useId()
const selectedLabel = computed(
  () => props.options.find(option => option.key === props.modelValue)?.label || props.label,
)
function outside(event: MouseEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) open.value = false
}
function select(value: string) {
  emit('update:modelValue', value)
  open.value = false
  trigger.value?.focus()
}
async function toggle() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    root.value
      ?.querySelector<HTMLButtonElement>('[role="menuitemradio"][aria-checked="true"]')
      ?.focus()
    if (document.activeElement === trigger.value)
      root.value?.querySelector<HTMLButtonElement>('[role="menuitemradio"]')?.focus()
  }
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (!open.value) return
    event.preventDefault()
    event.stopPropagation()
    open.value = false
    trigger.value?.focus()
  }
  if (event.key === 'Tab') open.value = false
  if (!open.value || !['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const buttons = Array.from(
    root.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') || [],
  )
  const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? buttons.length - 1
        : (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
  buttons[next]?.focus()
}
onMounted(() => document.addEventListener('click', outside))
onBeforeUnmount(() => document.removeEventListener('click', outside))
</script>
<template>
  <div ref="root" class="relative" @keydown="keydown">
    <button
      ref="trigger"
      type="button"
      :aria-label="label"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-controls="id"
      :disabled="disabled"
      class="flex items-center gap-1.5 font-semibold text-label-secondary transition-all hover:text-label active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-60"
      :class="[
        field
          ? 'w-full justify-between rounded-sm border border-hairline bg-surface-secondary text-left'
          : 'rounded-full bg-surface-tertiary',
        compact ? 'min-h-9 px-2.5 text-caption' : 'min-h-11 px-[13px] text-[13px]',
      ]"
      @click="toggle"
    >
      <Icon v-if="icon && !field" icon="material-symbols:filter-list" /><span
        class="min-w-0 break-all"
        >{{ selectedLabel }}</span
      ><Icon icon="material-symbols:expand-more" class="shrink-0" />
    </button>
    <AnimatePresence>
      <Motion
        v-if="open"
        :id="id"
        role="menu"
        :aria-label="label"
        :initial="{ opacity: 0, scale: 0.95 }"
        :animate="{ opacity: 1, scale: 1 }"
        :exit="{ opacity: 0, scale: 0.95 }"
        :transition="springSnappy"
        class="absolute left-0 top-full z-40 mt-1.5 max-h-60 min-w-[180px] max-w-[calc(100vw-3rem)] origin-top-left overflow-y-auto rounded-md bg-surface p-1.5 shadow-popover"
        :class="field ? 'w-full' : ''"
      >
        <button
          v-for="option in options"
          :key="option.key"
          type="button"
          role="menuitemradio"
          :aria-checked="modelValue === option.key"
          class="flex min-h-9 w-full items-center justify-between gap-[18px] break-all rounded-sm px-2.5 py-2 text-left text-sm text-label transition-colors hover:bg-surface-secondary"
          @click="select(option.key)"
        >
          {{ option.label
          }}<Icon
            v-if="modelValue === option.key"
            icon="material-symbols:check-rounded"
            class="text-primary"
          />
        </button>
      </Motion>
    </AnimatePresence>
  </div>
</template>
