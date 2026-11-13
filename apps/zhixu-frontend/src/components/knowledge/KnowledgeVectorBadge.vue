<script setup lang="ts">
import type { KnowledgeVectorState } from '../../services/apis/knowledge'
import { knowledgeVectorLabels } from '../../utils/knowledge'
defineProps<{ state: Exclude<KnowledgeVectorState, ''> }>()
const colors = {
  ready: 'bg-primary-tint text-primary-pressed',
  pending: 'bg-surface-secondary text-label-secondary',
  running: 'bg-info-tint text-info',
  partial: 'bg-warning-tint text-warning-strong',
}
</script>
<template>
  <span
    class="inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-1 text-note font-medium"
    :class="colors[state]"
  >
    <span class="h-1.5 w-1.5 bg-current" aria-hidden="true" />{{ knowledgeVectorLabels[state] }}
  </span>
</template>
