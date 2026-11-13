<script setup lang="ts">
import type { KnowledgeStatus } from '../../services/apis/knowledge'
import { knowledgeLabels } from '../../utils/knowledge'
defineProps<{ status: KnowledgeStatus }>()
const colors = {
  ready: 'bg-primary-tint text-primary-pressed',
  pending: 'bg-surface-secondary text-label-secondary',
  running: 'bg-info-tint text-info',
  failed: 'bg-warning-tint text-warning-strong',
}
</script>
<template>
  <span
    class="inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-1 text-note font-medium"
    :class="colors[status]"
  >
    <span class="h-1.5 w-1.5 bg-current" aria-hidden="true" />{{ knowledgeLabels[status] }}
  </span>
</template>
