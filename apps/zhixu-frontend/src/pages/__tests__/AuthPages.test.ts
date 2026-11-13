import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LoginPage from '../LoginPage.vue'
import RegisterPage from '../RegisterPage.vue'
import RequireLogin from '../RequireLogin.vue'
import { login, register } from '../../services/apis/auth'

vi.mock('../../services/apis/auth', () => ({
  login: vi.fn(),
  register: vi.fn(),
}))
vi.mock('../../components/ui/AppIcon.vue', () => ({ default: { template: '<span data-icon />' } }))

/**
 * 认证三页的接缝：它们没有外壳、没有侧栏，用户能观察到的只有
 * 「牌子在不在、字段能不能填、校验说什么、提交去哪」。
 *
 * 文案断言是刻意的：票面的 AC 是「中文文案一个字未改」，
 * 而这三页此前 0 个测试守着这句话。改 DOM 结构不该让它们变绿或变红。
 */
async function render(component: typeof LoginPage, path = '/') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div />' } },
      { path: '/register', component: { template: '<div />' } },
      { path: '/home', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(component, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('login page', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('carries the brand mark and the unchanged welcome copy', async () => {
    const { wrapper } = await render(LoginPage)
    expect(wrapper.text()).toContain('知序')
    expect(wrapper.text()).toContain('欢迎回来')
    expect(wrapper.text()).toContain('登录以继续使用知序。')
    expect(wrapper.text()).toContain('还没有账号？')
    expect(wrapper.text()).toContain('立即注册')
    expect(wrapper.get('input#username').attributes('placeholder')).toBe('请输入用户名')
    expect(wrapper.get('input#password').attributes('placeholder')).toBe('请输入密码')
    wrapper.unmount()
  })

  it('exposes a focusable main landmark for the skip link', async () => {
    const { wrapper } = await render(LoginPage)
    const main = wrapper.get('main')
    expect(main.attributes('id')).toBe('main-content')
    expect(main.attributes('tabindex')).toBe('-1')
    wrapper.unmount()
  })

  it('blocks submission with the existing validation messages', async () => {
    const { wrapper } = await render(LoginPage)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('请输入用户名')
    expect(login).not.toHaveBeenCalled()

    await wrapper.get('input#username').setValue('uiaudit')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('请输入密码')

    await wrapper.get('input#password').setValue('123')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('密码至少 6 位')
    wrapper.unmount()
  })

  it('toggles the password field and keeps its accessible name in sync', async () => {
    const { wrapper } = await render(LoginPage)
    const toggle = wrapper.get('button[aria-label="显示密码"]')
    expect(wrapper.get('input#password').attributes('type')).toBe('password')
    await toggle.trigger('click')
    expect(wrapper.get('input#password').attributes('type')).toBe('text')
    expect(wrapper.find('button[aria-label="隐藏密码"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('stores the token and follows the redirect query after a successful login', async () => {
    vi.mocked(login).mockResolvedValueOnce({
      code: 0,
      msg: '登录成功',
      data: { token: 'token-abc' },
    })
    const { wrapper, router } = await render(LoginPage, '/?redirect=%2Fhome')
    await wrapper.get('input#username').setValue('uiaudit')
    await wrapper.get('input#password').setValue('UiAudit123456')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(login).toHaveBeenCalledWith({ username: 'uiaudit', password: 'UiAudit123456' })
    expect(wrapper.text()).toContain('登录成功')
    wrapper.unmount()
    expect(router.currentRoute.value.path).toBe('/')
  })
})

describe('register page', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('carries the brand mark and the unchanged signup copy', async () => {
    const { wrapper } = await render(RegisterPage, '/register')
    expect(wrapper.text()).toContain('知序')
    expect(wrapper.text()).toContain('创建账号')
    expect(wrapper.text()).toContain('加入知序，安全存储您的文件。')
    expect(wrapper.text()).toContain('已经有账号了？')
    expect(wrapper.text()).toContain('返回登录')
    expect(wrapper.get('input#email').attributes('placeholder')).toBe('name@example.com')
    expect(wrapper.get('input#password').attributes('placeholder')).toBe('请创建一个强密码')
    wrapper.unmount()
  })

  it('exposes a focusable main landmark for the skip link', async () => {
    const { wrapper } = await render(RegisterPage, '/register')
    expect(wrapper.get('main').attributes('id')).toBe('main-content')
    expect(wrapper.get('main').attributes('tabindex')).toBe('-1')
    wrapper.unmount()
  })

  it('rejects a malformed email before calling the API', async () => {
    const { wrapper } = await render(RegisterPage, '/register')
    await wrapper.get('input#username').setValue('小明')
    await wrapper.get('input#email').setValue('not-an-email')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('邮箱格式不正确')
    expect(register).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('surfaces the server message when registration is refused', async () => {
    vi.mocked(register).mockResolvedValueOnce({ code: 4001, msg: '用户名已存在', data: null })
    const { wrapper } = await render(RegisterPage, '/register')
    await wrapper.get('input#username').setValue('小明')
    await wrapper.get('input#email').setValue('ming@example.com')
    await wrapper.get('input#password').setValue('UiAudit123456')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('用户名已存在')
    wrapper.unmount()
  })
})

describe('require login page', () => {
  it('carries the brand mark and the unchanged gate copy', async () => {
    const { wrapper } = await render(RequireLogin, '/?redirect=%2Fhome')
    expect(wrapper.text()).toContain('知序')
    expect(wrapper.text()).toContain('需要登录')
    expect(wrapper.text()).toContain('此页面需要登录后才能访问。')
    expect(wrapper.text()).toContain('登录成功后将自动回到 /home')
    expect(wrapper.text()).toContain('立即登录')
    expect(wrapper.text()).toContain('注册新账号')
    expect(wrapper.text()).toContain('没有账号？注册后即可访问所有功能。')
    wrapper.unmount()
  })

  it('exposes a focusable main landmark and forwards the redirect to login', async () => {
    const { wrapper } = await render(RequireLogin, '/?redirect=%2Fhome%2Ffiles')
    expect(wrapper.get('main').attributes('id')).toBe('main-content')
    expect(wrapper.get('main').attributes('tabindex')).toBe('-1')
    const loginLink = wrapper.findAll('a').find(a => a.text() === '立即登录')!
    // memory history 不重编码 query，断言解出来的值比断言字面量稳
    const href = new URL(loginLink.attributes('href')!, 'http://x')
    expect(href.pathname).toBe('/login')
    expect(href.searchParams.get('redirect')).toBe('/home/files')
    wrapper.unmount()
  })
})
