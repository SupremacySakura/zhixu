<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type CSSProperties } from 'vue'
import AppIcon from '../ui/AppIcon.vue'
import { AnimatePresence, Motion } from 'motion-v'
import { springSnappy } from '../../utils/motion'
import type { FileDisplayItem } from '../../types/file'
import FileKnowledgeAction from '../knowledge/FileKnowledgeAction.vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'

const props = withDefaults(
  defineProps<{
    visible: boolean
    target: FileDisplayItem | null
    position: { top: number; left: number } | null
    downloading?: boolean
    deleting?: boolean
  }>(),
  {
    downloading: false,
    deleting: false,
  },
)

const emit = defineEmits<{
  preview: []
  download: []
  rename: []
  move: []
  delete: []
  close: []
  knowledge: [file: FileDisplayItem, item: KnowledgeItem | null]
}>()

const menuRef = ref<{ $el?: HTMLElement } | HTMLElement | null>(null)
const isOpen = computed(() => props.visible && props.target !== null && props.position !== null)
const menuStyle = computed<CSSProperties>(() => ({
  top: `${props.position?.top ?? 0}px`,
  left: `${props.position?.left ?? 0}px`,
}))
const previewDisabled = computed(() => props.target?.type !== 'file')

const handleKeydown = (event: KeyboardEvent) => {
  if (props.visible && event.key === 'Escape') emit('close')
}

const focusMenu = () => {
  const el = (menuRef.value as { $el?: HTMLElement } | null)?.$el ?? (menuRef.value as HTMLElement)
  el?.focus()
}

watch(isOpen, async open => {
  if (!open) return
  await nextTick()
  focusMenu()
})

onMounted(() => document.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', handleKeydown))

const itemClass =
  'flex h-10 w-full items-center gap-2.5 px-3 text-left text-lead font-normal text-label transition-colors hover:bg-surface-secondary disabled:cursor-not-allowed disabled:opacity-50'
</script>

<template>
  <Teleport to="body">
    <!--
      从触发源 scale 0.95→1 + 淡入（transform-origin 锚定按钮，motion-v 弹簧；
      位置与翻转由 useFileActionMenu 计算）。
    -->
    <AnimatePresence>
      <Motion
        v-if="isOpen"
        key="file-action-menu"
        ref="menuRef"
        :initial="{ opacity: 0, scale: 0.95 }"
        :animate="{ opacity: 1, scale: 1 }"
        :exit="{ opacity: 0, scale: 0.95 }"
        :transition="springSnappy"
        class="fixed z-50 w-52 origin-top-right bg-surface p-1.5 shadow-popover outline-none"
        :style="menuStyle"
        role="menu"
        tabindex="-1"
        :aria-label="`${target?.name ?? '文件'} 操作菜单`"
        @click.stop
      >
        <button
          :class="[itemClass, previewDisabled ? 'text-label-tertiary' : '']"
          type="button"
          role="menuitem"
          aria-label="预览文件"
          :disabled="previewDisabled"
          @click="emit('preview')"
        >
          <AppIcon
            class="atom-glyph text-label-secondary"
            icon="material-symbols:visibility"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          预览
        </button>
        <button
          :class="itemClass"
          type="button"
          role="menuitem"
          aria-label="下载文件"
          :disabled="downloading"
          @click="emit('download')"
        >
          <AppIcon
            class="atom-glyph text-label-secondary"
            icon="material-symbols:download"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          下载
        </button>
        <button
          :class="itemClass"
          type="button"
          role="menuitem"
          aria-label="重命名文件"
          @click="emit('rename')"
        >
          <AppIcon
            class="atom-glyph text-label-secondary"
            icon="material-symbols:edit"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          重命名
        </button>
        <button
          :class="itemClass"
          type="button"
          role="menuitem"
          aria-label="移动文件"
          @click="emit('move')"
        >
          <AppIcon
            class="atom-glyph text-label-secondary"
            icon="material-symbols:drive-file-move"
            :style="{ '--atom-glyph-size': '18px' }"
          />
          移动到
        </button>
        <div v-if="target?.type === 'file'" class="border-t border-hairline px-3 py-2">
          <FileKnowledgeAction
            :file="target"
            @action="(file, item) => emit('knowledge', file, item)"
          />
        </div>
        <div class="mx-2.5 my-1 h-px bg-hairline"></div>
        <button
          :class="[itemClass, 'text-danger hover:bg-danger-tint disabled:hover:bg-transparent']"
          type="button"
          role="menuitem"
          aria-label="删除文件"
          :disabled="deleting"
          @click="emit('delete')"
        >
          <AppIcon icon="material-symbols:delete" />
          删除
        </button>
      </Motion>
    </AnimatePresence>
  </Teleport>
</template>
