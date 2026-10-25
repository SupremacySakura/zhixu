import request from '../request'
import type { ResponseData } from '../types'

export type KnowledgeStatus = 'pending' | 'running' | 'ready' | 'failed'
export type KnowledgeVectorState = '' | 'pending' | 'running' | 'ready' | 'partial'
export interface KnowledgeItem {
  id: number
  source_file_id: number
  source_file_name: string
  source_folder_id: number
  source_folder_name: string
  source_type: string
  title: string
  content: string
  status: KnowledgeStatus
  error_code: string
  error_message: string
  imported_at: string | null
  created_at: string
  updated_at: string
  vector_state: KnowledgeVectorState
  chunks_total: number
  chunks_ready: number
  chunks_failed: number
  embedding_model: string
  embedding_version: number
  vector_error: string
}
export interface KnowledgePage {
  items: KnowledgeItem[]
  total: number
  page: number
  page_size: number
}
export type ChunkState = 'pending' | 'ready' | 'failed'
export interface KnowledgeChunk {
  id: number
  seq: number
  section: string
  section_level: number
  chars: number
  start_char: number
  end_char: number
  state: ChunkState
  error_message: string
  content: string
  overlap: boolean
  overlap_chars: number
}
export interface KnowledgeChunkPage {
  items: KnowledgeChunk[]
  total: number
}
function unwrap<T>(data: ResponseData<T>): T {
  if (data.code !== 0 || data.data == null) throw new Error(data.msg || '知识库请求失败')
  return data.data
}
export async function deleteKnowledge(id: number, signal?: AbortSignal) {
  const res = await request.delete<ResponseData<null>>(`/api/knowledge/${id}`, { signal })
  if (res.data.code !== 0) throw new Error(res.data.msg || '删除知识失败')
}
export async function addKnowledge(fileId: number, title: string, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeItem>>(
    '/api/knowledge',
    { file_id: fileId, title },
    { signal },
  )
  return unwrap(res.data)
}
export async function getKnowledge(id: number, signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<KnowledgeItem>>(`/api/knowledge/${id}`, { signal })).data,
  )
}
export async function getKnowledgeByFile(fileId: number, signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<KnowledgeItem>>(`/api/knowledge/by-file/${fileId}`, { signal }))
      .data,
  )
}
export async function listKnowledge(
  page = 1,
  q = '',
  status: KnowledgeStatus | '' = '',
  vector: KnowledgeVectorState = '',
  signal?: AbortSignal,
) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgePage>>('/api/knowledge', {
        params: { page, page_size: 20, q, status, vector },
        signal,
      })
    ).data,
  )
}
export async function getKnowledgeChunks(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgeChunkPage>>(`/api/knowledge/${id}/chunks`, {
        signal,
      })
    ).data,
  )
}
export async function retryKnowledge(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeItem>>(`/api/knowledge/${id}/retry`, undefined, {
        signal,
      })
    ).data,
  )
}
