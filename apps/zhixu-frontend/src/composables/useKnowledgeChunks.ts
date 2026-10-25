import { onScopeDispose, ref, watch, type Ref } from 'vue'
import { getKnowledgeChunks, type KnowledgeChunk } from '../services/apis/knowledge'
import { knowledgeError } from '../utils/knowledge'

export function useKnowledgeChunks(id: Ref<number>, autoLoad = true) {
  const items = ref<KnowledgeChunk[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null
  async function load() {
    controller?.abort()
    const current = new AbortController()
    controller = current
    loading.value = true
    error.value = ''
    try {
      const result = await getKnowledgeChunks(id.value, current.signal)
      if (current.signal.aborted) return
      items.value = result.items
      total.value = result.total
    } catch (e) {
      if (!current.signal.aborted) error.value = knowledgeError(e)
    } finally {
      if (controller === current) loading.value = false
    }
  }
  watch(
    id,
    () => {
      controller?.abort()
      items.value = []
      total.value = 0
      error.value = ''
      if (autoLoad) void load()
    },
    { immediate: true },
  )
  onScopeDispose(() => controller?.abort())
  return { items, total, loading, error, load }
}
