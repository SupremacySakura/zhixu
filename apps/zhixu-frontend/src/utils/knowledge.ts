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
export const isMarkdownFile = (file: { type: string; name: string }) =>
  file.type === 'file' && /\.(md|markdown)$/i.test(file.name)
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
      return '正文已提取并保存，原始 Markdown 文件保持不变。'
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
