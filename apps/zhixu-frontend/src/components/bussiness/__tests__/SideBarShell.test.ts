/**
 * 侧栏外壳契约（#39）
 *
 * 这一组测的是**外壳几何**，不是侧栏的交互。为什么值得单独锁住：
 *
 * 1. 侧栏宽度 200px 与五个分组是 DESIGN.md §Layout 冻结的骨架。外壳是唯一
 *    允许写死像素的地方，一旦有人把它改回 `w-64`（256px）或把 9 项重新拍平，
 *    页面本身的 364 条测试**一条都不会红** —— 它们只关心内容区。
 * 2. 激活态的「3px 蓝条 + #e6e2d4 底 + 蓝字」是世界里唯一的选中语言。
 *    旧的 `bg-primary-tint` 整块填充已被明令退役（DESIGN.md:580
 *    「表示选中：3px 蓝左边条 + #e6e2d4 底，**不是换整个底色**」），
 *    而这条差异在缩略图下肉眼可辨，在测试里却完全不可见。
 * 3. 底部计数行（索引 N · 分块 M）是票面 AC，且它的数字必须走等宽字面。
 *
 * 断言方式说明：happy-dom 不做布局，`getComputedStyle` 拿不到 Tailwind 的
 * 实际像素值（CSS 由 vite 插件在构建期生成，测试进程里没有样式表）。
 * 所以这里断言的是**类的存在**，而不是渲染宽度 —— 类名就是外壳几何的契约载体，
 * 令牌值本身由 `src/style.css` 的 @theme 块保证。
 */
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import SideBar from '../SideBar.vue'

vi.mock('../../ui/AppIcon.vue', () => ({ default: { template: '<span />' } }))

const navItems = [
  { label: '仪表盘', icon: 'material-symbols:dashboard', to: '/home/dashboard', group: '总览' },
  {
    label: '知识库',
    icon: 'material-symbols:menu-book-outline',
    to: '/home/knowledge',
    group: '知识',
  },
  {
    label: '知识图谱',
    icon: 'material-symbols:account-tree-outline',
    to: '/home/knowledge/graph',
    group: '知识',
  },
  {
    label: '演化中心',
    icon: 'material-symbols:sync-alt-rounded',
    to: '/home/evolution',
    group: '知识',
  },
  { label: '文件管理', icon: 'material-symbols:folder-open', to: '/home/files', group: '来源' },
  { label: '上传', icon: 'material-symbols:cloud-upload', to: '/home/upload', group: '来源' },
  {
    label: '取件码',
    icon: 'material-symbols:key-outline',
    to: '/home/pickup-codes',
    group: '来源',
  },
  { label: '对话', icon: 'material-symbols:chat-bubble-outline', to: '/home/chat', group: '对话' },
  { label: '设置', icon: 'material-symbols:settings-outline', to: '/home/settings', group: '系统' },
]

async function render(props: Record<string, unknown> = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  await router.push('/home/knowledge')
  await router.isReady()
  const wrapper = mount(SideBar, {
    props: { brandTitle: '知序', brandSubtitle: '', navItems, ...props },
    global: { plugins: [router] },
  })
  await flushPromises()
  return wrapper
}

