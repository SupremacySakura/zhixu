import request from '../request'
import type { ResponseData } from '../types'
import type { Tag } from './tag'

export type KnowledgeStatus = 'pending' | 'running' | 'ready' | 'failed'
export type KnowledgeVectorState = '' | 'pending' | 'running' | 'ready' | 'partial'
export interface KnowledgeItem {
  id: number
  source_file_id: number | null
  source_file_name: string
  source_folder_id: number
  source_folder_name: string
  source_type: string
  source_url: string
  title: string
  content: string
  version: number
  /** AI 整理摘要：只有用户在详情页确认采纳后才有值。 */
  summary: string
  /** AI 整理关键词：采纳后写入，列表与检索侧复用。 */
  keywords: string[]
  is_favorite: boolean
  archived_at: string | null
  /** 可能过时的判定时间：只有演化检测判定过时并由用户采纳后才写入。 */
  outdated_at: string | null
  /** 过时判定理由：与 outdated_at 同时出现，详情页直接展示。 */
  outdated_reason: string
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
export interface KnowledgeVersionSummary {
  version_number: number
  title: string
  change_source: string
  created_at: string
  current: boolean
}
export interface KnowledgeVersionDetail extends KnowledgeVersionSummary {
  content: string
}
export interface KnowledgeVersionPage {
  items: KnowledgeVersionSummary[]
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
export async function createKnowledge(title: string, content: string, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeItem>>(
    '/api/knowledge',
    { title, content },
    { signal },
  )
  return unwrap(res.data)
}
export async function importKnowledgeFromWeb(url: string, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeItem>>(
    '/api/knowledge/import-web',
    { url },
    { signal, timeout: 45_000 },
  )
  return unwrap(res.data)
}
export async function updateKnowledge(
  id: number,
  title: string,
  content: string,
  baseVersion: number,
  signal?: AbortSignal,
) {
  const res = await request.put<ResponseData<KnowledgeItem>>(
    `/api/knowledge/${id}`,
    { title, content, base_version: baseVersion },
    { signal },
  )
  return unwrap(res.data)
}
export interface KnowledgeFavoriteState {
  id: number
  is_favorite: boolean
}
export interface KnowledgeArchiveState {
  id: number
  archived: boolean
  archived_at: string | null
}
export async function favoriteKnowledge(id: number, favorite: boolean, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeFavoriteState>>(
    `/api/knowledge/${id}/favorite`,
    { favorite },
    { signal },
  )
  return unwrap(res.data)
}
export async function archiveKnowledge(id: number, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeArchiveState>>(
    `/api/knowledge/${id}/archive`,
    {},
    { signal },
  )
  return unwrap(res.data)
}
export async function unarchiveKnowledge(id: number, signal?: AbortSignal) {
  const res = await request.post<ResponseData<KnowledgeArchiveState>>(
    `/api/knowledge/${id}/unarchive`,
    {},
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
export type KnowledgeSort = 'created' | 'updated' | 'title'
export interface KnowledgeListOptions {
  sourceType?: string
  tagId?: number
  favorite?: boolean
  archived?: boolean
  sort?: KnowledgeSort
}
export async function listKnowledge(
  page = 1,
  q = '',
  status: KnowledgeStatus | '' = '',
  vector: KnowledgeVectorState = '',
  options: KnowledgeListOptions = {},
  signal?: AbortSignal,
) {
  const params: Record<string, string | number | boolean> = {
    page,
    page_size: 20,
    q,
    status,
    vector,
  }
  if (options.sourceType) params.source_type = options.sourceType
  if (options.tagId) params.tag_id = options.tagId
  if (options.favorite !== undefined) params.favorite = options.favorite
  if (options.archived !== undefined) params.archived = options.archived
  if (options.sort) params.sort = options.sort
  return unwrap(
    (
      await request.get<ResponseData<KnowledgePage>>('/api/knowledge', {
        params,
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
/**
 * 侧栏底部计数（索引 N · 分块 M）。
 *
 * 为什么不复用 listKnowledge：它的 page_size 硬编码 20，而计数要覆盖全部知识，
 * 后端 page_size 上限是 100（超过报 400）。改它的签名会波及 3 个调用点与
 * 若干断言实参的测试，所以这里单独发一次请求，只取 total 与各条 chunks_total。
 *
 * 注意：total 是「索引条数」的权威值；chunks 只能由当前页的 chunks_total 累加得出。
 * 知识超过 100 条时分块数会偏低 —— 这是没有统计端点时的已知上限，
 * 比显示假数据好，且侧栏计数只是方位感，不是账目。
 */
export async function getKnowledgeCounts(signal?: AbortSignal) {
  const page = unwrap(
    (
      await request.get<ResponseData<KnowledgePage>>('/api/knowledge', {
        params: { page: 1, page_size: 100 },
        signal,
      })
    ).data,
  )
  const chunks = page.items.reduce((sum, item) => sum + (item.chunks_total || 0), 0)
  return { index: page.total, chunks }
}
export async function listKnowledgeVersions(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgeVersionPage>>(`/api/knowledge/${id}/versions`, {
        signal,
      })
    ).data,
  )
}
export async function getKnowledgeVersion(id: number, version: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgeVersionDetail>>(
        `/api/knowledge/${id}/versions/${version}`,
        { signal },
      )
    ).data,
  )
}
export async function restoreKnowledgeVersion(id: number, version: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeItem>>(
        `/api/knowledge/${id}/versions/${version}/restore`,
        undefined,
        { signal },
      )
    ).data,
  )
}
export type KnowledgeOrganizeStatus = 'pending' | 'running' | 'succeeded' | 'failed'
export interface KnowledgeOrganizeTask {
  id: number
  task_type: string
  status: KnowledgeOrganizeStatus
  error_message: string
  retry_count: number
  updated_at: string
}
export interface KnowledgeOrganizeTag {
  name: string
  /** 非零表示用户已有同名标签，采纳时按 ID 复用。 */
  tag_id: number
  existing: boolean
}
export interface KnowledgeOrganizeDraft {
  summary: string
  keywords: string[]
  suggested_tags: KnowledgeOrganizeTag[]
  concepts: string[]
  accepted: boolean
  accepted_at?: string | null
  /** 草稿生成后正文被编辑过：采纳前应提示重新生成。 */
  stale: boolean
  base_version: number
}
export interface KnowledgeOrganizeState {
  task: KnowledgeOrganizeTask | null
  draft: KnowledgeOrganizeDraft | null
}
export interface KnowledgeOrganizeAcceptInput {
  summary: string
  keywords: string[]
  tag_ids: number[]
  tag_names: string[]
}
export interface KnowledgeOrganizeAcceptResult {
  item: KnowledgeItem
  tags: Tag[]
  draft: KnowledgeOrganizeDraft | null
}
export async function getKnowledgeOrganize(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgeOrganizeState>>(`/api/knowledge/${id}/organize`, {
        signal,
      })
    ).data,
  )
}
export async function acceptKnowledgeOrganize(
  id: number,
  input: KnowledgeOrganizeAcceptInput,
  signal?: AbortSignal,
) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeOrganizeAcceptResult>>(
        `/api/knowledge/${id}/organize/accept`,
        input,
        { signal },
      )
    ).data,
  )
}
export async function regenerateKnowledgeOrganize(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeOrganizeState>>(
        `/api/knowledge/${id}/organize/regenerate`,
        undefined,
        { signal },
      )
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
export async function retryKnowledgeVectorize(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeItem>>(
        `/api/knowledge/${id}/vectorize/retry`,
        undefined,
        { signal },
      )
    ).data,
  )
}

