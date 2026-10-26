import request from '../request'
import type { ResponseData } from '../types'

export type ServiceId = 'mastra' | 'qdrant'
export type ServiceHealth =
  | 'healthy'
  | 'unconfigured'
  | 'unauthorized'
  | 'unreachable'
  | 'timeout'
  | 'unhealthy'

export interface ServiceStatus {
  id: ServiceId
  status: ServiceHealth
  latency_ms: number | null
  message: string
}

export interface ServiceStatusReport {
  checked_at: string
  services: ServiceStatus[]
}

function isServiceStatusReport(value: unknown): value is ServiceStatusReport {
  if (!value || typeof value !== 'object') return false
  const report = value as Partial<ServiceStatusReport>
  const statuses: ServiceHealth[] = [
    'healthy',
    'unconfigured',
    'unauthorized',
    'unreachable',
    'timeout',
    'unhealthy',
  ]
  return (
    typeof report.checked_at === 'string' &&
    !Number.isNaN(Date.parse(report.checked_at)) &&
    Array.isArray(report.services) &&
    report.services.length === 2 &&
    new Set(report.services.map(service => service?.id)).size === 2 &&
    report.services.every(
      service =>
        service &&
        (service.id === 'mastra' || service.id === 'qdrant') &&
        statuses.includes(service.status) &&
        typeof service.message === 'string' &&
        (service.latency_ms === null ||
          (typeof service.latency_ms === 'number' &&
            Number.isFinite(service.latency_ms) &&
            service.latency_ms >= 0)),
    )
  )
}

export async function getServiceStatus(signal?: AbortSignal): Promise<ServiceStatusReport> {
  const { data } = await request.get<ResponseData<ServiceStatusReport>>(
    '/api/settings/services/status',
    { signal },
  )
  if (data.code !== 0 || data.data == null)
    throw new Error(data.msg || '服务状态检测失败，请稍后重试')
  if (!isServiceStatusReport(data.data)) throw new Error('服务状态响应格式异常，请稍后重试')
  return data.data
}
