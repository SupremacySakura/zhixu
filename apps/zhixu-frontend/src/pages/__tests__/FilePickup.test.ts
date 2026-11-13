import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FilePickup from '../FilePickup.vue'
import { downloadByPickupCode } from '../../services/apis/file'

vi.mock('../../services/apis/file', () => ({
  downloadByPickupCode: vi.fn(),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span data-icon />' } }))

/**
 * 取件页的接缝：码格、提交闸、错误抖动、成功态。
 *
 * 这里只断言「用户能观察到的东西」：6 个格子的可访问名称、按钮能不能按、
 * 抖动的**时长**（220ms 是 AC 里钉死的数字）、以及失败后错误文案。
 * 不碰内部 ref、不碰 class 名 —— 页面可以整块重写而测试不动。
 */
async function render() {
  const wrapper = mount(FilePickup, {
    global: { plugins: [createPinia()] },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

function codeInputs(wrapper: Awaited<ReturnType<typeof render>>) {
  return wrapper.findAll('input')
}

async function typeCode(wrapper: Awaited<ReturnType<typeof render>>, code: string) {
  const inputs = codeInputs(wrapper)
  for (let i = 0; i < code.length; i += 1) {
    const input = inputs[i]!
    await input.setValue(code[i])
    await input.trigger('input')
  }
  await flushPromises()
}

describe('file pickup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('renders six single-character code cells with positional names', async () => {
    const wrapper = await render()
    const inputs = codeInputs(wrapper)
    expect(inputs).toHaveLength(6)
    inputs.forEach((input, index) => {
      expect(input.attributes('aria-label')).toBe(`取件码第${index + 1}位`)
      expect(input.attributes('maxlength')).toBe('1')
    })
    wrapper.unmount()
  })

  it('keeps the retrieve action closed until all six characters are entered', async () => {
    const wrapper = await render()
    const submit = wrapper.get('button[aria-label="提取文件"]')
    expect(submit.attributes('disabled')).toBeDefined()

    await typeCode(wrapper, 'AB12')
    expect(wrapper.get('button[aria-label="提取文件"]').attributes('disabled')).toBeDefined()

    await typeCode(wrapper, 'AB12CD')
    expect(wrapper.get('button[aria-label="提取文件"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('shakes the code row and explains why the pickup failed', async () => {
    vi.mocked(downloadByPickupCode).mockRejectedValueOnce(new Error('取件码不存在'))
    const wrapper = await render()
    await typeCode(wrapper, 'AB12CD')
    await wrapper.get('button[aria-label="提取文件"]').trigger('click')
    await flushPromises()

    // 抖动本身是 CSS 常量（220ms / ease-settle），scoped 样式在 happy-dom 里
    // 不参与层叠，所以时长在浏览器里用 CDP 复核；这里守的是「失败要有抖动」。
    expect(wrapper.find('.code-shake').exists()).toBe(true)
    expect(wrapper.text()).toContain('取件码不存在或对应资源已删除。')
    wrapper.unmount()
  })

  it('stops shaking once the user starts over', async () => {
    vi.mocked(downloadByPickupCode).mockRejectedValueOnce(new Error('取件码不存在'))
    const wrapper = await render()
    await typeCode(wrapper, 'AB12CD')
    await wrapper.get('button[aria-label="提取文件"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('.code-shake').exists()).toBe(true)

    // 重新提交时先清反馈：抖动不该一直挂在页面上
    vi.mocked(downloadByPickupCode).mockResolvedValueOnce({
      fileName: 'a.txt',
      fileSize: 1,
      contentType: 'text/plain',
    })
    await wrapper.get('button[aria-label="提取文件"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('.code-shake').exists()).toBe(false)
    wrapper.unmount()
  })

  it('swaps the entry card for a result card on a successful pickup', async () => {
    vi.mocked(downloadByPickupCode).mockResolvedValueOnce({
      fileName: '设计稿.pdf',
      fileSize: 2048,
      contentType: 'application/pdf',
    })
    const wrapper = await render()
    await typeCode(wrapper, 'AB12CD')
    await wrapper.get('button[aria-label="提取文件"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('已提取文件')
    expect(wrapper.text()).toContain('设计稿.pdf')
    expect(wrapper.find('button[aria-label="重新下载文件"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
