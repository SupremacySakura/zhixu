<script setup lang="ts">
/**
 * 原型浮动切换条 —— prototype 技能要求：
 * 视觉上明显区别于设计本身（白底黑框、等宽字体、无圆角、硬偏移），
 * 生产环境隐藏，输入框聚焦时不拦截方向键。
 */
const props = defineProps<{
  views: { key: string; label: string }[]
  view: string
  variants: { key: string; label: string; note: string }[]
  variant: string
}>()

const emit = defineEmits<{ 'update:view': [string]; 'update:variant': [string] }>()

const cycle = (dir: 1 | -1) => {
  const index = props.variants.findIndex(v => v.key === props.variant)
  const next = (index + dir + props.variants.length) % props.variants.length
  emit('update:variant', props.variants[next]!.key)
}

const onKey = (event: KeyboardEvent) => {
  if (!event.altKey) return
  const target = event.target as HTMLElement | null
  if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    cycle(1)
  } else if (event.key === 'ArrowLeft') {
    event.preventDefault()
    cycle(-1)
  }
}

window.addEventListener('keydown', onKey)
</script>

<template>
  <div
    class="fixed bottom-4 left-1/2 z-[80] flex -translate-x-1/2 items-stretch border-2 border-[#181818] bg-white font-type text-[11px] text-[#181818] shadow-[4px_4px_0_rgba(0,0,0,0.85)]"
    role="toolbar"
    aria-label="原型变体切换"
  >
    <div class="flex items-stretch border-r-2 border-[#181818]">
      <button
        v-for="v in views"
        :key="v.key"
        type="button"
        class="px-3 py-2 tracking-[0.06em]"
        :class="v.key === view ? 'bg-[#181818] text-white' : 'hover:bg-[#f2f2f2]'"
        @click="emit('update:view', v.key)"
      >
        {{ v.label }}
      </button>
    </div>

    <div class="flex items-stretch">
      <button
        type="button"
        class="px-2 py-2 hover:bg-[#f2f2f2]"
        aria-label="上一个变体"
        @click="cycle(-1)"
      >
        ◀
      </button>
      <select
        class="cursor-pointer border-x-2 border-[#181818] bg-white px-2 py-2 font-type text-[11px]"
        :value="variant"
        @change="emit('update:variant', ($event.target as HTMLSelectElement).value)"
      >
        <option v-for="v in variants" :key="v.key" :value="v.key">{{ v.label }}</option>
      </select>
      <button
        type="button"
        class="px-2 py-2 hover:bg-[#f2f2f2]"
        aria-label="下一个变体"
        @click="cycle(1)"
      >
        ▶
      </button>
    </div>

    <p class="max-w-[240px] border-l-2 border-[#181818] px-3 py-2 leading-4 text-[#444]">
      {{ variants.find(v => v.key === variant)?.note }}
    </p>
    <p class="border-l-2 border-[#181818] px-3 py-2 leading-4 text-[#888]">ALT ←→</p>
  </div>
</template>
