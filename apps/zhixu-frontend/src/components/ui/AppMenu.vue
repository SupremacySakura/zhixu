<script setup lang="ts">
/**
 * 菜单零件：从触发器翻开的一叠选项。
 *
 * 为什么自己写键盘处理而不是用原生 <select>：全仓 <select> 实测 0 处，
 * 筛选菜单早已取代它；而且世界里的菜单要能放 RouterLink、分隔线、危险项，
 * 原生 select 装不下。所以这里按 WAI-ARIA 的 menu 模式自己实现。
 *
 * 为什么菜单项用 role="menuitem" 而不是 menuitemradio：本组件的选项是
 * 「做一件事」，不是「选一个值」。需要单选语义的调用方传 checkedKey，
 * 那时才切成 menuitemradio —— 两种语义不能混在同一层。
 *
 * 为什么 Teleport 到 body：菜单常挂在 overflow:hidden 的容器或
 * transform 的祖先里，留在原地会被裁掉。挂到 body 后按触发器的
 * getBoundingClientRect 定位。
 */
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import AppIcon from './AppIcon.vue'

interface MenuItem {
  key: string
  label: string
  icon?: string
  /** 危险项：字用印章红，且默认落到菜单末尾。 */
  danger?: boolean
  disabled?: boolean
  /** 在这一项之前画一条分隔线：把「常用动作」与「危险动作」隔开。 */
  divider?: boolean
}

const props = withDefaults(
  defineProps<{
    /** 触发器文字。给了 items 之外的入口；纯图标触发器用 triggerLabel。 */
    label?: string
    /** 触发器只有图标时的可访问名称。 */
    triggerLabel?: string
    triggerIcon?: string
    items: readonly MenuItem[]
    /** 单选语义：给了就把菜单项切成 menuitemradio，并标出选中项。 */
    checkedKey?: string
    align?: 'start' | 'end'
    disabled?: boolean
    /** 受控展开（不传则内部自管）。 */
    modelValue?: boolean
  }>(),
  {
    label: '',
    triggerLabel: '更多操作',
    triggerIcon: 'material-symbols:more-horiz',
    checkedKey: '',
    align: 'end',
    disabled: false,
    modelValue: undefined,
  },
)

const emit = defineEmits<{
  select: [key: string]
  'update:modelValue': [value: boolean]
}>()

const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const open = ref(false)
const id = useId()
const position = ref({ top: 0, left: 0 })

/** 菜单项角色：单选语义用 menuitemradio，动作语义用 menuitem。 */
const itemRole = computed(() => (props.checkedKey ? 'menuitemradio' : 'menuitem'))

function entries(): HTMLElement[] {
  return Array.from(menu.value?.querySelectorAll<HTMLElement>('[role^="menuitem"]') || []).filter(
    entry => entry.getAttribute('aria-disabled') !== 'true',
  )
}

/** 按触发器的实际位置摆放；左右按 align 贴齐，纵向优先向下，空间不够才翻上去。 */
function place() {
  const rect = trigger.value?.getBoundingClientRect()
  if (!rect) return
  const height = menu.value?.offsetHeight || props.items.length * 32 + 8
  const width = menu.value?.offsetWidth || 192
  const below = window.innerHeight - rect.bottom
  const top = below < height + 12 && rect.top > below ? rect.top - height - 6 : rect.bottom + 6
  const left = props.align === 'end' ? rect.right - width : rect.left
  position.value = {
    top: Math.max(8, Math.min(top, window.innerHeight - height - 8)),
    left: Math.max(8, Math.min(left, window.innerWidth - width - 8)),
  }
}

async function show(focusLast = false) {
  place()
  open.value = true
  emit('update:modelValue', true)
  await nextTick()
  place()
  const options = entries()
  options[focusLast ? options.length - 1 : 0]?.focus({ preventScroll: true })
}

function close(restoreFocus = false) {
  open.value = false
  emit('update:modelValue', false)
  if (restoreFocus) trigger.value?.focus({ preventScroll: true })
}

function toggle() {
  if (open.value) close(true)
  else void show()
}

function select(item: MenuItem) {
  if (item.disabled) return
  close(true)
  emit('select', item.key)
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
  event.preventDefault()
  void show(event.key === 'ArrowUp')
}

function onMenuKeydown(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    close(true)
    return
  }
  if (event.key === 'Tab') {
    close()
    return
  }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  // 只在焦点确实落在菜单里时才接管方向键，否则会把页面别处的方向键吞掉。
  if (!(event.target instanceof Node) || !menu.value?.contains(event.target)) return
  event.preventDefault()
  const options = entries()
  const index = options.indexOf(document.activeElement as HTMLElement)
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? options.length - 1
        : (index + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length
  options[next]?.focus({ preventScroll: true })
}

/** 点外面关闭：pointerdown 覆盖鼠标与触摸，click 覆盖程序化触发。 */
function outside(event: Event) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) close()
}

function onViewportChange() {
  if (open.value) close()
}

function bind() {
  document.addEventListener('pointerdown', outside)
  document.addEventListener('click', outside)
  document.addEventListener('keydown', onMenuKeydown)
  window.addEventListener('resize', onViewportChange)
  window.addEventListener('scroll', onViewportChange, true)
}

function unbind() {
  document.removeEventListener('pointerdown', outside)
  document.removeEventListener('click', outside)
  document.removeEventListener('keydown', onMenuKeydown)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
}

watch(
  open,
  value => {
    unbind()
    if (value) bind()
  },
  { flush: 'sync' },
)

/** 受控模式下由外部驱动开合。 */
watch(
  () => props.modelValue,
  value => {
    if (value === undefined || value === open.value) return
    if (value) void show()
    else close()
  },
)

onBeforeUnmount(unbind)
</script>

<template>
  <div ref="root" class="atom-menu-root" @keydown="onTriggerKeydown">
    <button
      ref="trigger"
      type="button"
      class="atom-menu-trigger"
      :aria-label="label ? undefined : triggerLabel"
      :title="label ? undefined : triggerLabel"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-controls="id"
      :disabled="disabled || undefined"
      @click="toggle"
    >
      <slot name="trigger">
        <AppIcon :icon="triggerIcon" class="atom-glyph" aria-hidden="true" />
        <span v-if="label" class="truncate">{{ label }}</span>
      </slot>
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        :id="id"
        ref="menu"
        role="menu"
        :aria-label="label || triggerLabel"
        class="atom-menu-surface"
        :style="{ top: `${position.top}px`, left: `${position.left}px` }"
      >
        <slot name="menu-header" />
        <template v-for="item in items" :key="item.key">
          <div v-if="item.divider" class="atom-menu-separator" role="separator" />
          <button
            :key="item.key"
            type="button"
            :role="itemRole"
            :aria-checked="checkedKey ? checkedKey === item.key : undefined"
            :aria-disabled="item.disabled ? 'true' : undefined"
            :disabled="item.disabled || undefined"
            tabindex="-1"
            class="atom-menu-item"
            :class="{ 'atom-menu-item-danger': item.danger }"
            @click="select(item)"
          >
            <AppIcon v-if="item.icon" :icon="item.icon" class="atom-glyph" aria-hidden="true" />
            <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
            <AppIcon
              v-if="checkedKey && checkedKey === item.key"
              icon="material-symbols:check-rounded"
              class="atom-glyph"
              aria-hidden="true"
            />
          </button>
        </template>
        <slot name="menu-footer" />
      </div>
    </Teleport>
  </div>
</template>