describe('SideBar 外壳契约', () => {
  it('桌面侧栏宽 200px（不是旧的 256px），且保留 aside 语义', async () => {
    const wrapper = await render()
    const aside = wrapper.get('aside')
    // w-rail = 200px（--spacing-rail）。旧的 w-64 是 256px，票面明确要求收窄。
    expect(aside.classes()).toContain('w-rail')
    expect(aside.classes()).not.toContain('w-64')
    // 侧栏右边线是「外壳与内容之间」那条 hairline，必须保留
    expect(aside.classes()).toContain('border-r')
    expect(aside.classes()).toContain('border-hairline')
    wrapper.unmount()
  })

  it('导航按五个分组渲染，组标题顺序为 总览 / 知识 / 来源 / 对话 / 系统', async () => {
    const wrapper = await render()
    const groups = wrapper.findAll('[data-test="nav-group"]')
    expect(groups).toHaveLength(5)
    expect(groups.map(group => group.attributes('data-group'))).toEqual([
      '总览',
      '知识',
      '来源',
      '对话',
      '系统',
    ])
    // 组标题必须是独立的一行等宽小组标（micro 字级 + font-type 等宽字面）
    const firstTitle = groups[0].get('[data-test="nav-group-title"]')
    expect(firstTitle.text()).toBe('总览')
    expect(firstTitle.classes()).toContain('font-type')
    expect(firstTitle.classes()).toContain('text-micro')
    wrapper.unmount()
  })

  it('组内项顺序与传入顺序一致，且每项高 29px', async () => {
    const wrapper = await render()
    const groups = wrapper.findAll('[data-test="nav-group"]')
    const knowledgeLinks = groups[1].findAll('a')
    expect(knowledgeLinks.map(link => link.text())).toEqual(['知识库', '知识图谱', '演化中心'])
    for (const link of knowledgeLinks) {
      expect(link.classes()).toContain('h-nav-item')
    }
    wrapper.unmount()
  })

  it('激活项 = 3px 蓝左边条 + #e6e2d4 底 + 蓝字，不用整块 tint 填充', async () => {
    const wrapper = await render()
    // 路由停在 /home/knowledge，最长前缀命中的是「知识库」
    const active = wrapper.findAll('a').find(link => link.attributes('aria-current') === 'page')!
    expect(active.text()).toBe('知识库')
    expect(active.classes()).toContain('bg-nav-active')
    expect(active.classes()).toContain('text-primary')
    // 左边条：3px 蓝。用 border-l-[3px] 之外的令牌写法也行，但必须是 3px。
    expect(active.classes()).toContain('border-l-[3px]')
    expect(active.classes()).toContain('border-l-primary')
    // 退役的整块 tint 填充不许再出现
    expect(active.classes()).not.toContain('bg-primary-tint')

    const inactive = wrapper.findAll('a').find(link => link.text() === '知识图谱')!
    expect(inactive.attributes('aria-current')).toBeUndefined()
    expect(inactive.classes()).not.toContain('bg-nav-active')
    wrapper.unmount()
  })

  it('品牌区高 62px，字标走系统黑体 700 + .2em 字距，方章 16px 蓝底反白「序」', async () => {
    const wrapper = await render()
    const brand = wrapper.get('[data-test="brand"]')
    expect(brand.classes()).toContain('h-brand')
    expect(brand.text()).toContain('知序')
    expect(brand.text()).toContain('序')

    const wordmark = brand.get('[data-test="brand-wordmark"]')
    expect(wordmark.text()).toBe('知序')
    expect(wordmark.classes()).toContain('tracking-[0.2em]')
    // 字标是系统黑体，不能走 Courier（DESIGN.md:524）
    expect(wordmark.classes()).not.toContain('font-type')

    const seal = brand.get('[data-test="brand-seal"]')
    expect(seal.text()).toBe('序')
    expect(seal.classes()).toContain('w-seal')
    expect(seal.classes()).toContain('h-seal')
    expect(seal.classes()).toContain('bg-primary')
    expect(seal.classes()).toContain('text-surface-tertiary')
    wrapper.unmount()
  })

  it('底部固定一行等宽计数「索引 N · 分块 M」，数字走 font-type', async () => {
    const wrapper = await render({ indexCount: 142, chunkCount: 1286 })
    const counts = wrapper.get('[data-test="shell-counts"]')
    // 千位分隔符是这行的排版契约：1286 必须显示成 1,286
    expect(counts.text()).toContain('索引 142 · 分块 1,286')
    expect(counts.classes()).toContain('font-type')
    wrapper.unmount()
  })

  it('计数未知时显示占位符而不是假数据', async () => {
    const wrapper = await render()
    const counts = wrapper.get('[data-test="shell-counts"]')
    expect(counts.text()).toContain('索引 —')
    expect(counts.text()).toContain('分块 —')
    wrapper.unmount()
  })

  it('navItems 为空（未登录/加载中）时不渲染分组，也不炸', async () => {
    const wrapper = await render({ navItems: [] })
    expect(wrapper.findAll('[data-test="nav-group"]')).toHaveLength(0)
    // 导航地标必须在，AT 才能识别为 navigation 区域
    expect(wrapper.find('nav[aria-label="主导航"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('未标注 group 的项落进「其他」兜底分组，不会丢失', async () => {
    const wrapper = await render({
      navItems: [
        { label: '设置', icon: 'material-symbols:settings-outline', to: '/home/settings' },
      ],
    })
    expect(wrapper.findAll('a').map(link => link.text())).toEqual(['设置'])
    wrapper.unmount()
  })
})
