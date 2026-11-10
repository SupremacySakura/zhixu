/** 操作日志动作类型（与后端 model.OperationLogAction 一一对应） */
export type OperationLogAction =
  | 'upload'
  | 'upload_instant'
  | 'mkdir'
  | 'rename'
  | 'move'
  | 'delete'
  | 'download'
  | 'preview'
  | 'share_link_create'
  | 'share_link_delete'
  | 'pickup_code_create'
  | 'pickup_code_delete'
  | 'knowledge_add'
  | 'knowledge_create'
  | 'knowledge_delete'
  | 'knowledge_retry'
  | 'knowledge_vector_retry'

export type OperationLogTargetType = 'file' | 'folder' | 'knowledge'

/**
 * 操作日志条目。
 * target_name / folder_name 为操作发生时的名称快照，
 * 目标后续被改名或删除不影响历史展示。
 */
export interface OperationLogItem {
  id: number
  action: OperationLogAction
  target_type: OperationLogTargetType
  target_id: number
  target_name: string
  /** 所在目录快照，空串表示根目录 */
  folder_name: string
  /** JSON 字符串扩展字段（size / old_name / from / to / packaged / code / source_file / knowledge_removed），由前端按需解析 */
  detail: string
  created_at: string
}

export interface OperationLogListResponse {
  list: OperationLogItem[]
  total: number
}
