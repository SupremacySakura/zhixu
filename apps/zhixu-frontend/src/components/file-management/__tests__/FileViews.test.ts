import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { FileDisplayItem, FileItemKey } from '../../../types/file'
import FileListView from '../FileListView.vue'
import FilePagination from '../FilePagination.vue'

vi.mock('../../ui/AppIcon.vue', () => ({
  default: { template: '<span aria-hidden="true" />' },
}))

const entry = (overrides: Partial<FileDisplayItem>): FileDisplayItem => ({
  id: 1,
  key: 'file:1',
  name: 'file.txt',
  type: 'file',
  file_type: 'document',
  size: 10,
  updated_at: '2026-01-01T00:00:00Z',
  icon: 'material-symbols:description',
  iconBg: 'bg-primary/10',
  iconFg: 'text-primary',
  typeLabel: 'Document',
  lastModifiedText: '2026/1/1',
  ...overrides,
})

describe('file management views', () => {
  it('selects colliding file and folder ids by their typed key', async () => {
    const folder = entry({ key: 'folder:1', type: 'folder', name: 'folder' })
    const file = entry({ key: 'file:1', type: 'file', name: 'file' })
    const wrapper = mount(FileListView, {
      props: {
        files: [folder, file],
        loading: false,
        hasParentFolder: false,
        selectedIds: new Set<FileItemKey>(['folder:1']),
        allSelected: false,
      },
      global: { stubs: { AppIcon: true } },
    })

    const rowCheckboxes = wrapper.findAll('input[type="checkbox"][aria-label^="选择 "]')
    expect((rowCheckboxes[0]?.element as HTMLInputElement).checked).toBe(true)
    expect((rowCheckboxes[1]?.element as HTMLInputElement).checked).toBe(false)

    await rowCheckboxes[1]?.setValue(true)
    expect(wrapper.emitted('toggle-select')?.[0]).toEqual([file, true])
  })

  it('emits a bounded page change from the shared paginator', async () => {
    const wrapper = mount(FilePagination, {
      props: {
        page: 2,
        totalPages: 5,
        startIndex: 11,
        endIndex: 20,
        totalCount: 50,
        loading: false,
      },
      global: { stubs: { AppIcon: true } },
    })

    await wrapper.get('button[aria-label="下一页"]').trigger('click')
    expect(wrapper.emitted('change')).toEqual([[3]])
  })
})
