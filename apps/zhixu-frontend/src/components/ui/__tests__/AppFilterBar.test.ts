/**
 * AppFilterBar 契约（#39 窄屏筛选收起）
 *
 * 为什么值得单测：这个组件要同时满足两条互相拉扯的要求 ——
 * 桌面端必须**与接入前逐字节一致**（否则 4 个页面的筛选测试全红），
 * 窄屏端必须把控件收进一张升起面板（否则票面的 AC 没实现）。
 *
 * 最容易写错的实现方式是把插槽渲染两遍（桌面一份、面板一份），
 * 那样 `data-test="type-filter"` 会命中两份，`wrapper.find` 的结果
 * 就取决于渲染顺序 —— 现有钩子测试会变成薛定谔的绿。所以这里
 * **专门断言「插槽只出现一次」**。
 */
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AppFilterBar from '../AppFilterBar.vue'

function render(props: Record<string, unknown> = {}) {
  return mount(AppFilterBar, {
    props,
    slots: {
      default: `
        <select data-test="type-filter" aria-label="筛选处理状态"><option>全部</option></select>
        <select data-test="tag-filter" aria-label="筛选标签"><option>全部</option></select>
      `,
    },
    global: { stubs: { AppIcon: true } },
  })
}

describe('AppFilterBar 窄屏筛选收起', () => {
  it('桌面默认平铺：控件都在，没有收起按钮占位', () => {
    const wrapper = render()
    expect(wrapper.find('[data-test="type-filter"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="tag-filter"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('插槽只渲染一次（防止桌面/面板各渲染一份导致钩子命中两份）', () => {
    const wrapper = render()
    expect(wrapper.findAll('[data-test="type-filter"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-test="tag-filter"]')).toHaveLength(1)
    wrapper.unmount()
  })

  it('窄屏触发按钮存在，文案默认「筛选」，带 activeCount 时带角标', () => {
    const plain = render()
    expect(plain.get('[data-test="filter-bar-trigger"]').text()).toContain('筛选')
    plain.unmount()

    const counted = render({ activeCount: 3 })
    expect(counted.get('[data-test="filter-bar-trigger"]').text()).toContain('筛选 3')
    counted.unmount()
  })

  it('activeCount 为 0 时不显示「0」', () => {
    const wrapper = render({ activeCount: 0 })
    expect(wrapper.get('[data-test="filter-bar-trigger"]').text()).not.toContain('0')
    wrapper.unmount()
  })

  it('点触发按钮后面板升起：role=dialog + aria-modal，且面板里能看到控件', async () => {
    const wrapper = render()
    const panel = () => wrapper.get('[data-test="filter-bar-panel"]')

    // 收起态：面板是隐藏的普通容器，不该冒充对话框
    expect(panel().attributes('role')).toBeUndefined()

    await wrapper.get('[data-test="filter-bar-trigger"]').trigger('click')

    expect(panel().attributes('role')).toBe('dialog')
    expect(panel().attributes('aria-modal')).toBe('true')
    expect(panel().attributes('aria-label')).toBe('筛选')
    expect(panel().text()).toContain('完成')
    expect(wrapper.get('[data-test="filter-bar-trigger"]').attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })

  it('面板里点「完成」收起', async () => {
    const wrapper = render()
    await wrapper.get('[data-test="filter-bar-trigger"]').trigger('click')
    const done = wrapper
      .get('[data-test="filter-bar-panel"]')
      .findAll('button')
      .find(b => b.text() === '完成')
    expect(done).toBeTruthy()
    await done!.trigger('click')
    expect(wrapper.get('[data-test="filter-bar-panel"]').attributes('role')).toBeUndefined()
    wrapper.unmount()
  })

  it('点遮罩收起', async () => {
    const wrapper = render()
    await wrapper.get('[data-test="filter-bar-trigger"]').trigger('click')
    // 遮罩是 fixed inset-0 z-menu 的那一层，没有 data-test，按 class 找
    const scrim = wrapper.findAll('div').find(d => d.classes().includes('bg-scrim'))
    expect(scrim).toBeTruthy()
    await scrim!.trigger('click')
    expect(wrapper.get('[data-test="filter-bar-panel"]').attributes('role')).toBeUndefined()
    wrapper.unmount()
  })

  it('面板不用 aside（会打乱 ChatPage 的 findAll(aside) 计数）', () => {
    const wrapper = render()
    expect(wrapper.findAll('aside')).toHaveLength(0)
    wrapper.unmount()
  })

  it('自定义 label 同时作用于按钮与面板标题', async () => {
    const wrapper = render({ label: '筛选知识' })
    expect(wrapper.get('[data-test="filter-bar-trigger"]').text()).toContain('筛选知识')
    await wrapper.get('[data-test="filter-bar-trigger"]').trigger('click')
    expect(wrapper.get('[data-test="filter-bar-panel"]').attributes('aria-label')).toBe('筛选知识')
    wrapper.unmount()
  })
})
