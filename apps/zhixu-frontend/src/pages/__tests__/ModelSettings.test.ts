import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelSettings from '../ModelSettings.vue'
import ProviderEditor from '../../components/ui/ProviderEditor.vue'
import {
  getAIConfiguration,
  getAIProviderCatalog,
  saveAIProvider,
  saveAIDefaults,
} from '../../services/apis/aiSettings'
import type { AIConfiguration } from '../../services/apis/aiSettings'
import { getServiceStatus } from '../../services/apis/serviceStatus'
import AppToast from '../../components/ui/AppToast.vue'
import { defineComponent } from 'vue'

vi.mock('../../services/apis/serviceStatus', () => ({ getServiceStatus: vi.fn() }))

vi.mock('../../services/apis/aiSettings', () => ({
  getAIConfiguration: vi.fn(),
  getAIProviderCatalog: vi.fn(),
  saveAIProvider: vi.fn(),
  saveAIDefaults: vi.fn(),
  deleteAIProvider: vi.fn(),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

// 页面在生产里永远活在带 toast 宿主的外壳里；测试如实照搬这个结构，
// 而不是让页面自己渲染提示——那样测的是旧架构，不是用户看到的东西。
const Harness = defineComponent({
  components: { Page: ModelSettings, AppToast },
  template: '<div><Page /><AppToast /></div>',
})
const fixture = (): AIConfiguration => ({
  providers: [
    {
      id: 1,
      provider_id: 'gateway',
      name: '模型网关',
      catalog_id: '',
      base_url: 'https://example.com/v1',
      protocol: 'chat',
      auth_type: 'key',
      credential_configured: true,
      models: [
        { id: 2, provider_id: 1, model_id: 'chat-model', kind: 'chat', dimension: 0 },
        { id: 3, provider_id: 1, model_id: 'embed', kind: 'embedding', dimension: 1024 },
        { id: 4, provider_id: 1, model_id: 'embed-next', kind: 'embedding', dimension: 1024 },
      ],
    },
  ],
  settings: {
    chat_model_id: 2,
    embedding_model_id: 3,
    rag_top_k: 5,
    rag_min_score: 0.5,
    evolve_candidate_min_score: 0.65,
    evolve_threshold_stale: false,
    config_version: 5,
    embedding_version: 1,
    needs_reindex: false,
  },
})
beforeEach(() => {
  vi.mocked(getAIConfiguration).mockResolvedValue(fixture())
  vi.mocked(getAIProviderCatalog).mockResolvedValue([])
  vi.mocked(getServiceStatus).mockResolvedValue({
    checked_at: '2026-10-26T08:00:00Z',
    services: [
      { id: 'mastra', status: 'healthy', latency_ms: 10, message: '连接正常' },
      { id: 'qdrant', status: 'unconfigured', latency_ms: null, message: '未配置向量库' },
    ],
  })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})
describe('ModelSettings', () => {
  it('loads configuration and checks real service reports only on tab entry or refresh', async () => {
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('模型网关')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(getServiceStatus).not.toHaveBeenCalled()
    const tab = wrapper.findAll('nav button').find(b => b.text() === '服务状态')!
    await tab.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('运行正常')
    expect(wrapper.text()).toContain('未配置向量库')
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
    await tab.trigger('click')
    expect(getServiceStatus).toHaveBeenCalledTimes(1)
    vi.mocked(getServiceStatus).mockRejectedValueOnce(new Error('刷新失败'))
    await wrapper
      .findAll('button')
      .find(b => b.text() === '刷新状态')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('旧结果')
    expect(wrapper.text()).toContain('运行正常')
    expect(wrapper.get('time').attributes('datetime')).toBe('2026-10-26T08:00:00Z')
    expect(getServiceStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('confirms same-dimension embedding changes before submitting', async () => {
    vi.mocked(saveAIDefaults).mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get('[aria-label="选择默认 Embedding 模型"]').trigger('click')
    const option = wrapper
      .findAll('[role="menuitemradio"]')
      .find(b => b.text().includes('embed-next'))!
    await option.trigger('click')
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    expect(saveAIDefaults).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('即使维度相同')
    await wrapper
      .findAll('dialog button')
      .find(b => b.text() === '确认切换')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).toHaveBeenCalledWith(
      {
        chat_model_id: 2,
        embedding_model_id: 4,
        rag_top_k: 5,
        rag_min_score: 0.5,
        expected_version: 5,
        confirm_reindex: true,
      },
      expect.any(AbortSignal),
    )
    wrapper.unmount()
  })
  it('submits retrieval breadth and threshold with the defaults payload', async () => {
    vi.mocked(saveAIDefaults).mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get<HTMLInputElement>('#rag-top-k').setValue(20)
    await wrapper.get<HTMLInputElement>('#rag-min-score').setValue(0.7)
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).toHaveBeenCalledWith(
      {
        chat_model_id: 2,
        embedding_model_id: 3,
        rag_top_k: 20,
        rag_min_score: 0.7,
        expected_version: 5,
        confirm_reindex: false,
      },
      expect.any(AbortSignal),
    )
    wrapper.unmount()
  })
  it('rejects out-of-range retrieval settings before saving', async () => {
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get<HTMLInputElement>('#rag-min-score').setValue(0.01)
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('召回下限需在 0.05–1 之间')
    wrapper.unmount()
  })
  it('labels the retrieval floor as 召回下限 and never as a bare 相似度阈值', async () => {
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    // 术语表禁止用「相似度阈值」单独指代三个门槛之一，标签必须带上前缀。
    expect(wrapper.get('label[for="rag-min-score"]').text()).toBe('召回下限')
    expect(wrapper.text()).not.toContain('相似度阈值')
    expect(wrapper.text()).toContain('相似度低于召回下限')
    wrapper.unmount()
  })
  it('submits the evolution candidate gate alongside the other defaults', async () => {
    vi.mocked(saveAIDefaults).mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    const gate = wrapper.get<HTMLInputElement>('#evolve-candidate-min-score')
    expect(gate.element.value).toBe('0.65')
    expect(wrapper.text()).toContain('低于该值的近邻不会进入判定')
    await gate.setValue(0.5)
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).toHaveBeenCalledWith(
      {
        chat_model_id: 2,
        embedding_model_id: 3,
        rag_top_k: 5,
        rag_min_score: 0.5,
        evolve_candidate_min_score: 0.5,
        expected_version: 5,
        confirm_reindex: false,
      },
      expect.any(AbortSignal),
    )
    wrapper.unmount()
  })
  it('rejects an evolution gate outside 0.45–1 before saving', async () => {
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get<HTMLInputElement>('#evolve-candidate-min-score').setValue(0.3)
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('演化候选门槛需在 0.45–1 之间')
    wrapper.unmount()
  })
  it('warns that a new embedding model invalidates the gate and mixes similarity until reindex', async () => {
    vi.mocked(getAIConfiguration).mockResolvedValue({
      ...fixture(),
      settings: { ...fixture().settings, evolve_threshold_stale: true },
    })
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    const notice = wrapper.get('[data-test="evolve-threshold-stale"]')
    expect(notice.text()).toContain('相似度分布')
    expect(notice.text()).toContain('向量重建完成前')
    expect(notice.text()).toContain('混算')
    expect(notice.text()).toContain('0.65')
    wrapper.unmount()
  })
  it('keeps the saved gate untouched when only the embedding model changes', async () => {
    vi.mocked(saveAIDefaults).mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get('[aria-label="选择默认 Embedding 模型"]').trigger('click')
    await wrapper
      .findAll('[role="menuitemradio"]')
      .find(b => b.text().includes('embed-next'))!
      .trigger('click')
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保存默认模型')!
      .trigger('click')
    await wrapper
      .findAll('dialog button')
      .find(b => b.text() === '确认切换')!
      .trigger('click')
    await flushPromises()
    const payload = vi.mocked(saveAIDefaults).mock.calls[0][0]
    expect(payload).not.toHaveProperty('evolve_candidate_min_score')
    expect(payload.confirm_reindex).toBe(true)
    wrapper.unmount()
  })
  it('clears the stale warning when the user resaves the current gate', async () => {
    vi.mocked(getAIConfiguration).mockResolvedValue({
      ...fixture(),
      settings: { ...fixture().settings, evolve_threshold_stale: true },
    })
    vi.mocked(saveAIDefaults).mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper
      .findAll('button')
      .find(b => b.text() === '保留当前门槛')!
      .trigger('click')
    await flushPromises()
    expect(saveAIDefaults).toHaveBeenCalledWith(
      expect.objectContaining({ evolve_candidate_min_score: 0.65, confirm_reindex: false }),
      expect.any(AbortSignal),
    )
    expect(wrapper.find('[data-test="evolve-threshold-stale"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('retains the editor on save failure and clears it only after success', async () => {
    vi.mocked(saveAIProvider)
      .mockRejectedValueOnce(new Error('无法保存配置'))
      .mockResolvedValue(fixture())
    const wrapper = mount(Harness, { global: { stubs: { teleport: true } } })
    await flushPromises()
    await wrapper.get('[aria-label="配置 模型网关"]').trigger('click')
    const editor = wrapper.getComponent(ProviderEditor)
    await editor.get('#provider-key').setValue('private-key')
    await editor.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('无法保存配置')
    expect((editor.get('#provider-key').element as HTMLInputElement).value).toBe('private-key')
    await editor.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.findComponent(ProviderEditor).exists()).toBe(false)
    wrapper.unmount()
  })
})
describe('ProviderEditor', () => {
  it('tells users that private endpoints need a deployment switch', () => {
    const config = fixture()
    const wrapper = mount(ProviderEditor, {
      global: { stubs: { teleport: true } },
      props: {
        provider: config.providers[0]!,
        preset: null,
        configuration: config,
        busy: false,
        error: '',
        conflict: false,
      },
    })
    expect(wrapper.text()).toContain('MODEL_ALLOW_PRIVATE_NETWORKS')
    expect(wrapper.text()).toContain('内网或本机地址')
    wrapper.unmount()
  })
  it('omits unchanged credentials, supports clearing, and validates duplicate model IDs', async () => {
    const config = fixture()
    const wrapper = mount(ProviderEditor, {
      global: { stubs: { teleport: true } },
      props: {
        provider: config.providers[0]!,
        preset: null,
        configuration: config,
        busy: false,
        error: '',
        conflict: false,
      },
    })
    expect((wrapper.get('#provider-key').element as HTMLInputElement).value).toBe('')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('submit')![0]![0]).not.toHaveProperty('api_key')
    await wrapper
      .findAll('button')
      .find(b => b.text() === '清除凭据')!
      .trigger('click')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('submit')![1]![0]).toHaveProperty('clear_api_key', true)
    const models = wrapper
      .findAll('input')
      .filter(input => input.attributes('placeholder') === '填写服务商的模型 ID')
    await models[1]!.setValue('CHAT-MODEL')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.text()).toContain('模型 ID 不能为空或重复')
    expect(wrapper.emitted('submit')).toHaveLength(2)
    wrapper.unmount()
  })
})
