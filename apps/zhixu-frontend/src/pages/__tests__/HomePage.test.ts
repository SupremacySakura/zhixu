import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import HomePage from '../HomePage.vue'
import SideBar from '../../components/bussiness/SideBar.vue'
import { getProfile } from '../../services/apis/auth'
import { useUserStore } from '../../stores/user'

vi.mock('../../services/apis/auth', () => ({
  getProfile: vi.fn(),
  checkLogin: vi.fn(),
}))
vi.mock('@iconify/vue', () => ({ Icon: { template: '<span />' } }))

const profile = {
  id: 1,
  username: '小明',
  email: 'ming@example.com',
  avatar_url: '',
  storage_limit: 1024,
  storage_used: 0,
  created_at: '',
}

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    // 注意：不要把 HomePage 挂到路由上，否则页面内的 <router-view> 会递归渲染自身
    routes: [{ path: '/login', component: { template: '<div />' } }],
  })
  await router.push('/')
  await router.isReady()
  const wrapper = mount(HomePage, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('HomePage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('loads the profile on mount and shows it in the sidebar', async () => {
    useUserStore().setToken('token')
    vi.mocked(getProfile).mockResolvedValueOnce({ code: 0, msg: '', data: profile })
    const { wrapper } = await render()
    expect(getProfile).toHaveBeenCalledTimes(1)
    expect(useUserStore().profile?.username).toBe('小明')
    expect(wrapper.text()).toContain('小明')
    expect(wrapper.text()).toContain('ming@example.com')
    wrapper.unmount()
  })

  it('skips the profile request when logged out', async () => {
    const { wrapper } = await render()
    expect(getProfile).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the cached profile when the refresh fails', async () => {
    const store = useUserStore()
    store.setToken('token')
    store.setProfile(profile)
    vi.mocked(getProfile).mockRejectedValueOnce(new Error('network'))
    const { wrapper } = await render()
    expect(store.profile?.username).toBe('小明')
    expect(wrapper.text()).toContain('小明')
    wrapper.unmount()
  })

  it('confirms before logout, then clears the session and redirects to login', async () => {
    const store = useUserStore()
    store.setToken('token')
    store.setProfile(profile)
    vi.mocked(getProfile).mockResolvedValue({ code: 0, msg: '', data: profile })
    const { wrapper, router } = await render()

    wrapper.findAllComponents(SideBar)[0]!.vm.$emit('logout')
    await flushPromises()
    const dialog = document.querySelector('[role="dialog"]')!
    expect(dialog.textContent).toContain('退出后需要重新登录')

    const confirm = Array.from(dialog.querySelectorAll('button')).find(
      button => button.textContent?.trim() === '退出登录',
    )!
    confirm.click()
    await flushPromises()

    expect(store.token).toBe('')
    expect(store.profile).toBeNull()
    expect(router.currentRoute.value.path).toBe('/login')
    wrapper.unmount()
  })

  it('cancelling the dialog keeps the session', async () => {
    const store = useUserStore()
    store.setToken('token')
    vi.mocked(getProfile).mockResolvedValue({ code: 0, msg: '', data: profile })
    const { wrapper, router } = await render()

    wrapper.findAllComponents(SideBar)[0]!.vm.$emit('logout')
    await flushPromises()
    const dialog = document.querySelector('[role="dialog"]')!
    const cancel = Array.from(dialog.querySelectorAll('button')).find(
      button => button.textContent?.trim() === '取消',
    )!
    cancel.click()
    await flushPromises()

    expect(store.token).toBe('token')
    expect(router.currentRoute.value.path).toBe('/')
    wrapper.unmount()
  })
})
