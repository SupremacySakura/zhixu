import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import KnowledgeActionMenu from '../../components/knowledge/KnowledgeActionMenu.vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'

vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))
vi.mock('motion-v', () => ({
  AnimatePresence: { template: '<slot />' },
  Motion: { template: '<div><slot /></div>' },
}))
const mounted: ReturnType<typeof mount>[] = []
afterEach(() => {
  mounted.forEach(wrapper => wrapper.unmount())
  mounted.length = 0
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})
async function render(overrides: Partial<KnowledgeItem> = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/home/knowledge/:id', component: { template: '<div />' } },
      { path: '/home/knowledge/:id/chunks', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  const wrapper = mount(KnowledgeActionMenu, {
    props: {
      item: {
        id: 10,
        title: '笔记',
        status: 'ready',
        chunks_total: 7,
        ...overrides,
      } as KnowledgeItem,
    },
    attachTo: document.body,
    global: { plugins: [router] },
  })
  mounted.push(wrapper)
  return { wrapper, router }
}
const menu = () => document.querySelector<HTMLElement>('[role="menu"]')!
const entries = () => [...menu().querySelectorAll<HTMLElement>('[role="menuitem"]')]

describe('knowledge action menu', () => {
  it('keeps only one trigger and opens the three actions without changing the card', async () => {
    const { wrapper, router } = await render()
    expect(wrapper.findAll('button')).toHaveLength(1)
    expect(menu()).toBeNull()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(entries().map(entry => entry.textContent?.trim())).toEqual([
      '查看知识',
      '分块结构',
      '删除知识',
    ])
    expect(menu().querySelector('[role="separator"]')).not.toBeNull()
    expect(entries()[2]!.classList.contains('text-danger')).toBe(true)
    expect(wrapper.emitted('delete')).toBeUndefined()
    entries()[1]!.click()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/home/knowledge/10/chunks')
    expect(menu()).toBeNull()
  })
  it('supports arrows, Home/End, Escape and restoring focus', async () => {
    const { wrapper } = await render()
    await wrapper.get('button').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(document.activeElement).toBe(entries()[0])
    document.activeElement!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }),
    )
    expect(document.activeElement).toBe(entries()[1])
    document.activeElement!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'End', bubbles: true }),
    )
    expect(document.activeElement).toBe(entries()[2])
    document.activeElement!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Home', bubbles: true }),
    )
    expect(document.activeElement).toBe(entries()[0])
    document.activeElement!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    )
    await nextTick()
    expect(menu()).toBeNull()
    expect(document.activeElement).toBe(wrapper.get('button').element)
    await wrapper.get('button').trigger('keydown', { key: 'ArrowUp' })
    await flushPromises()
    expect(document.activeElement).toBe(entries()[2])
  })
  it('emits status and delete actions and hides unavailable chunks', async () => {
    const { wrapper } = await render({ status: 'failed', chunks_total: 0 })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(entries().map(entry => entry.textContent?.trim())).toEqual(['查看失败原因', '删除知识'])
    entries()[0]!.click()
    await nextTick()
    expect(wrapper.emitted('track')?.[0]?.[0]).toMatchObject({ id: 10, status: 'failed' })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    entries()[1]!.click()
    await nextTick()
    expect(wrapper.emitted('delete')?.[0]?.[0]).toMatchObject({ id: 10 })
    expect(menu()).toBeNull()
  })
  it('closes on outside click, scroll, resize and when another card opens', async () => {
    const { wrapper } = await render()
    const other = await render({ id: 11, title: '另一份笔记' })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(menu()).toBeNull()
    for (const event of ['scroll', 'resize']) {
      await wrapper.get('button').trigger('click')
      await flushPromises()
      window.dispatchEvent(new Event(event))
      await nextTick()
      expect(menu()).toBeNull()
      expect(document.activeElement).toBe(wrapper.get('button').element)
    }
    await wrapper.get('button').trigger('click')
    await flushPromises()
    await other.wrapper.get('button').trigger('click')
    await flushPromises()
    expect(document.querySelectorAll('[role="menu"]')).toHaveLength(1)
    expect(wrapper.get('button').attributes('aria-expanded')).toBe('false')
  })
  it('right-aligns the popup and flips above near the bottom', async () => {
    const { wrapper } = await render()
    vi.spyOn(wrapper.get('button').element, 'getBoundingClientRect').mockReturnValue({
      top: window.innerHeight - 60,
      bottom: window.innerHeight - 16,
      right: 700,
    } as DOMRect)
    await wrapper.get('button').trigger('click')
    await flushPromises()
    const popup = menu().parentElement!
    expect(popup.style.left).toBe(`${Math.max(12, Math.min(500, window.innerWidth - 212))}px`)
    expect(popup.style.transformOrigin).toBe('bottom right')
    expect(Number.parseFloat(popup.style.top)).toBeLessThan(window.innerHeight - 60)
  })
})
