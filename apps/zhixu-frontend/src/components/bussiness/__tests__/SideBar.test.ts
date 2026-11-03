import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import SideBar from '../SideBar.vue'

vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))

async function render(props: Record<string, unknown> = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  await router.push('/home/knowledge')
  await router.isReady()
  const wrapper = mount(SideBar, {
    props: { brandTitle: '知序', brandSubtitle: '', navItems: [], ...props },
    global: { plugins: [router] },
  })
  await flushPromises()
  return wrapper
}

describe('SideBar', () => {
  it('shows the current user and emits logout from the logout button', async () => {
    const wrapper = await render({ userName: '小明', userEmail: 'ming@example.com' })
    expect(wrapper.text()).toContain('小明')
    expect(wrapper.text()).toContain('ming@example.com')
    // 无头像时显示用户名首字符占位
    expect(wrapper.text()).toContain('小')
    await wrapper.get('[aria-label="退出登录"]').trigger('click')
    expect(wrapper.emitted('logout')).toHaveLength(1)
    wrapper.unmount()
  })

  it('hides the user section without a loaded profile', async () => {
    const wrapper = await render()
    expect(wrapper.find('[aria-label="退出登录"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders the avatar image when avatar_url exists', async () => {
    const wrapper = await render({
      userName: '小明',
      userEmail: 'ming@example.com',
      userAvatar: 'https://example.com/a.png',
    })
    const img = wrapper.get('img[alt="小明 的头像"]')
    expect(img.attributes('src')).toBe('https://example.com/a.png')
    wrapper.unmount()
  })
})
