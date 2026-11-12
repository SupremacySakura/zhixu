import request from '../request'
import type { ResponseData } from '../types'

/** 提醒类型：重复、过时、冲突，三者共用一张表与一套裁决流程。 */
export type EvolutionAlertType = 'duplicate' | 'outdated' | 'conflict'
/** 提醒状态：待处理 / 已处理 / 已忽略 / 暂不处理。待处理与暂不处理同属待办视野。 */
export type EvolutionAlertStatus = 'pending' | 'resolved' | 'dismissed' | 'deferred'
/** 提醒严重度，只做徽标展示，不参与排序。 */
export type EvolutionSeverity = 'review' | 'strong'
/** 裁决动作；defer 是状态迁移而非裁决动作，不写裁决时间。 */
export type EvolutionAction =
  | 'accept_revision'
  | 'keep_both'
  | 'archive_one'
  | 'create_relation'
  | 'mark_outdated'
  | 'mark_verified'
  | 'dismiss'
  | 'defer'

/** 证据引文：某条知识正文里的原话，由后端校验过可定位性。 */
export interface EvolutionEvidence {
  knowledge_id: number
  quote: string
}

/** 提醒的结构化判定结果；冲突场景额外带修订稿，缺失时前端把「采纳修订」置灰。 */
export interface EvolutionPayload {
  type: EvolutionAlertType
  reason: string
  evidence: EvolutionEvidence[]
  suggested_action: EvolutionAction
  revised_title?: string
  revised_content?: string
}

/** 提醒涉及的知识；subjects[0] 是本次触发检查的知识，subjects[1] 是被比对的知识。 */
export interface EvolutionSubject {
  id: number
  title: string
  source_type: string
  archived: boolean
  outdated: boolean
}

export interface EvolutionAlert {
  id: number
  alert_type: EvolutionAlertType
  status: EvolutionAlertStatus
  severity: EvolutionSeverity
  subjects: EvolutionSubject[]
  payload: EvolutionPayload
  resolved_at: string | null
  resolved_action: string
  resolution_note: string
  created_at: string
  updated_at: string
}

export interface EvolutionAlertPage {
  items: EvolutionAlert[]
  total: number
  page: number
  page_size: number
}

/** 首页卡片用的计数：pending 与 deferred 分开，by_type 三类都含暂不处理。 */
export interface EvolutionSummary {
  pending: number
  deferred: number
  by_type: { duplicate: number; outdated: number; conflict: number }
}

export interface EvolutionAlertQuery {
  page?: number
  pageSize?: number
  type?: EvolutionAlertType | ''
  status?: EvolutionAlertStatus | ''
  /** 只看待办视野（待处理 + 暂不处理）；后端优先于 status。 */
  open?: boolean
}

export interface EvolutionResolveInput {
  action: EvolutionAction
  note?: string
  /** 乐观校验：提交时提醒所处的状态，已被改动时后端返回 409。 */
  baseStatus?: EvolutionAlertStatus
}

function unwrap<T>(data: ResponseData<T>): T {
  if (data.code !== 0 || data.data == null) throw new Error(data.msg || '演化提醒请求失败')
  return data.data
}

/** 分页查询提醒；空字符串筛选不进入查询串，避免后端把它当成非法枚举值。 */
export async function listEvolutionAlerts(query: EvolutionAlertQuery = {}, signal?: AbortSignal) {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.pageSize ?? 20,
  }
  if (query.type) params.type = query.type
  if (query.status) params.status = query.status
  if (query.open) params.open = 'true'
  return unwrap(
    (
      await request.get<ResponseData<EvolutionAlertPage>>('/api/evolution/alerts', {
        params,
        signal,
      })
    ).data,
  )
}

/** 待处理计数；首页卡片只调这一个接口，不拉列表。 */
export async function getEvolutionSummary(signal?: AbortSignal) {
  return unwrap(
    (await request.get<ResponseData<EvolutionSummary>>('/api/evolution/alerts/summary', { signal }))
      .data,
  )
}

/** 提交一次裁决；已处理的提醒再提交按幂等返回当前提醒。 */
export async function resolveEvolutionAlert(
  id: number,
  input: EvolutionResolveInput,
  signal?: AbortSignal,
) {
  const body: { action: EvolutionAction; note?: string; base_status?: EvolutionAlertStatus } = {
    action: input.action,
  }
  const note = input.note?.trim()
  if (note) body.note = note
  if (input.baseStatus) body.base_status = input.baseStatus
  return unwrap(
    (
      await request.post<ResponseData<EvolutionAlert>>(
        `/api/evolution/alerts/${id}/resolve`,
        body,
        { signal },
      )
    ).data,
  )
}
