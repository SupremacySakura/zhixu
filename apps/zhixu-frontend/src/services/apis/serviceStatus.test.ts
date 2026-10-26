import { afterEach, describe, expect, it, vi } from 'vitest'
import request from '../request'
import { getServiceStatus } from './serviceStatus'

vi.mock('../request', () => ({ default: { get: vi.fn() } }))
afterEach(() => vi.clearAllMocks())

describe('service status API', () => {
  it('uses the authenticated API client, unwraps the report, and forwards cancellation', async () => {
    const report = {
      checked_at: '2026-10-26T08:00:00Z',
      services: [
        { id: 'mastra', status: 'healthy', latency_ms: 0, message: '连接正常' },
        { id: 'qdrant', status: 'unconfigured', latency_ms: null, message: '未配置' },
      ],
    }
    vi.mocked(request.get).mockResolvedValue({ data: { code: 0, msg: 'ok', data: report } })
    const signal = new AbortController().signal
    expect(await getServiceStatus(signal)).toEqual(report)
    expect(request.get).toHaveBeenCalledWith('/api/settings/services/status', { signal })
  })

  it.each([
    { checked_at: 'invalid', services: [] },
    { checked_at: '2026-10-26T08:00:00Z', services: null },
    {
      checked_at: '2026-10-26T08:00:00Z',
      services: [
        { id: 'mastra', status: 'healthy', latency_ms: 0, message: 'ok' },
        { id: 'mastra', status: 'healthy', latency_ms: 0, message: 'ok' },
      ],
    },
    {
      checked_at: '2026-10-26T08:00:00Z',
      services: [
        { id: 'mastra', status: 'unknown', latency_ms: 0, message: 'ok' },
        { id: 'qdrant', status: 'healthy', latency_ms: -1, message: 'ok' },
      ],
    },
  ])('rejects malformed reports rather than displaying service health', async report => {
    vi.mocked(request.get).mockResolvedValueOnce({ data: { code: 0, data: report } })
    await expect(getServiceStatus()).rejects.toThrow('服务状态响应格式异常')
  })

  it('rejects business failures and missing reports', async () => {
    vi.mocked(request.get).mockResolvedValueOnce({ data: { code: 500, msg: '检测暂不可用' } })
    await expect(getServiceStatus()).rejects.toThrow('检测暂不可用')
    vi.mocked(request.get).mockResolvedValueOnce({ data: { code: 0, data: null } })
    await expect(getServiceStatus()).rejects.toThrow('服务状态检测失败')
  })
})
