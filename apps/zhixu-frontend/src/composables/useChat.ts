import { ref, onScopeDispose } from 'vue'
import {
  streamChat,
  getConversation,
  type ChatSource,
  type ChatWireMessage,
} from '../services/apis/chat'
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  model?: string
  status?: 'generating' | 'completed' | 'stopped' | 'failed'
  error?: string
  sources?: ChatSource[]
  query?: string
  rag?: boolean
}
export function useChat() {
  const messages = ref<ChatMessage[]>([])
  const running = ref(false)
  // Set after the first turn: the backend opens a Mastra thread and echoes its id back
  // in the meta event. Subsequent turns reuse it so history accumulates server-side.
  const conversationId = ref('')
  const loadingHistory = ref(false)
  let controller: AbortController | null = null
  let historyController: AbortController | null = null
  function invalidateHistory() {
    historyController?.abort()
    historyController = null
    loadingHistory.value = false
  }
  function stop() {
    controller?.abort()
    const last = messages.value.at(-1)
    if (last?.status === 'generating') last.status = 'stopped'
    running.value = false
  }
  function reset() {
    invalidateHistory()
    stop()
    messages.value = []
    conversationId.value = ''
  }
  // loadConversation restores a persisted thread for viewing and continuing.
  async function loadConversation(id: string) {
    stop()
    invalidateHistory()
    const current = new AbortController()
    historyController = current
    loadingHistory.value = true
    try {
      const detail = await getConversation(id, current.signal)
      if (historyController !== current || current.signal.aborted) return
      conversationId.value = detail.conversation.id
      messages.value = detail.messages.map(m => ({
        role: m.role,
        content: m.content,
        status: 'completed',
        query: m.query,
        sources: m.sources,
        // Persisted history has no explicit mode; even empty sources mark a RAG turn.
        rag: m.query !== undefined || m.sources !== undefined,
      }))
    } catch (e) {
      if (historyController === current && !current.signal.aborted) throw e
    } finally {
      if (historyController === current) {
        historyController = null
        loadingHistory.value = false
      }
    }
  }
  async function generate(modelID: number, label: string, rag: boolean) {
    const current = new AbortController()
    controller = current
    running.value = true
    const history: ChatWireMessage[] = messages.value
      .filter(m => m.role === 'user' || m.status === 'completed')
      .map(m => ({ role: m.role, content: m.content }))
    const message: ChatMessage = {
      role: 'assistant',
      content: '',
      model: label,
      status: 'generating',
      rag,
    }
    messages.value.push(message)
    const index = messages.value.length - 1
    try {
      await streamChat(
        modelID,
        history,
        current.signal,
        event => {
          if (current.signal.aborted) return
          const reply = messages.value[index]!
          if (event.type === 'delta') {
            if (reply.content.length + event.text.length > 200000)
              throw new Error('回复过长，请开始新对话')
            reply.content += event.text
          }
          if (event.type === 'meta') {
            reply.model = `${event.provider} / ${event.model}`
            if (event.conversation_id) conversationId.value = event.conversation_id
          }
          if (event.type === 'sources') {
            reply.sources = event.items
            reply.query = event.query
          }
        },
        rag,
        conversationId.value || undefined,
      )
      if (!current.signal.aborted) {
        const reply = messages.value[index]!
        if (!reply.content.trim())
          throw new Error('模型未返回公开文字回复，请检查模型是否支持普通对话')
        reply.status = 'completed'
      }
    } catch (e) {
      if (!current.signal.aborted) {
        const reply = messages.value[index]!
        reply.status = 'failed'
        reply.error = e instanceof Error ? e.message : '模型调用失败，请手动重试'
      }
    } finally {
      if (controller === current) running.value = false
    }
  }
  async function send(text: string, modelID: number, label: string, rag = false) {
    if (running.value || !text.trim()) return false
    const last = messages.value.at(-1)
    if (last?.role === 'assistant' && last.status !== 'completed') return false
    invalidateHistory()
    messages.value.push({ role: 'user', content: text.trim() })
    await generate(modelID, label, rag)
    return true
  }
  async function retry(index: number, modelID: number, label: string) {
    const reply = messages.value[index]
    if (running.value || loadingHistory.value || reply?.role !== 'assistant') return
    const rag = reply.rag ?? (reply.query !== undefined || reply.sources !== undefined)
    messages.value.splice(index)
    await generate(modelID, label, rag)
  }
  onScopeDispose(() => {
    invalidateHistory()
    stop()
  })
  return {
    messages,
    running,
    conversationId,
    loadingHistory,
    stop,
    reset,
    loadConversation,
    send,
    retry,
  }
}
