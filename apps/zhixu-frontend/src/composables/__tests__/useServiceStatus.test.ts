import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, ref } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { getServiceStatus, type ServiceStatusReport } from '../../services/apis/serviceStatus'
import { useServiceStatus } from '../useServiceStatus'

vi.mock('../../services/apis/serviceStatus', () => ({ getServiceStatus: vi.fn() }))
const report = (checked_at = '2026-10-26T08:00:00Z'): ServiceStatusReport => ({
  checked_at,
  services: [
    { id: 'mastra', status: 'healthy', latency_ms: 12, message: '连接正常' },
    { id: 'qdrant', status: 'unconfigured', latency_ms: null, message: '未配置' },
  ],
})
function deferred() {
  let resolve!: (value: ServiceStatusReport) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<ServiceStatusReport>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}
function setup(initial = false) {
  const active = ref(initial)
  const scope = effectScope()
  const state = scope.run(() => useServiceStatus(active))!
  return { active, scope, ...state }
}
beforeEach(() => vi.mocked(getServiceStatus).mockReset())
afterEach(() => vi.useRealTimers())

describe('useServiceStatus', () => {
  it('checks once per entry, supports manual refresh, and never polls', async () => {
    vi.useFakeTimers()
    vi.mocked(getServiceStatus).mockResolvedValue(report())
    const state = setup()
    expect(getServiceStatus).not.toHaveBeenCalled()
    expect(await state.refresh()).toBe(false)
    state.active.value = true
    await flushPromises()
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(120_000)
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
    await state.refresh()
    expect(getServiceStatus).toHaveBeenCalledTimes(2)
    state.active.value = false
    state.active.value = true
    await flushPromises()
    expect(getServiceStatus).toHaveBeenCalledTimes(3)
    state.scope.stop()
  })

  it('keeps the last report and timestamp on failure, then clears stale state after recovery', async () => {
    vi.mocked(getServiceStatus).mockResolvedValue(report())
    const state = setup(true)
    await flushPromises()
    vi.mocked(getServiceStatus).mockRejectedValueOnce(new Error('网络不可用'))
    await state.refresh()
    expect(state.report.value).toEqual(report())
    expect(state.stale.value).toBe(true)
    expect(state.error.value).toBe('网络不可用')
    vi.mocked(getServiceStatus).mockResolvedValueOnce(report('2026-10-26T08:01:00Z'))
    await state.refresh()
    expect(state.report.value?.checked_at).toBe('2026-10-26T08:01:00Z')
    expect(state.stale.value).toBe(false)
    expect(state.error.value).toBe('')
    state.scope.stop()
  })

  it('shows an initial error without inventing a result and prevents duplicate refreshes', async () => {
    const pending = deferred()
    vi.mocked(getServiceStatus).mockReturnValueOnce(pending.promise)
    const state = setup(true)
    expect(state.loading.value).toBe(true)
    expect(await state.refresh()).toBe(false)
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
    pending.reject(new Error('首次检测失败'))
    await flushPromises()
    expect(state.report.value).toBeNull()
    expect(state.stale.value).toBe(false)
    expect(state.error.value).toBe('首次检测失败')
    expect(state.loading.value).toBe(false)
    state.scope.stop()
  })

  it.each(['resolve', 'reject'] as const)(
    'cancels on tab exit and ignores late %s without disturbing a newer request',
    async outcome => {
      const old = deferred()
      const current = deferred()
      vi.mocked(getServiceStatus)
        .mockReturnValueOnce(old.promise)
        .mockReturnValueOnce(current.promise)
      const state = setup(true)
      const signal = vi.mocked(getServiceStatus).mock.calls[0]![0]!
      state.active.value = false
      expect(signal.aborted).toBe(true)
      expect(state.loading.value).toBe(false)
      state.active.value = true
      if (outcome === 'resolve') old.resolve(report('old'))
      else old.reject(new Error('过期请求失败'))
      await flushPromises()
      expect(state.loading.value).toBe(true)
      expect(state.report.value).toBeNull()
      expect(state.error.value).toBe('')
      current.resolve(report())
      await flushPromises()
      expect(state.report.value).toEqual(report())
      expect(state.loading.value).toBe(false)
      state.scope.stop()
    },
  )

  it('aborts on disposal and refuses late writes or further requests', async () => {
    const pending = deferred()
    vi.mocked(getServiceStatus).mockReturnValueOnce(pending.promise)
    const state = setup(true)
    const signal = vi.mocked(getServiceStatus).mock.calls[0]![0]!
    state.scope.stop()
    expect(signal.aborted).toBe(true)
    pending.resolve(report())
    await flushPromises()
    expect(state.report.value).toBeNull()
    expect(state.loading.value).toBe(false)
    expect(await state.refresh()).toBe(false)
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
  })
})
