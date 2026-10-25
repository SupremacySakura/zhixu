<script setup lang="ts">
import { computed } from 'vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'
import { knowledgeDate } from '../../utils/knowledge'
import KnowledgeVectorBadge from './KnowledgeVectorBadge.vue'

const props = defineProps<{ item: KnowledgeItem }>()
const visible = computed(() => props.item.status === 'ready' && Boolean(props.item.vector_state))
const state = computed(() => props.item.vector_state)
const rows = computed(() => {
  const list: { label: string; value: string }[] = []
  if (state.value === 'running' || state.value === 'partial' || state.value === 'ready') {
    list.push({ label: '分块', value: `${props.item.chunks_total}` })
    list.push({
      label: '已入库',
      value:
        state.value === 'ready'
          ? `全部 ${props.item.chunks_total} 块`
          : `${props.item.chunks_ready}/${props.item.chunks_total} 块`,
    })
  }
  if (state.value === 'partial')
    list.push({ label: '失败', value: `${props.item.chunks_failed} 块` })
  list.push({ label: '嵌入模型', value: props.item.embedding_model || '尚未配置' })
  list.push({ label: '索引版本', value: `v${props.item.embedding_version}` })
  list.push({ label: '触发方式', value: '正文入库后自动' })
  list.push({ label: '更新时间', value: knowledgeDate(props.item.updated_at) })
  return list
})
const message = computed(() => {
  switch (state.value) {
    case 'pending':
      return '正文已入库，向量化任务排队中，将自动开始。'
    case 'running':
      return '正在向量化，按块推进，不使用虚构百分比。'
    case 'partial':
      return (
        props.item.vector_error ||
        `${props.item.chunks_failed} 个分块失败，可在分块结构中查看原因。`
      )
    case 'ready':
      return '全部分块已写入向量库。'
    default:
      return ''
  }
})
const danger = computed(() => state.value === 'partial')
const showChunksLink = computed(() => props.item.chunks_total > 0)
</script>
<template>
  <section v-if="visible" class="rounded-lg border border-hairline bg-surface p-5">
    <div class="mb-5 flex items-center justify-between gap-3">
      <h2 class="text-body font-semibold">向量化</h2>
      <KnowledgeVectorBadge v-if="state" :state="state" />
    </div>
    <dl class="space-y-5">
      <div v-for="row in rows" :key="row.label">
        <dt class="text-caption text-label-secondary">{{ row.label }}</dt>
        <dd class="mt-1.5 break-all text-subhead">{{ row.value }}</dd>
      </div>
    </dl>
    <p
      v-if="message"
      class="mt-5 rounded-sm px-3 py-2.5 text-caption leading-5"
      :class="danger ? 'bg-danger-tint text-danger' : 'bg-surface-secondary text-label-secondary'"
      role="status"
    >
      {{ message }}
    </p>
    <RouterLink
      v-if="showChunksLink"
      :to="`/home/knowledge/${item.id}/chunks`"
      class="knowledge-button-tint mt-4 w-full"
      >查看分块结构</RouterLink
    >
  </section>
</template>
