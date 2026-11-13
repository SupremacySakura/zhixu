import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { PickupCodeItem } from '../../../services/types/file'
import PickupCodeCardList from '../PickupCodeCardList.vue'

vi.mock('../../ui/AppIcon.vue', () => ({
  default: { template: '<span aria-hidden="true" />' },
}))

const code = (overrides: Partial<PickupCodeItem>): PickupCodeItem => ({
  id: 1,
  code: 'ABC123',
  file_id: 1,
  folder_id: null,
  name: 'alpha.txt',
  type: 'file',
  download: 3,
  max_download: 10,
  expire_time: '2026-01-08T00:00:00Z',
  created_at: '2025-12-01T00:00:00Z',
  status: 'Active',
  ...overrides,
})

const mountList = (items: PickupCodeItem[]) =>
  mount(PickupCodeCardList, {
    props: { items },
    global: { stubs: { AppIcon: true } },
  })

/**
 * 窄屏卡片列表的接缝：一条取件码在窄屏上要看得见哪几件事，以及能不能打开操作菜单。
 *
 * 为什么单独抽组件：取件码原本只有 `<table>` 一种形态，7 列 + `min-w-[820px]` +
 * `overflow-x-auto`，375px 下必然横向滚动，而 AC 明确禁止横向滚动、要求降级成
 * 两列卡片列表。表格没法在 CSS 里折成卡片，所以窄屏形态必须另写一份结构；
 * 把它抽成组件，这一份结构就能被单独钉住，而不用整页 mount 再猜。
 *
 * 加载态与空态**不在**这里：它们由页面统一渲染一次。两套形态各写一份同样的中文
 * 文案，迟早会漂成两句不一样的话。
 *
 * 断言的都是「用户能看到的字」与「点了会不会出声」：不碰 class 名、不碰内部 ref。
 */
describe('pickup code card list', () => {
  it('shows one card per code with the fields a narrow screen can still carry', () => {
    const wrapper = mountList([
      code({ id: 1, code: 'ABC123', name: '设计稿.pdf', type: 'file' }),
      code({ id: 2, code: 'XYZ789', name: '归档', type: 'folder', status: 'Expired' }),
    ])

    const text = wrapper.text()
    expect(text).toContain('ABC123')
    expect(text).toContain('设计稿.pdf')
    expect(text).toContain('XYZ789')
    expect(text).toContain('归档')
    // 状态是方块点 + 文字，不是彩色胶囊：文字必须在，形状由 atom-badge-dot 负责
    expect(text).toContain('生效中')
    expect(text).toContain('已过期')
    // 用量与过期时间这两件事在窄屏上不能丢，否则卡片等于只剩文件名
    expect(text).toContain('3 / 10')
    expect(text).toContain('2026-01-08')
  })

  it('hands the row to the caller when its action trigger is pressed', async () => {
    const item = code({ id: 7, code: 'MENU01' })
    const wrapper = mountList([item])

    await wrapper.get('button[aria-label="取件码操作菜单"]').trigger('click')

    expect(wrapper.emitted('open-menu')?.[0]?.[0]).toEqual(item)
  })
})
