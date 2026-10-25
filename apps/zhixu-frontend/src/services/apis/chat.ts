import { useUserStore } from '../../stores/user'
import request from '../request'

export interface ChatWireMessage {
  role: 'user' | 'assistant'
  content: string
}
export interface ChatSource {
  knowledge_id: number
  title: string
  section: string
  content: string
  score: number
}
export type ChatEvent =
  | { type: 'delta'; text: string }
  | { type: 'meta'; model_id: number; model: string; provider: string; conversation_id?: string }
  | { type: 'sources'; query: string; items: ChatSource[] }
  | { type: 'done' }
  | { type: 'error'; message: string }

export interface ConversationSummary {
  id: string
  title: string
  created_at: string
  updated_at: string
}
export interface ConversationMessage {
  role: 'user' | 'assistant'
  content: string
  created_at: string
  query?: string
  sources?: ChatSource[]
}
export interface ConversationDetail {
  conversation: ConversationSummary
  messages: ConversationMessage[]
}

export async function listConversations(signal?: AbortSignal) {
  const res = await request.get<{ code: number; data?: { conversations: ConversationSummary[] } }>(
    '/api/chat/conversations',
    { signal },
  )
  return res.data.data?.conversations ?? []
}
export async function getConversation(id: string, signal?: AbortSignal) {
  const res = await request.get<{ code: number; data?: ConversationDetail }>(
    `/api/chat/conversations/${id}`,
    { signal },
  )
  if (!res.data.data) throw new Error('会话不存在')
  return res.data.data
}
export async function renameConversation(id: string, title: string) {
  await request.patch(`/api/chat/conversations/${id}`, { title })
}
export async function deleteConversation(id: string) {
  await request.delete(`/api/chat/conversations/${id}`)
}

export async function readChatStream(response: Response, onEvent: (event: ChatEvent) => void) {
  if (!response.ok) {
    const data = await response.json().catch(() => null)
    throw new Error(data?.msg || '对话请求失败，请检查登录状态与模型服务')
  }
  if (!response.body || !response.headers.get('Content-Type')?.includes('application/x-ndjson'))
    throw new Error('对话服务返回格式无效')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  function consume(line: string) {
    if (!line.trim()) return
    const event = JSON.parse(line) as ChatEvent
    if (event.type === 'error') throw new Error(event.message)
    if (event.type === 'done') completed = true
    if (event.type === 'delta' && typeof event.text !== 'string') throw new Error('回复格式无效')
    onEvent(event)
  }
  try {
    for (;;) {
      const { value, done } = await reader.read()
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true })
      let split: number
      while ((split = buffer.indexOf('\n')) !== -1) {
        consume(buffer.slice(0, split))
        buffer = buffer.slice(split + 1)
      }
      if (buffer.length > 1024 * 1024) throw new Error('回复事件过大')
      if (done) break
    }
    if (buffer.trim()) consume(buffer)
    if (!completed) throw new Error('回复连接中断，已收到的内容保留，请手动重试')
  } finally {
    await reader.cancel().catch(() => {})
  }
}
export async function streamChat(
  modelID: number,
  messages: ChatWireMessage[],
  signal: AbortSignal,
  onEvent: (event: ChatEvent) => void,
  rag = false,
  conversationID?: string,
) {
  const user = useUserStore()
  // Dedicated fetch avoids Axios's automatic retries and 30-second timeout.
  const response = await fetch('/api/chat', {
    method: 'POST',
    signal,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${user.token}` },
    body: JSON.stringify({ model_id: modelID, messages, rag, conversation_id: conversationID }),
  })
  if (response.status === 401) user.logout()
  await readChatStream(response, onEvent)
}
