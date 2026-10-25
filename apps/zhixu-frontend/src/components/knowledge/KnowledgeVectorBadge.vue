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
    class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-1 text-caption font-medium"
    :class="colors[state]"
  >
    <span class="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />{{
      knowledgeVectorLabels[state]
    }}
  </span>
</template>