export type RelationType =
  | 'related'
  | 'depends_on'
  | 'supplements'
  | 'contrasts'
  | 'updates'
  /** 矛盾：对称关系，只能由用户手动或冲突提醒建立，AI 关联推荐不会产出。 */
  | 'contradicts'
export type RelationStatus = 'suggested' | 'confirmed' | 'rejected'
export type RelationSource = 'ai' | 'manual'
export interface KnowledgeRelation {
  id: number
  source_id: number
  target_id: number
  relation_type: RelationType
  status: RelationStatus
  source: RelationSource
  confidence: number
  evidence: string
  /** 对端知识：详情页直接展示标题并跳转。 */
  counterpart_id: number
  counterpart_title: string
  created_at: string
  /** 确认时间：确认或手动建立的关系才有值。 */
  confirmed_at: string | null
}
export interface RelationTask {
  id: number
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  error_message: string
  retry_count: number
  updated_at: string
}
export interface KnowledgeRelationList {
  confirmed: KnowledgeRelation[]
  suggested: KnowledgeRelation[]
  /** 当前知识的关联推荐任务；尚未触发过时为 null。 */
  task: RelationTask | null
}
export async function listKnowledgeRelations(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.get<ResponseData<KnowledgeRelationList>>(`/api/knowledge/${id}/relations`, {
        signal,
      })
    ).data,
  )
}
export async function createKnowledgeRelation(
  id: number,
  input: { target_id: number; relation_type: RelationType },
  signal?: AbortSignal,
) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeRelation>>(`/api/knowledge/${id}/relations`, input, {
        signal,
      })
    ).data,
  )
}
export async function deleteKnowledgeRelation(relationId: number, signal?: AbortSignal) {
  const res = await request.delete<ResponseData<null>>(`/api/relations/${relationId}`, { signal })
  if (res.data.code !== 0) throw new Error(res.data.msg || '删除关系失败')
}
export async function confirmKnowledgeRelation(relationId: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeRelation>>(
        `/api/relations/${relationId}/confirm`,
        undefined,
        { signal },
      )
    ).data,
  )
}
export async function rejectKnowledgeRelation(relationId: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<KnowledgeRelation>>(
        `/api/relations/${relationId}/reject`,
        undefined,
        { signal },
      )
    ).data,
  )
}
export async function updateKnowledgeRelationType(
  relationId: number,
  relationType: RelationType,
  signal?: AbortSignal,
) {
  return unwrap(
    (
      await request.put<ResponseData<KnowledgeRelation>>(
        `/api/relations/${relationId}`,
        { relation_type: relationType },
        { signal },
      )
    ).data,
  )
}
/** 手动触发一次关联推荐；活跃任务由后端复用，返回当前任务状态。 */
export async function regenerateKnowledgeRelations(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<{ task: RelationTask }>>(
        `/api/knowledge/${id}/relations/regenerate`,
        undefined,
        { signal },
      )
    ).data,
  )
}

