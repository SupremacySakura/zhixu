import { onScopeDispose, ref, watch, type Ref } from 'vue'
import axios from 'axios'
import { getServiceStatus, type ServiceStatusReport } from '../services/apis/serviceStatus'

function statusError(error: unknown): string {
  if (axios.isAxiosError<{ msg?: string }>(error))
    return error.response?.data.msg || '服务状态检测失败，请检查网络后重试'
  return error instanceof Error ? error.message : '服务状态检测失败，请稍后重试'
}

export function useServiceStatus(active: Readonly<Ref<boolean>>) {
  const report = ref<ServiceStatusReport | null>(null)
  const loading = ref(false)
  const error = ref('')
  const stale = ref(false)
  let controller: AbortController | null = null
  let disposed = false

  function cancel() {
    const previous = controller
    controller = null
    previous?.abort()
    loading.value = false
  }

  async function refresh(): Promise<boolean> {
    if (disposed || !active.value || loading.value) return false
    const current = new AbortController()
    controller = current
    loading.value = true
    error.value = ''
    const isCurrent = () =>
      !disposed && active.value && controller === current && !current.signal.aborted
    try {
      const result = await getServiceStatus(current.signal)
      if (!isCurrent()) return false
      report.value = result
      stale.value = false
      return true
    } catch (cause) {
      if (isCurrent()) {
        error.value = statusError(cause)
        stale.value = report.value !== null
      }
      return false
    } finally {
      if (isCurrent()) {
        controller = null
        loading.value = false
      }
    }
  }

  watch(
    active,
    value => {
      if (value) void refresh()
      else cancel()
    },
    { immediate: true, flush: 'sync' },
  )
  onScopeDispose(() => {
    disposed = true
    cancel()
  })
  return { report, loading, error, stale, refresh }
}
