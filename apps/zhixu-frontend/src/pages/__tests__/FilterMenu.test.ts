import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import FilterMenu from '../../components/ui/FilterMenu.vue'

vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span data-icon />' } }))

describe('shared filter menu', () => {
  it('supports keyboard selection and Escape without a native select', async () => {
    const wrapper = mount(FilterMenu, {
      props: {
        modelValue: '',
        label: '筛选状态',
        options: [
          { key: '', label: '全部' },
          { key: 'ready', label: '已加入' },
        ],
      },
      attachTo: document.body,
    })
    await wrapper.get('[aria-haspopup="menu"]').trigger('click')
    await flushPromises()
    expect(document.activeElement?.textContent).toContain('全部')
    await wrapper.get('[role="menuitemradio"]').trigger('keydown', { key: 'ArrowDown' })
    expect(document.activeElement?.textContent).toContain('已加入')
    await wrapper.findAll('[role="menuitemradio"]')[1]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['ready'])
    await wrapper.get('[aria-haspopup="menu"]').trigger('click')
    await flushPromises()
    await wrapper.get('[role="menuitemradio"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.get('[aria-haspopup="menu"]').attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('select').exists()).toBe(false)
    wrapper.unmount()
  })

  it('supports a compact row, an optional leading icon and a disabled trigger', async () => {
    const compact = mount(FilterMenu, {
      props: {
        modelValue: 'related',
        label: '关系类型',
        compact: true,
        icon: false,
        options: [
          { key: 'related', label: '相关' },
          { key: 'updates', label: '更新替代' },
        ],
      },
    })
    const trigger = compact.get('[aria-haspopup="menu"]')
    // 紧凑高度用于详情页侧栏；icon=false 时只剩展开箭头一个图标。
    expect(trigger.classes()).toContain('min-h-9')
    expect(trigger.findAll('[data-icon]')).toHaveLength(1)
    expect(trigger.html()).not.toContain('material-symbols:filter-list')
    expect(trigger.text()).toContain('相关')
    await trigger.trigger('click')
    await flushPromises()
    expect(compact.find('[role="menu"]').exists()).toBe(true)
    compact.unmount()

    const regular = mount(FilterMenu, {
      props: {
        modelValue: '',
        label: '筛选状态',
        options: [{ key: '', label: '全部' }],
      },
    })
    expect(regular.get('[aria-haspopup="menu"]').classes()).toContain('min-h-11')
    expect(regular.get('[aria-haspopup="menu"]').html()).toContain('material-symbols:filter-list')
    regular.unmount()

    const disabled = mount(FilterMenu, {
      props: {
        modelValue: '',
        label: '筛选状态',
        disabled: true,
        options: [{ key: '', label: '全部' }],
      },
    })
    const disabledTrigger = disabled.get('[aria-haspopup="menu"]')
    expect(disabledTrigger.attributes('disabled')).toBeDefined()
    expect(disabledTrigger.classes()).toContain('disabled:cursor-not-allowed')
    disabled.unmount()
  })
})
