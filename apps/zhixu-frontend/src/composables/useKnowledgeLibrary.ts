import { onScopeDispose, ref, watch } from 'vue'
import {
  listKnowledge,
  type KnowledgeItem,
  type KnowledgeStatus,
  type KnowledgeVectorState,
} from '../services/apis/knowledge'
import { isKnowledgeActive, isKnowledgeVectorActive, knowledgeError } from '../utils/knowledge'

export function useKnowledgeLibrary() {
  const items = ref<KnowledgeItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const search = ref('')
  const status = ref<KnowledgeStatus | ''>('')
  const vector = ref<KnowledgeVectorState>('')
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null
  let debounce: ReturnType<typeof setTimeout> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let query = ''
  async function load(background = false) {
    controller?.abort()
    clearTimeout(timer)
    const current = new AbortController()
    controller = current
    if (!background) loading.value = true
    error.value = ''
    try {
      const result = await listKnowledge(
        page.value,
        query,
        status.value,
        vector.value,
        current.signal,
      )
      if (current.signal.aborted) return
      const lastPage = Math.max(1, Math.ceil(result.total / 20))
      if (page.value > lastPage) {
        page.value = lastPage
        await load()
        return
      }
      items.value = result.items
      total.value = result.total
      if (
        items.value.some(
          item => isKnowledgeActive(item.status) || isKnowledgeVectorActive(item.vector_state),
        ) ||
        status.value === 'pending' ||
        status.value === 'running' ||
        vector.value === 'pending' ||
        vector.value === 'running'
      )
        timer = setTimeout(() => load(true), 3000)
    } catch (e) {
      if (!current.signal.aborted) error.value = knowledgeError(e)
    } finally {
      if (controller === current) loading.value = false
    }
  }
  watch(search, () => {
    controller?.abort()
    clearTimeout(debounce)
    clearTimeout(timer)
    debounce = setTimeout(() => {
      query = search.value.trim()
      page.value = 1
      void load()
    }, 300)
  })
  watch(status, () => {
    page.value = 1
    void load()
  })
  watch(vector, () => {
    page.value = 1
    void load()
  })
  function changePage(value: number) {
    page.value = value
    void load()
  }
  onScopeDispose(() => {
    controller?.abort()
    clearTimeout(debounce)
    clearTimeout(timer)
  })
  return { items, total, page, search, status, vector, loading, error, load, changePage }
}
