import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ChatPage from '../ChatPage.vue'
import { getAIConfiguration, getAIProviderCatalog } from '../../services/apis/aiSettings'
import { streamChat, listConversations, getConversation } from '../../services/apis/chat'
vi.mock('../../services/apis/aiSettings', () => ({
  getAIConfiguration: vi.fn(),
  getAIProviderCatalog: vi.fn(),
  saveAIProvider: vi.fn(),
  saveAIDefaults: vi.fn(),
  deleteAIProvider: vi.fn(),
}))
vi.mock('../../services/apis/chat', () => ({
  streamChat: vi.fn(),
  listConversations: vi.fn(),
  getConversation: vi.fn(),
  renameConversation: vi.fn(),
  deleteConversation: vi.fn(),
}))
vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))
const global = { stubs: { RouterLink: { template: '<a><slot /></a>' }, teleport: true } }
beforeEach(() => {
  vi.mocked(listConversations).mockResolvedValue([])
  vi.mocked(getAIProviderCatalog).mockResolvedValue([])
})
afterEach(() => vi.resetAllMocks())
function configureModel(embeddingModelID = 0) {
  vi.mocked(getAIConfiguration).mockResolvedValue({
    providers: [
      {
        id: 1,
        provider_id: 'gateway',
        catalog_id: '',
        name: '模型网关',
        base_url: 'https://example.com',
        protocol: 'chat',
        auth_type: 'none',
        credential_configured: false,
        models: [{ id: 2, provider_id: 1, model_id: 'chat', kind: 'chat', dimension: 0 }],
      },
    ],
    settings: {
      chat_model_id: 2,
      embedding_model_id: embeddingModelID,
      config_version: 1,
      embedding_version: 0,
      needs_reindex: false,
    },
  })
}
describe('ChatPage', () => {
  it('shows history loading and blocks form/Enter submissions without clearing the draft', async () => {
    configureModel()
    vi.mocked(listConversations).mockResolvedValue([
      { id: 'history', title: '旧会话', created_at: '', updated_at: '' },
    ])
    let finish!: () => void
    vi.mocked(getConversation).mockImplementation(
      () =>
        new Promise(resolve => {
          finish = () =>
            resolve({
              conversation: { id: 'history', title: '旧会话', created_at: '', updated_at: '' },
              messages: [{ role: 'user', content: '历史问题', created_at: '' }],
            })
        }),
    )
    const wrapper = mount(ChatPage, { global })
    await flushPromises()
    await wrapper.get('textarea').setValue('保留草稿')
    const historyButton = wrapper.findAll('aside button').find(b => b.text() === '旧会话')!
    await historyButton.trigger('click')
    expect(wrapper.text()).toContain('正在读取对话历史…')
    expect(wrapper.get('textarea').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[aria-label="发送消息"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    await wrapper.get('textarea').trigger('keydown', { key: 'Enter' })
    expect(streamChat).not.toHaveBeenCalled()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('保留草稿')
    finish()
    await flushPromises()
    expect(wrapper.text()).toContain('历史问题')
    expect(wrapper.text()).not.toContain('正在读取对话历史…')
    expect(wrapper.get('textarea').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it.each([true, false])(
    'retries with original rag=%s even after the checkbox changes',
    async rag => {
      configureModel(3)
      vi.mocked(streamChat).mockRejectedValue(new Error('调用失败'))
      const wrapper = mount(ChatPage, { global })
      await flushPromises()
      const checkbox = wrapper.get('input[type="checkbox"]')
      await checkbox.setValue(rag)
      await wrapper.get('textarea').setValue('问题')
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      await checkbox.setValue(!rag)
      const retryButton = wrapper.findAll('button').find(b => b.text() === '重试这条消息')!
      await retryButton.trigger('click')
      await flushPromises()
      expect(vi.mocked(streamChat).mock.calls.map(call => call[4])).toEqual([rag, rag])
      wrapper.unmount()
    },
  )
  it('shows configuration guidance without requiring embedding', async () => {
    vi.mocked(getAIConfiguration).mockResolvedValue({
      providers: [],
      settings: {
        chat_model_id: 0,
        embedding_model_id: 0,
        config_version: 0,
        embedding_version: 0,
        needs_reindex: false,
      },
    })
    vi.mocked(getAIProviderCatalog).mockResolvedValue([])
    const wrapper = mount(ChatPage, { global })
    await flushPromises()
    expect(wrapper.text()).toContain('普通对话不需要 Embedding 配置')
    expect(wrapper.get('textarea').attributes('disabled')).toBeDefined()
    expect(wrapper.find('select').exists()).toBe(false)
    wrapper.unmount()
  })
  it('submits the configured model and displays actual streamed text', async () => {
    vi.mocked(getAIProviderCatalog).mockResolvedValue([])
    vi.mocked(getAIConfiguration).mockResolvedValue({
      providers: [
        {
          id: 1,
          provider_id: 'gateway',
          catalog_id: '',
          name: '模型网关',
          base_url: 'https://example.com',
          protocol: 'chat',
          auth_type: 'none',
          credential_configured: false,
          models: [{ id: 2, provider_id: 1, model_id: 'chat', kind: 'chat', dimension: 0 }],
        },
      ],
      settings: {
        chat_model_id: 2,
        embedding_model_id: 0,
        config_version: 1,
        embedding_version: 0,
        needs_reindex: false,
      },
    })
    vi.mocked(streamChat).mockImplementation(async (_model, _messages, _signal, emit) => {
      emit({ type: 'meta', model_id: 2, model: 'chat', provider: '模型网关' })
      emit({ type: 'delta', text: '真实流内容' })
      emit({ type: 'done' })
    })
    const wrapper = mount(ChatPage, { global })
    await flushPromises()
    await wrapper.get('textarea').setValue('你好')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(streamChat).toHaveBeenCalledWith(
      2,
      [{ role: 'user', content: '你好' }],
      expect.any(AbortSignal),
      expect.any(Function),
      false,
      undefined,
    )
    expect(wrapper.text()).toContain('真实流内容')
    expect(wrapper.text()).toContain('回复完成')
    wrapper.unmount()
  })
})
