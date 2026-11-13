import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import FileManagement from '../FileManagement.vue'
import { useUserStore } from '../../stores/user'
import { createMemoryHistory, createRouter } from 'vue-router'
import {
  getListByFolderIDAndUserID,
  getListCountByFolderIDAndUserID,
} from '../../services/apis/file'

vi.mock('../../components/ui/AppIcon.vue', () => ({
  default: { template: '<span aria-hidden="true" />' },
}))

vi.mock('../../services/apis/file', () => ({
  createPublicShareLink: vi.fn(),
  deleteFile: vi.fn(),
  deletePublicShareLink: vi.fn(),
  downloadById: vi.fn(),
  getListByFolderIDAndUserID: vi.fn(),
  getListCountByFolderIDAndUserID: vi.fn(),
  getPublicShareLink: vi.fn(),
  makeDirectory: vi.fn(),
  moveFile: vi.fn(),
  previewFileById: vi.fn(),
  renameFile: vi.fn(),
  uploadFile: vi.fn(),
}))

const listMock = vi.mocked(getListByFolderIDAndUserID)
const countMock = vi.mocked(getListCountByFolderIDAndUserID)

describe('FileManagement', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the authenticated file page and switches to the lazy grid view', async () => {
    countMock.mockResolvedValue(2)
    listMock.mockResolvedValue([
      {
        id: 1,
        name: 'same-id-folder',
        type: 'folder',
        file_type: '',
        size: 0,
        updated_at: '2026-01-01T00:00:00Z',
      },
      {
        id: 1,
        name: 'same-id-file.txt',
        type: 'file',
        file_type: 'document',
        size: 10,
        updated_at: '2026-01-01T00:00:00Z',
      },
    ])

    const pinia = createPinia()
    useUserStore(pinia).setToken('test-token')
    const wrapper = mount(FileManagement, {
      global: {
        plugins: [
          pinia,
          createRouter({
            history: createMemoryHistory(),
            routes: [{ path: '/', component: { template: '<div />' } }],
          }),
        ],
        stubs: { AppIcon: true },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('same-id-folder')
    expect(wrapper.text()).toContain('same-id-file.txt')
    expect(listMock).toHaveBeenCalledWith(0, 1, 10, expect.any(AbortSignal))

    await wrapper.get('button[aria-label="网格视图"]').trigger('click')
    await vi.dynamicImportSettled()
    await flushPromises()
    expect(wrapper.text()).toContain('same-id-folder')
    expect(wrapper.find('.grid').exists()).toBe(true)

    wrapper.unmount()
  })
})
