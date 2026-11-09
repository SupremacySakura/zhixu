import axios from 'axios'
import type {
  KnowledgeItem,
  KnowledgeStatus,
  KnowledgeVectorState,
} from '../services/apis/knowledge'

export const knowledgeLabels: Record<KnowledgeStatus, string> = {
  pending: '排队中',
  running: '处理中',
  ready: '已加入',
  failed: '加入失败',
}
export const knowledgeVectorLabels: Record<Exclude<KnowledgeVectorState, ''>, string> = {
  pending: '待向量化',
  running: '向量化中',
  ready: '已向量化',
  partial: '部分失败',
}
export interface KnowledgeFormat {
  /** 与后端 knowledge_items.source_type 对齐 */
  type: string
  /** 展示名 */
  label: string
  /** 小写、含点的扩展名列表 */
  extensions: string[]
  /** 源文件大小上限（字节），与后端提取器一致 */
  maxBytes: number
}
// 可知识化格式能力表：新增格式在此登记，门控、大小限制与标题剥离全部由此驱动。
export const KNOWLEDGE_FORMATS: KnowledgeFormat[] = [
  {
    type: 'markdown',
    label: 'Markdown',
    extensions: ['.md', '.markdown'],
    maxBytes: 20 * 1024 * 1024,
  },
  {
    type: 'text',
    label: '文本',
    extensions: ['.txt'],
    maxBytes: 20 * 1024 * 1024,
  },
  {
    type: 'pdf',
    label: 'PDF',
    extensions: ['.pdf'],
    maxBytes: 20 * 1024 * 1024,
  },
  {
    type: 'docx',
    label: 'Word',
    extensions: ['.docx'],
    maxBytes: 20 * 1024 * 1024,
  },
  {
    type: 'csv',
    label: 'CSV',
    extensions: ['.csv'],
    maxBytes: 20 * 1024 * 1024,
  },
  {
    type: 'xlsx',
    label: 'Excel',
    extensions: ['.xlsx'],
    maxBytes: 20 * 1024 * 1024,
  },
]
export const getKnowledgeFormat = (file: {
  type: string
  name: string
}): KnowledgeFormat | null => {
  if (file.type !== 'file') return null
  const name = file.name.toLowerCase()
  return KNOWLEDGE_FORMATS.find(f => f.extensions.some(ext => name.endsWith(ext))) ?? null
}
export const isMarkdownFile = (file: { type: string; name: string }) =>
  getKnowledgeFormat(file)?.type === 'markdown'
// 详情页来源展示名；未知历史类型原样透出。
export const knowledgeSourceLabel = (sourceType: string) =>
  KNOWLEDGE_FORMATS.find(f => f.type === sourceType)?.label ?? sourceType
export const stripKnowledgeExtension = (name: string) => {
  const format = getKnowledgeFormat({ type: 'file', name })
  const ext = format?.extensions.find(e => name.toLowerCase().endsWith(e))
  return ext ? name.slice(0, -ext.length) : name
}
export const isKnowledgeActive = (status: KnowledgeStatus) =>
  status === 'pending' || status === 'running'
export const isKnowledgeVectorActive = (state: KnowledgeVectorState) =>
  state === 'pending' || state === 'running'
export const hasKnowledgeVector = (item: Pick<KnowledgeItem, 'status' | 'vector_state'>) =>
  item.status === 'ready' && Boolean(item.vector_state)
export const knowledgeDescription = (
  item: Pick<
    KnowledgeItem,
    'status' | 'error_message' | 'vector_state' | 'chunks_total' | 'chunks_ready' | 'chunks_failed'
  >,
) => {
  if (item.status === 'failed') return item.error_message
  if (item.status !== 'ready') return '正在准备知识正文，完成后即可阅读。'
  switch (item.vector_state) {
    case 'pending':
      return '正文已入库，等待自动向量化。无需手动操作。'
    case 'running':
      return `正在向量化，已入库 ${item.chunks_ready}/${item.chunks_total} 块。`
    case 'partial':
      return `${item.chunks_ready}/${item.chunks_total} 个分块已入库，${item.chunks_failed} 个失败。可在分块结构中查看失败原因。`
    case 'ready':
      return `已向量化（${item.chunks_total} 个分块），可打开分块结构核对切分结果。`
    default:
      return '正文已提取并保存，原始文件保持不变。'
  }
}
export const knowledgeError = (error: unknown) => {
  if (axios.isAxiosError<{ msg?: string }>(error))
    return error.response?.data.msg || '请求失败，请检查网络后重试'
  return error instanceof Error ? error.message : '请求失败，请稍后重试'
}
export const knowledgeDate = (date: string | null) => {
  if (!date) return '尚未完成'
  const value = new Date(date)
  return Number.isNaN(value.getTime()) ? '未知时间' : value.toLocaleString('zh-CN')
}
