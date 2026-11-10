import type { OperationLogItem } from '../services/types/log'
import { sanitizeFileName } from './file'

/**
 * 操作日志的可视化描述（48px 行 feed 使用）。
 * icon 为 Iconify material-symbols 名称；colorClass 只使用 DESIGN.md 定义的色板 token。
 */
export interface OperationLogDescription {
  icon: string
  colorClass: string
  /** 行标题（目标名称快照，目标已删时回退为占位文案） */
  title: string
  /** 副标题前缀，如「上传至」「原名称」 */
  subtitle: string
  /** 副标题中高亮片段（目录名 / 旧名 / 取件码），空串表示无高亮 */
  highlight: string
}

const parseDetail = (raw: string): Record<string, unknown> => {
  if (!raw) return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // 非法 JSON 按无 detail 处理，不阻断主流程
  }
  return {}
}

const detailString = (detail: Record<string, unknown>, key: string): string => {
  const value = detail[key]
  return typeof value === 'string' ? value : ''
}

const detailNumber = (detail: Record<string, unknown>, key: string): number => {
  const value = detail[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

const folderLabel = (folderName: string): string => sanitizeFileName(folderName) || '根目录'

const fallbackTitle = (targetType: OperationLogItem['target_type']): string => {
  if (targetType === 'folder') return '未知文件夹'
  if (targetType === 'knowledge') return '未知知识'
  return '未知文件'
}

const UNKNOWN_ACTION_META = {
  icon: 'material-symbols:help-outline-rounded',
  colorClass: 'bg-surface-tertiary text-label-secondary',
} as const

/** 将操作日志条目映射为展示描述（纯函数，22 个 action 全覆盖） */
export const describeOperationLog = (log: OperationLogItem): OperationLogDescription => {
  const detail = parseDetail(log.detail)
  const title = sanitizeFileName(log.target_name) || fallbackTitle(log.target_type)

  switch (log.action) {
    case 'upload':
      return {
        icon: 'material-symbols:upload-outline-rounded',
        colorClass: 'bg-primary-tint text-primary',
        title,
        subtitle: '上传至',
        highlight: folderLabel(log.folder_name),
      }
    case 'upload_instant':
      return {
        icon: 'material-symbols:bolt-outline-rounded',
        colorClass: 'bg-info-tint text-info',
        title,
        subtitle: '秒传至',
        highlight: folderLabel(log.folder_name),
      }
    case 'mkdir':
      return {
        icon: 'material-symbols:create-new-folder-outline-rounded',
        colorClass: 'bg-primary-tint text-primary',
        title,
        subtitle: '新建文件夹',
        highlight: '',
      }
    case 'rename': {
      const oldName = sanitizeFileName(detailString(detail, 'old_name'))
      return {
        icon: 'material-symbols:drive-file-rename-outline-rounded',
        colorClass: 'bg-warning-tint text-warning',
        title,
        subtitle: oldName ? '重命名自' : '重命名',
        highlight: oldName,
      }
    }
    case 'move': {
      const to = sanitizeFileName(detailString(detail, 'to'))
      return {
        icon: 'material-symbols:drive-file-move-outline-rounded',
        colorClass: 'bg-info-tint text-info',
        title,
        subtitle: '移动至',
        highlight: to || '根目录',
      }
    }
    case 'delete': {
      const knowledgeRemoved = detailNumber(detail, 'knowledge_removed')
      return {
        icon: 'material-symbols:delete-outline-rounded',
        colorClass: 'bg-pink-tint text-pink',
        title,
        subtitle: knowledgeRemoved > 0 ? `已删除，含 ${knowledgeRemoved} 条知识` : '已删除',
        highlight: '',
      }
    }
    case 'download':
      return {
        icon: 'material-symbols:download-outline-rounded',
        colorClass: 'bg-purple-tint text-purple',
        title,
        subtitle: detail.packaged === true ? '打包下载' : '下载',
        highlight: '',
      }
    case 'preview':
      return {
        icon: 'material-symbols:visibility-outline-rounded',
        colorClass: 'bg-surface-tertiary text-label-secondary',
        title,
        subtitle: '预览',
        highlight: '',
      }
    case 'share_link_create':
      return {
        icon: 'material-symbols:link-outline-rounded',
        colorClass: 'bg-info-tint text-info',
        title,
        subtitle: '创建分享链接',
        highlight: '',
      }
    case 'share_link_delete':
      return {
        icon: 'material-symbols:link-off-outline-rounded',
        colorClass: 'bg-pink-tint text-pink',
        title,
        subtitle: '删除分享链接',
        highlight: '',
      }
    case 'pickup_code_create':
      return {
        icon: 'material-symbols:local-shipping-outline-rounded',
        colorClass: 'bg-purple-tint text-purple',
        title,
        subtitle: '创建取件码',
        highlight: detailString(detail, 'code'),
      }
    case 'pickup_code_delete':
      return {
        icon: 'material-symbols:local-shipping-outline-rounded',
        colorClass: 'bg-pink-tint text-pink',
        title,
        subtitle: '删除取件码',
        highlight: detailString(detail, 'code'),
      }
    case 'knowledge_add':
      return {
        icon: 'material-symbols:bookmark-add-outline-rounded',
        colorClass: 'bg-primary-tint text-primary',
        title,
        subtitle: '加入知识库',
        highlight: folderLabel(log.folder_name),
      }
    case 'knowledge_create':
      return {
        icon: 'material-symbols:note-add-outline-rounded',
        colorClass: 'bg-primary-tint text-primary',
        title,
        subtitle: '手动创建知识',
        highlight: '',
      }
    case 'knowledge_update': {
      const version = detailNumber(detail, 'version')
      return {
        icon: 'material-symbols:edit-note-rounded',
        colorClass: 'bg-primary-tint text-primary',
        title,
        subtitle: '编辑知识正文',
        highlight: version > 0 ? `第 ${version} 版` : '',
      }
    }
    case 'knowledge_restore': {
      const restoredFrom = detailNumber(detail, 'restored_from')
      return {
        icon: 'material-symbols:history-rounded',
        colorClass: 'bg-purple-tint text-purple',
        title,
        subtitle: '恢复历史版本',
        highlight: restoredFrom > 0 ? `第 ${restoredFrom} 版` : '',
      }
    }
    case 'knowledge_delete':
      return {
        icon: 'material-symbols:bookmark-remove-outline-rounded',
        colorClass: 'bg-pink-tint text-pink',
        title,
        subtitle: '移出知识库',
        highlight: '',
      }
    case 'knowledge_retry':
      return {
        icon: 'material-symbols:refresh-rounded',
        colorClass: 'bg-warning-tint text-warning',
        title,
        subtitle: '重试导入',
        highlight: '',
      }
    case 'knowledge_vector_retry':
      return {
        icon: 'material-symbols:refresh-rounded',
        colorClass: 'bg-warning-tint text-warning',
        title,
        subtitle: '重试向量化',
        highlight: '',
      }
    case 'knowledge_favorite':
      return {
        icon: 'material-symbols:star-rounded',
        colorClass: 'bg-warning-tint text-warning',
        title,
        subtitle: detail.favorite === false ? '取消收藏' : '收藏知识',
        highlight: '',
      }
    case 'knowledge_archive':
      return {
        icon: 'material-symbols:archive-outline-rounded',
        colorClass: 'bg-surface-tertiary text-label-secondary',
        title,
        subtitle: '归档知识',
        highlight: '',
      }
    case 'knowledge_unarchive':
      return {
        icon: 'material-symbols:unarchive-outline-rounded',
        colorClass: 'bg-info-tint text-info',
        title,
        subtitle: '恢复归档',
        highlight: '',
      }
    default:
      return { ...UNKNOWN_ACTION_META, title, subtitle: '', highlight: '' }
  }
}
