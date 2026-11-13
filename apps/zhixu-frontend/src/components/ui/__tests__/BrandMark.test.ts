import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BrandMark from '../BrandMark.vue'

/**
 * 品牌行的接缝：它是四页唯一共同出现的品牌位。
 * 断言只落在「看得见的东西」与「辅助技术能读到的东西」上：
 * 字标文字、方章的装饰性（aria-hidden），以及方章确实是 16px 见方。
 */
describe('brand mark', () => {
  it('renders the wordmark as the only accessible name', () => {
    const wrapper = mount(BrandMark)
    // 方章里的「序」在 SVG 里，字标是它之外唯一的文字
    expect(wrapper.get('span').text()).toBe('知序')
    // 方章是装饰：旁边已经有文字「知序」，屏幕阅读器不该再念一遍图形
    expect(wrapper.get('svg').attributes('aria-hidden')).toBe('true')
    wrapper.unmount()
  })

  it('draws the seal at 16px with the reversed 序 glyph', () => {
    const wrapper = mount(BrandMark)
    const seal = wrapper.get('svg')
    expect(seal.attributes('width')).toBe('16')
    expect(seal.attributes('height')).toBe('16')
    expect(seal.get('text').text()).toBe('序')
    wrapper.unmount()
  })
})
