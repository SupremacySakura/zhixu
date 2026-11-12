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
      <div class="flex h-28 w-28 items-center justify-center rounded-full bg-primary-tint">
        <div
          class="flex h-20 w-20 items-center justify-center rounded-full bg-primary/20 text-primary"
        >
          <AppIcon class="text-[34px]" :icon="illustrationIcon" />
        </div>
      </div>
      <div class="absolute right-2 top-1.5 h-3 w-3 rounded-full bg-primary/20"></div>
      <div class="absolute bottom-3 left-0 h-2 w-2 rounded-full bg-primary/15"></div>
    </div>
    <p class="text-title-3 mb-1">{{ title }}</p>
    <p class="text-subhead text-label-secondary mb-5">{{ description }}</p>
    <button
      v-if="!hasSearchQuery"
      class="flex h-10 items-center gap-1.5 rounded-full bg-primary px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-150 hover:bg-primary-hover active:scale-[0.97] active:bg-primary-pressed"
      type="button"
      @click="emit('upload')"
    >
      <AppIcon class="text-[16px]" icon="material-symbols:upload" />
      上传文件
    </button>
  </div>
</template>
