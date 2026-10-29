import request from '../request'
import type { ResponseData } from '../types'

export type ModelKind = 'chat' | 'embedding'
export type ModelProtocol = 'chat' | 'responses'
export interface AIModel {
  id: number
  provider_id: number
  model_id: string
  kind: ModelKind
  dimension: number
}
export interface ProviderPreset {
  provider_id: string
  name: string
  base_url: string
  protocol: ModelProtocol
  auth_type: 'key' | 'none'
}
export interface AIProvider extends ProviderPreset {
  id: number
  catalog_id: string
  credential_configured: boolean
  models: AIModel[]
}
export interface AISettings {
  chat_model_id: number
  embedding_model_id: number
  rag_top_k: number
  rag_min_score: number
  config_version: number
  embedding_version: number
  needs_reindex: boolean
}
export interface AIConfiguration {
  providers: AIProvider[]
  settings: AISettings
}
export interface ProviderPayload extends ProviderPreset {
  catalog_id: string
  models: { model_id: string; kind: ModelKind; dimension: number }[]
  api_key?: string
  clear_api_key?: boolean
  expected_version: number
  confirm_reindex: boolean
}
export interface DefaultsPayload {
  chat_model_id: number
  embedding_model_id: number
  rag_top_k?: number
  rag_min_score?: number
  expected_version: number
  confirm_reindex: boolean
}
function unwrap<T>(response: ResponseData<T>) {
  if (response.code !== 0 || response.data == null)
    throw new Error(response.msg || '模型设置请求失败')
  return response.data
}
export async function getAIConfiguration(signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<AIConfiguration>>('/api/settings/ai', { signal })).data,
  )
}
export async function getAIProviderCatalog(signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<ProviderPreset[]>>('/api/settings/ai/catalog', { signal }))
      .data,
  )
}
export async function saveAIProvider(
  id: number | null,
  payload: ProviderPayload,
  signal?: AbortSignal,
) {
  const url = '/api/settings/ai/providers'
  const response =
    id === null
      ? await request.post<ResponseData<AIConfiguration>>(url, payload, { signal })
      : await request.put<ResponseData<AIConfiguration>>(`${url}/${id}`, payload, { signal })
  return unwrap(response.data)
}
export async function deleteAIProvider(id: number, version: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.delete<ResponseData<AIConfiguration>>(`/api/settings/ai/providers/${id}`, {
        data: { expected_version: version },
        signal,
      })
    ).data,
  )
}
export async function saveAIDefaults(payload: DefaultsPayload, signal?: AbortSignal) {
  return unwrap(
    (
      await request.put<ResponseData<AIConfiguration>>('/api/settings/ai/defaults', payload, {
        signal,
      })
    ).data,
  )
}
