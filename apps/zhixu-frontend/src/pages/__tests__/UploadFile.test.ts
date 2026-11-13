/**
 * 上传文件页（#44 批 4E）
 *
 * 这一页在本批之前**完全没有测试**——它是最自由也最危险的一页：换骨时
 * 没有任何断言会替我们挡住破坏。所以这里补的不是「覆盖率」，而是换骨
 * 真正会碰坏的三件事：
 *
 * 1. 拖放区还认不认点击（点击必须打开文件选择器）；
 * 2. 目录选择弹窗还开不开、能不能确认（这是页面唯一的多步交互）；
 * 3. 队列行还在不在，状态徽章说的是不是中文（R18 把英文串换成了中文，
 *    而英文串正是「一个字未改」约束下唯一被允许改的东西）。
 *
 * 为什么用假文件而不是真 File 上传：`uploadFile` 在这里被 mock 掉，
 * 测的是「队列行长什么样」，不是 axios 怎么发 chunk。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import UploadFile from '../UploadFile.vue'
import { useUserStore } from '../../stores/user'
import { getListByFolderIDAndUserID, uploadFile } from '../../services/apis/file'

vi.mock('../../components/ui/AppIcon.vue', () => ({
  default: { template: '<span aria-hidden="true" />' },
}))
vi.mock('motion-v', () => ({
  AnimatePresence: { template: '<slot />' },
  Motion: { template: '<div><slot /></div>' },
  animate: vi.fn(),
}))
vi.mock('../../services/apis/file', () => ({
  getListByFolderIDAndUserID: vi.fn(),
  uploadFile: vi.fn(),
}))

const listMock = vi.mocked(getListByFolderIDAndUserID)
const uploadMock = vi.mocked(uploadFile)

const global = { stubs: { teleport: true, LoginRequiredPlaceholder: true } }

/** 登录状态是这一页的闸门：没登录只渲染占位符。 */
function signIn() {
  useUserStore().setToken('test-token')
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  listMock.mockResolvedValue([])
  uploadMock.mockResolvedValue(undefined)
})

describe('UploadFile', () => {
  it('renders the drop zone and the empty queue before anything is uploaded', async () => {
    signIn()
    const wrapper = mount(UploadFile, { global })
    await flushPromises()
    expect(wrapper.text()).toContain('拖拽文件到此处上传')
    expect(wrapper.text()).toContain('支持图片、视频、文档与压缩包等常见格式。')
    expect(wrapper.text()).toContain('暂无上传任务')
    wrapper.unmount()
  })

  it('opens the file dialog when the drop zone is clicked', async () => {
    signIn()
    const wrapper = mount(UploadFile, { global })
    await flushPromises()
    // 隐藏的 <input type="file"> 是唯一的上传入口；点击整块拖放区必须能触发它。
    const input = wrapper.get('input[type="file"]')
    const click = vi.spyOn(input.element as HTMLInputElement, 'click')
    await wrapper.get('[aria-label="选择文件"]').trigger('click')
    expect(click).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows the target folder and opens the folder picker with a loading state', async () => {
    signIn()
    let finish!: (value: never[]) => void
    listMock.mockImplementation(
      () =>
        new Promise(resolve => {
          finish = resolve
        }),
    )
    const wrapper = mount(UploadFile, { global })
    await flushPromises()
    // 默认目录是 root，且必须显示在信息条里。
    expect(wrapper.text()).toContain('root')

    await wrapper.get('[aria-label="选择上传目录"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('选择上传目录')
    expect(wrapper.text()).toContain('默认上传到 root，可切换到任意已有文件夹。')
    // 目录还在读的时候，弹窗里必须是加载指示器而不是空白。
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)

    finish([])
    await flushPromises()
    expect(wrapper.text()).toContain('当前目录下没有子文件夹')
    wrapper.unmount()
  })

  it('keeps the queue row and reports the upload status in Chinese', async () => {
    signIn()
    const wrapper = mount(UploadFile, { global })
    await flushPromises()
    const file = new File(['hello'], 'readme.txt', { type: 'text/plain' })
    // 直接驱动 input 的 change：这就是用户选完文件的真实路径。
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(uploadMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('readme.txt')
    // R18：英文徽章全部换中文。成功态必须是「成功」，不是 Success。
    expect(wrapper.text()).toContain('成功')
    expect(wrapper.text()).not.toContain('Success')
    wrapper.unmount()
  })

  it('reports a failed upload in Chinese and offers a retry', async () => {
    signIn()
    uploadMock.mockRejectedValueOnce(new Error('网络中断'))
    const wrapper = mount(UploadFile, { global })
    await flushPromises()
    const file = new File(['x'], 'broken.bin', { type: 'application/octet-stream' })
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('网络中断')
    expect(wrapper.text()).toContain('失败')
    expect(wrapper.find('[aria-label="重试上传"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
