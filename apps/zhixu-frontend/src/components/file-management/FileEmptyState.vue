<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../ui/AppIcon.vue'

const props = withDefaults(
  defineProps<{
    query?: string
  }>(),
  {
    query: '',
  },
)

const emit = defineEmits<{
  upload: []
}>()

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
  <div class="flex flex-col items-center justify-center text-center">
    <!-- 双层圆环插画（FileEmptyState.html 规格） -->
    <div class="relative mb-5">
      <div class="flex h-28 w-28 items-center justify-center bg-primary-tint">
        <div class="flex h-20 w-20 items-center justify-center bg-primary/20 text-primary">
          <AppIcon
            class="atom-glyph"
            :icon="illustrationIcon"
            :style="{ '--atom-glyph-size': '34px' }"
          />
        </div>
      </div>
      <div class="absolute right-2 top-1.5 h-3 w-3 bg-primary/20"></div>
      <div class="absolute bottom-3 left-0 h-2 w-2 bg-primary/15"></div>
    </div>
    <p class="text-title-3 mb-1">{{ title }}</p>
    <p class="text-subhead text-label-secondary mb-5">{{ description }}</p>
    <button
      v-if="!hasSearchQuery"
      class="flex h-10 items-center gap-1.5 px-5 text-sm font-semibold transition-colors knowledge-button-primary"
      type="button"
      @click="emit('upload')"
    >
      <AppIcon
        class="atom-glyph"
        icon="material-symbols:upload"
        :style="{ '--atom-glyph-size': '16px' }"
      />
      上传文件
    </button>
  </div>
</template>