/** 演化检查任务：后端直接返回任务快照，形状与其它 AI 任务一致。 */
export interface EvolveTask {
  id: number
  task_type: string
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  error_message: string
  retry_count: number
  updated_at: string
}
/** 手动重跑一次演化检查；活跃任务由后端复用，返回当前任务状态。 */
export async function regenerateKnowledgeEvolve(id: number, signal?: AbortSignal) {
  return unwrap(
    (
      await request.post<ResponseData<EvolveTask>>(
        `/api/knowledge/${id}/evolve/regenerate`,
        undefined,
        { signal },
      )
    ).data,
  )
}

export interface KnowledgeGraphNode {
  id: number
  title: string
  /** 知识来源类型：manual/web/markdown/text/pdf/docx/csv/xlsx 等。 */
  source_type: string
  summary: string
  /** 该节点在当前子图中的连接度（双向关系计数）。 */
  degree: number
  updated_at: string
}
export interface KnowledgeGraphEdge {
  id: number
  /** 关系方向保留 source_id → target_id。 */
  source_id: number
  target_id: number
  relation_type: RelationType
  status: RelationStatus
  source: RelationSource
  confidence: number
}
export interface KnowledgeGraph {
  nodes: KnowledgeGraphNode[]
  edges: KnowledgeGraphEdge[]
  /** 节点超过上限时按连接度截断。 */
  truncated: boolean
}
export interface KnowledgeGraphQuery {
  centerId?: number
  includeSuggested?: boolean
  /** 只看命中该标签的知识；0 或省略表示全部标签。 */
  tagId?: number
  /** 只看该来源类型的知识：manual/web 或文件导入格式（markdown/pdf 等）；空字符串表示全部来源。 */
  sourceType?: string
}

/** 局部图谱：不带中心取最近更新的 50 条知识，带中心只展开一跳。 */
export async function getKnowledgeGraph(options: KnowledgeGraphQuery = {}, signal?: AbortSignal) {
  const params: Record<string, string | number> = {
    depth: 1,
    include_suggested: options.includeSuggested ? 1 : 0,
  }
  if (options.centerId && options.centerId > 0) params.center_id = options.centerId
  if (options.tagId && options.tagId > 0) params.tag_id = options.tagId
  if (options.sourceType) params.source_type = options.sourceType
  return unwrap(
    (await request.get<ResponseData<KnowledgeGraph>>('/api/knowledge/graph', { params, signal }))
      .data,
  )
}
