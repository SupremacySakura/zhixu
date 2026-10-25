<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { AnimatePresence, Motion } from 'motion-v'
import type { KnowledgeItem } from '../../services/apis/knowledge'
import { fadeTransition } from '../../utils/motion'

const props = defineProps<{ item: KnowledgeItem }>()
const emit = defineEmits<{ track: [item: KnowledgeItem]; delete: [item: KnowledgeItem] }>()
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const open = ref(false)
const id = useId()
const position = ref({ top: 0, left: 0, above: false })
const itemClass =
  'flex min-h-11 w-full items-center gap-2.5 rounded-sm px-3 py-2.5 text-left text-subhead focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2 active:bg-surface-tertiary'

function entries() {
  return Array.from(menu.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') || [])
}
function close(restoreFocus = false) {
  open.value = false
  if (restoreFocus) trigger.value?.focus({ preventScroll: true })
}
function place() {
  const rect = trigger.value?.getBoundingClientRect()
  if (!rect) return
  const width = Math.min(200, window.innerWidth - 24)
  const height = (menu.value?.offsetHeight || (props.item.chunks_total > 0 ? 155 : 111)) + 2
  const below = window.innerHeight - rect.bottom
  const above = below < height + 20 && rect.top > below
  position.value = {
    top: Math.max(
      12,
      Math.min(above ? rect.top - height - 8 : rect.bottom + 8, window.innerHeight - height - 12),
    ),
    left: Math.max(12, Math.min(rect.right - width, window.innerWidth - width - 12)),
    above,
  }
}
async function show(last = false) {
  place()
  open.value = true
  await nextTick()
  if (!open.value) return
  place()
  const options = entries()
  options[last ? options.length - 1 : 0]?.focus({ preventScroll: true })
}
function toggle() {
  if (open.value) close(true)
  else void show()
}
function onTriggerKeydown(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
  event.preventDefault()
  void show(event.key === 'ArrowUp')
}
function onKeydown(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    close(true)
    return
  }
  if (!(event.target instanceof Node) || !menu.value?.contains(event.target)) return
  if (event.key === 'Tab') {
    close(true)
    return
  }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const options = entries()
  const current = options.indexOf(document.activeElement as HTMLElement)
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? options.length - 1
        : (current + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length
  options[next]?.focus({ preventScroll: true })
}
function outside(event: Event) {
  if (
    event.target instanceof Node &&
    !trigger.value?.contains(event.target) &&
    !menu.value?.contains(event.target)
  )
    close()
}
function viewportChanged(event: Event) {
  if (event.target instanceof Node && menu.value?.contains(event.target)) return
  close(!!menu.value?.contains(document.activeElement))
}
function track() {
  close(true)
  emit('track', props.item)
}
function selectDelete() {
  close(true)
  emit('delete', props.item)
}
function removeListeners() {
  document.removeEventListener('pointerdown', outside)
  document.removeEventListener('click', outside)
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', viewportChanged)
  window.removeEventListener('scroll', viewportChanged, true)
}
watch(
  open,
  value => {
    removeListeners()
    if (!value) return
    document.addEventListener('pointerdown', outside)
    document.addEventListener('click', outside)
    document.addEventListener('keydown', onKeydown)
    window.addEventListener('resize', viewportChanged)
    window.addEventListener('scroll', viewportChanged, true)
  },
  { flush: 'sync' },
)
onBeforeUnmount(removeListeners)
</script>

<template>
  <div class="shrink-0">
    <button
      ref="trigger"
      type="button"
      :aria-label="`更多操作 ${item.title}`"
      title="更多操作"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-controls="id"
      class="knowledge-more-button flex h-11 w-11 items-center justify-center rounded-sm border border-hairline text-label-secondary transition-transform duration-150 active:scale-[0.97]"
      :class="open ? 'bg-surface-secondary text-label' : 'bg-surface'"
      @click="toggle"
      @keydown="onTriggerKeydown"
    >
      <Icon icon="material-symbols:more-horiz" class="text-2xl" aria-hidden="true" />
    </button>
    <Teleport to="body">
      <AnimatePresence>
        <Motion
          v-if="open"
          :id="id"
          :initial="{ opacity: 0 }"
          :animate="{ opacity: 1 }"
          :exit="{ opacity: 0 }"
          :transition="fadeTransition"
          class="fixed z-40 w-[200px] max-w-[calc(100vw-24px)] rounded-lg border border-hairline bg-material-thick shadow-popover backdrop-blur-xl backdrop-saturate-180"
          :style="{
            top: `${position.top}px`,
            left: `${position.left}px`,
            transformOrigin: position.above ? 'bottom right' : 'top right',
          }"
        >
          <div ref="menu" role="menu" :aria-label="`知识操作 ${item.title}`" class="p-1.5">
            <RouterLink
              v-if="item.status === 'ready'"
              :to="`/home/knowledge/${item.id}`"
              role="menuitem"
              tabindex="-1"
              :class="[itemClass, 'knowledge-menu-item text-label']"
              @click="close(true)"
            >
              <Icon
                icon="material-symbols:menu-book-outline"
                class="text-lg"
                aria-hidden="true"
              />查看知识
            </RouterLink>
            <button
              v-else
              type="button"
              role="menuitem"
              tabindex="-1"
              :class="[itemClass, 'knowledge-menu-item text-label']"
              @click="track"
            >
              <Icon icon="material-symbols:info-outline" class="text-lg" aria-hidden="true" />{{
                item.status === 'failed' ? '查看失败原因' : '查看进度'
              }}
            </button>
            <RouterLink
              v-if="item.chunks_total > 0"
              :to="`/home/knowledge/${item.id}/chunks`"
              role="menuitem"
              tabindex="-1"
              :class="[itemClass, 'knowledge-menu-item text-label']"
              @click="close(true)"
            >
              <Icon
                icon="material-symbols:layers-outline"
                class="text-lg"
                aria-hidden="true"
              />分块结构
            </RouterLink>
            <div role="separator" class="mx-1.5 my-[5px] h-px bg-hairline"></div>
            <button
              type="button"
              role="menuitem"
              tabindex="-1"
              :aria-label="`删除知识 ${item.title}`"
              :class="[itemClass, 'knowledge-menu-item knowledge-menu-danger text-danger']"
              @click="selectDelete"
            >
              <Icon
                icon="material-symbols:delete-outline"
                class="text-lg"
                aria-hidden="true"
              />删除知识
            </button>
          </div>
        </Motion>
      </AnimatePresence>
    </Teleport>
  </div>
</template>

<style scoped>
@media (hover: hover) and (pointer: fine) {
  .knowledge-more-button:hover,
  .knowledge-menu-item:hover {
    background: var(--color-surface-secondary);
  }
  .knowledge-menu-danger:hover {
    background: var(--color-danger-tint);
  }
}
</style>
