<script setup lang="ts">
import { computed, ref } from 'vue'
import AppIcon from '../ui/AppIcon.vue'

const props = defineProps<{
  page: number
  totalPages: number
  startIndex: number
  endIndex: number
  totalCount: number
  loading: boolean
}>()

const emit = defineEmits<{
  change: [page: number]
}>()

const jumpToPageInput = ref('')
const safeTotalPages = computed(() => Math.max(1, props.totalPages))
const pageNumbers = computed(() => {
  const total = safeTotalPages.value
  if (total <= 3) return Array.from({ length: total }, (_, index) => index + 1)
  if (props.page <= 1) return [1, 2, 3]
  if (props.page >= total) return [total - 2, total - 1, total]
  return [props.page - 1, props.page, props.page + 1]
})

const requestPage = (page: number) => {
  const target = Math.min(Math.max(1, page), safeTotalPages.value)
  if (target !== props.page) emit('change', target)
}

const submitJump = () => {
  const target = Number.parseInt(jumpToPageInput.value, 10)
  if (!Number.isFinite(target) || target < 1 || target > safeTotalPages.value) return
  jumpToPageInput.value = ''
  requestPage(target)
}

const pageButtonClass =
  'min-w-8 rounded-full text-[13px] font-semibold tracking-[0.01em] transition-[transform,background-color,color] duration-100'
</script>

<template>
  <div
    class="flex flex-col gap-3 border-t border-hairline bg-surface px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between sm:px-6"
  >
    <p class="text-caption text-label-secondary">
      第 <span class="font-bold text-label">{{ startIndex }}-{{ endIndex }}</span> 项，共
      <span class="font-bold text-label">{{ totalCount }}</span> 项
    </p>
    <div class="flex flex-wrap items-center gap-1">
      <button
        :class="[
          pageButtonClass,
          'flex h-8 items-center justify-center px-1.5 text-label-secondary hover:bg-surface-secondary hover:text-label active:scale-[0.94] disabled:cursor-default disabled:opacity-55 disabled:hover:bg-transparent',
          loading && 'pointer-events-none opacity-45',
        ]"
        type="button"
        aria-label="上一页"
        :disabled="page <= 1 || loading"
        @click="requestPage(page - 1)"
      >
        <AppIcon class="text-[16px]" icon="material-symbols:chevron-left" />
      </button>
      <button
        v-for="pageNumber in pageNumbers"
        :key="pageNumber"
        :class="[
          pageButtonClass,
          'h-8 px-1.5',
          pageNumber === page
            ? 'cursor-default bg-primary-tint font-bold text-primary'
            : 'text-label-secondary hover:bg-surface-secondary hover:text-label active:scale-[0.94] disabled:cursor-default disabled:opacity-45 disabled:hover:bg-transparent',
          loading && pageNumber !== page && 'pointer-events-none opacity-45',
        ]"
        type="button"
        :aria-label="`第 ${pageNumber} 页`"
        :aria-current="pageNumber === page ? 'page' : undefined"
        :disabled="loading && pageNumber !== page"
        @click="requestPage(pageNumber)"
      >
        {{ pageNumber }}
      </button>
      <span v-if="safeTotalPages > 3" class="px-1.5 text-[13px] text-label-tertiary">…</span>
      <button
        :class="[
          pageButtonClass,
          'flex h-8 items-center justify-center px-1.5 text-label-secondary hover:bg-surface-secondary hover:text-label active:scale-[0.94] disabled:cursor-default disabled:opacity-55 disabled:hover:bg-transparent',
          loading && 'pointer-events-none opacity-45',
        ]"
        type="button"
        aria-label="下一页"
        :disabled="page >= safeTotalPages || loading"
        @click="requestPage(page + 1)"
      >
        <AppIcon class="text-[16px]" icon="material-symbols:chevron-right" />
      </button>
      <div v-if="safeTotalPages > 1" class="ml-2.5 flex items-center gap-1.5">
        <span class="text-caption text-label-tertiary">跳至</span>
        <input
          v-model="jumpToPageInput"
          type="number"
          :min="1"
          :max="safeTotalPages"
          :disabled="loading"
          aria-label="跳转页码"
          placeholder="页码"
          class="h-8 w-14 rounded-sm border border-hairline bg-surface-secondary px-2 text-center text-[13px] text-label transition-[border-color,background-color,box-shadow] duration-150 placeholder:text-label-tertiary focus:border-primary focus:bg-surface focus:outline-none focus:ring-[3px] focus:ring-primary-tint disabled:opacity-45"
          @keyup.enter="submitJump"
        />
        <span class="text-caption text-label-tertiary">页</span>
      </div>
    </div>
  </div>
</template>
