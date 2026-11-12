import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import KnowledgeHistoryDialog from '../KnowledgeHistoryDialog.vue'
import {
  getKnowledgeVersion,
  listKnowledgeVersions,
  restoreKnowledgeVersion,
} from '../../../services/apis/knowledge'
vi.mock('../../ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('../../../services/apis/knowledge', async importOriginal => {
  const actual = await importOriginal<typeof import('../../../services/apis/knowledge')>()
  return {
    ...actual,
    listKnowledgeVersions: vi.fn(),
    getKnowledgeVersion: vi.fn(),
    restoreKnowledgeVersion: vi.fn(),
  }
})
const versionSummary = (version: number, current: boolean) => ({
  version_number: version,
  title: `第 ${version} 版标题`,
  change_source: 'manual',
  created_at: '2026-11-10T10:00:00+08:00',
  current,
})
const mountDialog = () =>
  mount(KnowledgeHistoryDialog, {
    props: { knowledgeId: 10 },
    attachTo: document.body,
  })
const buttonByText = (text: string) =>
  [...document.body.querySelectorAll('button')].find(b => b.textContent?.includes(text))
describe('KnowledgeHistoryDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  it('loads versions and previews the current version', async () => {
    vi.mocked(listKnowledgeVersions).mockResolvedValue({
      items: [versionSummary(2, true), versionSummary(1, false)],
      total: 2,
    })
    vi.mocked(getKnowledgeVersion).mockResolvedValue({
      ...versionSummary(2, true),
      content: '# 当前正文',
    })
    const wrapper = mountDialog()
    await flushPromises()
    expect(getKnowledgeVersion).toHaveBeenCalledWith(10, 2)
    const text = document.body.textContent || ''
    expect(text).toContain('第 2 版')
    expect(text).toContain('第 1 版')
    expect(text).toContain('当前')
    expect(text).toContain('第 2 版预览')
    wrapper.unmount()
  })
  it('previews a snapshot and restores it', async () => {
    vi.mocked(listKnowledgeVersions).mockResolvedValue({
      items: [versionSummary(2, true), versionSummary(1, false)],
      total: 2,
    })
    vi.mocked(getKnowledgeVersion).mockImplementation(async (_id, version) => ({
      ...versionSummary(version, version === 2),
      content: version === 2 ? '# 当前正文' : '# 第一版正文',
    }))
    const restored = { id: 10, version: 3 } as never
    vi.mocked(restoreKnowledgeVersion).mockResolvedValue(restored)
    const wrapper = mountDialog()
    await flushPromises()
    buttonByText('第 1 版')!.click()
    await flushPromises()
    expect(getKnowledgeVersion).toHaveBeenLastCalledWith(10, 1)
    expect(document.body.textContent).toContain('第 1 版预览')
    buttonByText('恢复此版本')!.click()
    await flushPromises()
    expect(restoreKnowledgeVersion).toHaveBeenCalledWith(10, 1)
    expect(wrapper.emitted('restored')?.[0]).toEqual([restored])
    wrapper.unmount()
  })
  it('disables restoring the current version', async () => {
    vi.mocked(listKnowledgeVersions).mockResolvedValue({
      items: [versionSummary(2, true), versionSummary(1, false)],
      total: 2,
    })
    vi.mocked(getKnowledgeVersion).mockResolvedValue({
      ...versionSummary(2, true),
      content: '# 当前正文',
    })
    const wrapper = mountDialog()
    await flushPromises()
    const restoreButton = buttonByText('恢复此版本')!
    expect(restoreButton.hasAttribute('disabled')).toBe(true)
    wrapper.unmount()
  })
})
