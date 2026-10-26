<script setup lang="ts">
import { computed } from 'vue'
import type { ServiceHealth, ServiceStatusReport } from '../../services/apis/serviceStatus'

const props = defineProps<{
  report: ServiceStatusReport | null
  loading: boolean
  error: string
  stale: boolean
}>()
defineEmits<{ refresh: [] }>()
const services = [
  { id: 'mastra', name: 'Mastra AI 服务', description: '模型调用与向量化工作流' },
  { id: 'qdrant', name: 'Qdrant 向量库', description: '向量存储与检索，知识正文仍保留在 MySQL' },
] as const
const healthLabels: Record<ServiceHealth, string> = {
  healthy: '运行正常',
  unconfigured: '未配置',
  unauthorized: '认证失败',
  unreachable: '无法连接',
  timeout: '检测超时',
  unhealthy: '服务异常',
}
const rows = computed(() =>
  services.map(service => ({
    ...service,
    result: props.report?.services.find(result => result.id === service.id),
  })),
)
const checkedTime = computed(() => {
  if (!props.report) return ''
  const date = new Date(props.report.checked_at)
  return Number.isNaN(date.getTime()) ? props.report.checked_at : date.toLocaleString('zh-CN')
})
function badgeClass(status: ServiceHealth | undefined) {
  if (status === 'healthy') return 'bg-primary-tint text-primary-pressed'
  if (status === 'unconfigured') return 'bg-warning-tint text-warning-strong'
  if (status) return 'bg-danger-tint text-danger'
  return 'bg-surface-secondary text-label-secondary'
}
</script>

<template>
  <section class="rounded-lg bg-surface p-6 shadow-card" :aria-busy="loading">
    <header class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h2 class="text-title-3">基础服务</h2>
        <p class="mt-2 text-caption text-label-secondary">
          最近检测：<time v-if="report" :datetime="report.checked_at">{{ checkedTime }}</time
          ><span v-else>尚未完成检测</span>
          <span v-if="stale" class="ml-2 text-warning-strong">旧结果</span>
        </p>
      </div>
      <button
        type="button"
        class="knowledge-button-secondary"
        :disabled="loading"
        @click="$emit('refresh')"
      >
        {{ loading ? '正在检测…' : '刷新状态' }}
      </button>
    </header>
    <p class="mb-5 mt-3 text-subhead text-label-secondary">
      这些连接由部署配置管理，不属于模型供应商设置。仅检测基础服务，不调用付费模型。
    </p>
    <p v-if="loading" role="status" class="mb-4 text-subhead text-label-secondary">
      正在检测基础服务…<span v-if="report">下方保留上次结果。</span>
    </p>
    <p
      v-if="error"
      role="alert"
      class="mb-4 rounded-md bg-warning-tint p-4 text-subhead text-warning-strong"
    >
      {{ error }}<span v-if="stale"> 当前显示旧结果，不代表最新服务状态。</span>
    </p>
    <article
      v-for="entry in rows"
      :key="entry.id"
      class="flex flex-wrap items-center justify-between gap-3 border-t border-hairline py-5"
    >
      <div class="min-w-0 flex-1">
        <h3 class="text-headline">{{ entry.name }}</h3>
        <p class="mt-2 text-subhead text-label-secondary">{{ entry.description }}</p>
        <p v-if="entry.result?.message" class="mt-2 break-words text-caption text-label-secondary">
          {{ entry.result.message }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <span
          v-if="entry.result?.latency_ms != null"
          class="text-caption tabular-nums text-label-secondary"
          >{{ entry.result.latency_ms }} ms</span
        >
        <span class="rounded-full px-3 py-1 text-caption" :class="badgeClass(entry.result?.status)">
          {{ entry.result ? healthLabels[entry.result.status] : '尚无检测结果' }}
        </span>
      </div>
    </article>
    <p class="text-caption leading-6 text-label-secondary">
      进入此分类时检测一次，也可手动刷新，不会自动轮询。短时间刷新可能复用最近结果。基础服务正常不代表模型调用已验证，也不代表所有知识已完成向量化；文件管理和知识阅读不依赖这些
      AI 服务。
    </p>
  </section>
</template>
