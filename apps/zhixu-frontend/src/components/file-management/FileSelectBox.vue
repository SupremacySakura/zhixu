<script setup lang="ts">
/**
 * 行首选择框：一个复选框 + 世界里的勾。
 *
 * 为什么抽出来：这段 markup 原本在 FileListView（表头 / 「..」行 / 文件行）与
 * FileGridView（卡片）里各抄一遍，共 4 份、每份 20 行，其中 18 行是同一段内联 SVG。
 * 世界只有一个勾，所以它只该有一处真值。
 *
 * 可访问名称由调用方给（`选择 <文件名>` / `选择当前页全部文件`）：这里只管形状，
 * 不管它在哪一行。原生 input 保留 sr-only，键盘与读屏行为因此一字未变。
 */
withDefaults(
  defineProps<{
    /** 可访问名称。必填——没有它读屏只会念「复选框」。 */
    label: string
    checked: boolean
  }>(),
  { checked: false },
)

const emit = defineEmits<{ change: [checked: boolean] }>()
</script>

<template>
  <label class="inline-flex cursor-pointer" @click.stop>
    <input
      type="checkbox"
      class="peer sr-only"
      :aria-label="label"
      :checked="checked"
      @change="emit('change', ($event.target as HTMLInputElement).checked)"
    />
    <span
      class="flex h-[18px] w-[18px] items-center justify-center border-[1.5px] border-label-tertiary bg-surface transition-[background-color,border-color] hover:border-primary peer-checked:border-primary peer-checked:bg-primary peer-focus-visible:ring-2 peer-focus-visible:ring-primary/50"
    >
      <svg
        v-if="checked"
        class="h-[11px] w-[11px]"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="m5 12.5 5 5L19 8"
          stroke="#fff"
          stroke-width="3"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </span>
  </label>
</template>
