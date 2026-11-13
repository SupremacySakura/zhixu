<script setup lang="ts">
/**
 * 空态：靠左的一段话 + 一个动作，不是居中的大插画。
 *
 * 迁移前这里是「28px 方块套 20px 方块」的居中插画，加两颗装饰方块，
 * 再配一颗 40px 高的胶囊主按钮。DESIGN.md 的空态条款明确禁止这个组合
 * （居中大图标 + 胶囊按钮），世界里的空态是索引卡上的一行说明。
 *
 * 图标仍保留，但降到与正文同级的 20px、靠左排在文字前，不再当插画用。
 */
import { computed } from 'vue'
import AppButton from '../ui/AppButton.vue'
import AppIcon from '../ui/AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 搜索关键词：非空说明是「搜不到」，空说明是「还没东西」。 */
    query?: string
  }>(),
  { query: '' },
)

const emit = defineEmits<{ upload: [] }>()

const hasSearchQuery = computed(() => props.query.trim().length > 0)

const illustrationIcon = computed(() =>
  hasSearchQuery.value ? 'material-symbols:search-off' : 'material-symbols:cloud-upload-outline',
)

const title = computed(() => (hasSearchQuery.value ? '没有找到匹配的文件' : '还没有文件'))

const description = computed(() =>
  hasSearchQuery.value ? '试试其他关键词' : '点击上传按钮，开始管理你的文件',
)
</script>

<template>
  <div class="flex flex-col items-start gap-3">
    <div class="flex items-center gap-2 text-label-tertiary">
      <AppIcon :icon="illustrationIcon" class="atom-glyph" aria-hidden="true" />
      <p class="text-lead text-label">{{ title }}</p>
    </div>
    <p class="text-note text-label-secondary">{{ description }}</p>
    <AppButton
      v-if="!hasSearchQuery"
      tone="primary"
      icon="material-symbols:upload"
      @click="emit('upload')"
    >
      上传文件
    </AppButton>
  </div>
</template>
