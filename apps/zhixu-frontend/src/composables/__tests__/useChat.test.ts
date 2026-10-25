import { effectScope } from 'vue'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { useChat } from '../useChat'
import { streamChat, getConversation, type ConversationDetail } from '../../services/apis/chat'
vi.mock('../../services/apis/chat', () => ({ streamChat: vi.fn(), getConversation: vi.fn() }))
afterEach(() => vi.resetAllMocks())
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
function conversation(id: string): ConversationDetail {
  return {
    conversation: { id, title: id, created_at: '', updated_at: '' },
    messages: [{ role: 'user', content: id, created_at: '' }],
  }
}
describe('temporary chat', () => {
  it('sends completed multi-turn context and never duplicates the user on retry', async () => {
    vi.mocked(streamChat).mockImplementation(async (_id, _messages, _signal, emit) => {
      emit({ type: 'delta', text: '你好' })
      emit({ type: 'done' })
    })
    const scope = effectScope()
    const chat = scope.run(useChat)!
    await chat.send('第一轮', 2, 'provider/model')
    await chat.send('第二轮', 2, 'provider/model')
    expect(vi.mocked(streamChat).mock.calls[1]![1]).toEqual([
      { role: 'user', content: '第一轮' },
      { role: 'assistant', content: '你好' },
      { role: 'user', content: '第二轮' },
    ])
    vi.mocked(streamChat).mockRejectedValueOnce(new Error('认证失败'))
    await chat.send('第三轮', 2, 'provider/model')
    expect(chat.messages.value.at(-1)?.status).toBe('failed')
    await chat.retry(5, 2, 'provider/model')
    expect(chat.messages.value).toHaveLength(6)
    scope.stop()
  })
  it('aborts on reset and prevents stale deltas from reviving cleared messages', async () => {
    let finish!: () => void
    vi.mocked(streamChat).mockImplementation(
      (_id, _messages, _signal, emit) =>
        new Promise<void>(resolve => {
          finish = () => {
            emit({ type: 'delta', text: '旧回复' })
            resolve()
          }
        }),
    )
    const scope = effectScope()
    const chat = scope.run(useChat)!
    const task = chat.send('你好', 2, 'provider/model')
    const signal = vi.mocked(streamChat).mock.calls[0]![2]
    chat.reset()
    expect(signal.aborted).toBe(true)
    finish()
    await task
    expect(chat.messages.value).toEqual([])
    scope.stop()
  })
  it.each([true, false])(
    'retry preserves the original rag=%s mode before any sources arrive',
    async rag => {
      vi.mocked(streamChat).mockRejectedValue(new Error('调用失败'))
      const scope = effectScope()
      const chat = scope.run(useChat)!
      await chat.send('第一轮', 2, 'provider/model', rag)
      expect(chat.messages.value[1]?.rag).toBe(rag)
      await chat.retry(1, 2, 'provider/model')
      expect(vi.mocked(streamChat).mock.calls[1]![4]).toBe(rag)
      expect(chat.messages.value).toHaveLength(2)
      scope.stop()
    },
  )
  it('stores knowledge sources on the assistant message and forwards the rag flag', async () => {
    vi.mocked(streamChat).mockImplementation(async (_id, _messages, _signal, emit) => {
      emit({
        type: 'sources',
        query: '回滚步骤',
        items: [
          { knowledge_id: 9, title: '发布流程', section: '部署', content: '先备份', score: 0.9 },
        ],
      })
      emit({ type: 'delta', text: '根据 [1] 先备份。' })
      emit({ type: 'done' })
    })
    const scope = effectScope()
    const chat = scope.run(useChat)!
    await chat.send('怎么回滚？', 2, 'provider/model', true)
    expect(vi.mocked(streamChat).mock.calls[0]![4]).toBe(true)
    const reply = chat.messages.value.at(-1)
    expect(reply?.status).toBe('completed')
    expect(reply?.query).toBe('回滚步骤')
    expect(reply?.sources).toHaveLength(1)
    expect(reply?.sources?.[0]?.title).toBe('发布流程')
    scope.stop()
  })
})
describe('conversation history', () => {
  it('aborts older loads and lets only the latest result replace history', async () => {
    const first = deferred<ConversationDetail>()
    const second = deferred<ConversationDetail>()
    vi.mocked(getConversation)
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const scope = effectScope()
    const chat = scope.run(useChat)!
    const older = chat.loadConversation('old')
    const newer = chat.loadConversation('new')
    expect(vi.mocked(getConversation).mock.calls[0]![1]?.aborted).toBe(true)
    expect(chat.loadingHistory.value).toBe(true)
    second.resolve(conversation('new'))
    await newer
    first.resolve(conversation('old'))
    await older
    expect(chat.conversationId.value).toBe('new')
    expect(chat.messages.value[0]?.content).toBe('new')
    expect(chat.loadingHistory.value).toBe(false)
    scope.stop()
  })
  it('does not clear a newer loading state or surface an obsolete failure', async () => {
    const first = deferred<ConversationDetail>()
    const second = deferred<ConversationDetail>()
    vi.mocked(getConversation)
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const scope = effectScope()
    const chat = scope.run(useChat)!
    const older = chat.loadConversation('old')
    const newer = chat.loadConversation('new')
    first.reject(new Error('旧请求失败'))
    await expect(older).resolves.toBeUndefined()
    expect(chat.loadingHistory.value).toBe(true)
    second.reject(new Error('当前请求失败'))
    await expect(newer).rejects.toThrow('当前请求失败')
    expect(chat.loadingHistory.value).toBe(false)
    scope.stop()
  })
  it.each(['reset', 'dispose', 'send'] as const)(
    '%s invalidates pending history even if the transport ignores abort',
    async action => {
      const pending = deferred<ConversationDetail>()
      vi.mocked(getConversation).mockReturnValueOnce(pending.promise)
      vi.mocked(streamChat).mockImplementation(async (_id, _messages, _signal, emit) => {
        emit({ type: 'delta', text: '新回复' })
      })
      const scope = effectScope()
      const chat = scope.run(useChat)!
      const task = chat.loadConversation('old')
      const signal = vi.mocked(getConversation).mock.calls[0]![1]
      if (action === 'reset') chat.reset()
      if (action === 'dispose') scope.stop()
      if (action === 'send') await chat.send('新消息', 2, 'provider/model')
      expect(signal?.aborted).toBe(true)
      expect(chat.loadingHistory.value).toBe(false)
      pending.resolve(conversation('old'))
      await task
      expect(chat.conversationId.value).toBe('')
      expect(chat.messages.value.map(m => m.content)).toEqual(
        action === 'send' ? ['新消息', '新回复'] : [],
      )
      scope.stop()
    },
  )
  it('captures conversation_id from meta and forwards it on the next turn', async () => {
    vi.mocked(streamChat).mockImplementation(async (_id, _messages, _signal, emit) => {
      emit({ type: 'meta', model_id: 2, provider: 'p', model: 'm', conversation_id: 'conv-1' })
      emit({ type: 'delta', text: '好' })
      emit({ type: 'done' })
    })
    const scope = effectScope()
    const chat = scope.run(useChat)!
    await chat.send('第一轮', 2, 'provider/model')
    expect(chat.conversationId.value).toBe('conv-1')
    expect(vi.mocked(streamChat).mock.calls[0]![5]).toBeUndefined()
    await chat.send('第二轮', 2, 'provider/model')
    expect(vi.mocked(streamChat).mock.calls[1]![5]).toBe('conv-1')
    scope.stop()
  })
  it.each([
    { metadata: {}, rag: false },
    { metadata: { sources: [] }, rag: true },
    { metadata: { query: '检索词' }, rag: true },
  ])('infers persisted retry mode from metadata $metadata', async ({ metadata, rag }) => {
    vi.mocked(getConversation).mockResolvedValue({
      ...conversation('history'),
      messages: [
        { role: 'user', content: '问题', created_at: '' },
        { role: 'assistant', content: '回复', created_at: '', ...metadata },
      ],
    })
    vi.mocked(streamChat).mockRejectedValue(new Error('调用失败'))
    const scope = effectScope()
    const chat = scope.run(useChat)!
    await chat.loadConversation('history')
    expect(chat.messages.value[1]?.rag).toBe(rag)
    await chat.retry(1, 2, 'provider/model')
    expect(vi.mocked(streamChat).mock.calls[0]![4]).toBe(rag)
    scope.stop()
  })
  it('loadConversation restores messages with sources and reset clears the id', async () => {
    vi.mocked(getConversation).mockResolvedValue({
      conversation: { id: 'conv-9', title: '发布', created_at: '', updated_at: '' },
      messages: [
        { role: 'user', content: '怎么发布？', created_at: '' },
        {
          role: 'assistant',
          content: '先构建',
          created_at: '',
          query: '发布',
          sources: [
            { knowledge_id: 9, title: '发布流程', section: '步骤', content: '构建', score: 0.9 },
          ],
        },
      ],
    })
    const scope = effectScope()
    const chat = scope.run(useChat)!
    await chat.loadConversation('conv-9')
    expect(chat.conversationId.value).toBe('conv-9')
    expect(chat.messages.value).toHaveLength(2)
    expect(chat.messages.value[1]?.status).toBe('completed')
    expect(chat.messages.value[1]?.query).toBe('发布')
    expect(chat.messages.value[1]?.sources?.[0]?.title).toBe('发布流程')
    chat.reset()
    expect(chat.conversationId.value).toBe('')
    expect(chat.messages.value).toEqual([])
    scope.stop()
  })
})
