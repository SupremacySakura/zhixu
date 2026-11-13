/**
 * 表单与对话群落位契约（#44 批 4E）
 *
 * 这一组测的是「上传 / 模型设置 / 对话三页是否真的住进了索引卡盒世界」，
 * 以及票主七条裁定（R12–R18）有没有被后来的改动悄悄推翻。
 *
 * 为什么和 ShellContract 一样**读源码**而不是挂载页面：
 * 1. 这里关心的是「按钮是不是 AppButton」「加载态是不是 AppLoading」「间距是不是 24/48」
 *    —— 全是 class 与组件标签层面的事实。happy-dom 不做布局，也没有样式表，
 *    挂载后 `getComputedStyle` 拿不到 24px，断言不出任何东西。
 * 2. 要覆盖六个文件就得 mock 背后十几个 API，成本高且脆弱；而这些契约与业务无关。
 * 3. 同一层级的先例：`scripts/check-world-drift.mjs` 对源码跑 Linter 断言规则还活着。
 *
 * 它拦得住什么：有人把发送按钮换成 AppButton（R12 会炸 2 个既有用例）、
 * 有人给 AppLoading 传 label（R13 会炸 ServiceStatusPanel 的 get('[role="status"]')）、
 * 有人把 `gap-6` 改回 `gap-4`、有人把英文徽章写回 Success/Failed。
 * 它拦不住什么：类写对了但排版难看 —— 那是截图复核的事。
 */
import { describe, expect, it } from 'vitest'

import UploadFile from '../UploadFile.vue?raw'
import ModelSettings from '../ModelSettings.vue?raw'
import ChatPage from '../ChatPage.vue?raw'
import ProviderEditor from '../../components/ui/ProviderEditor.vue?raw'
import ServiceStatusPanel from '../../components/ui/ServiceStatusPanel.vue?raw'
import ConversationSidebar from '../../components/chat/ConversationSidebar.vue?raw'

const FILES: Record<string, string> = {
  UploadFile,
  ModelSettings,
  ChatPage,
  ProviderEditor,
  ServiceStatusPanel,
  ConversationSidebar,
}

/** 去掉 HTML 注释再匹配：注释里会写「比原来的 max-w-wide 明显窄」这类说明文字。 */
function stripComments(code: string) {
  return code.replace(/<!--[\s\S]*?-->/g, '')
}

/** 取出所有 `<button …>` 开标签（原生按钮），用于「还有没有旧按钮类」的断言。 */
function buttonTags(code: string) {
  return stripComments(code).match(/<button\b[^>]*>/g) ?? []
}

/** 取出某个组件的所有开标签，用于「有没有传某个 prop」的断言。 */
function componentTags(code: string, name: string) {
  return stripComments(code).match(new RegExp(`<${name}\\b[^>]*>`, 'g')) ?? []
}

describe('表单与对话群落位契约', () => {
  it('三页加载态统一用同一个加载指示零件 AppLoading', () => {
    for (const page of ['UploadFile', 'ModelSettings', 'ChatPage']) {
      expect(FILES[page], `${page} 的加载态没有换成 AppLoading`).toContain('AppLoading')
    }
    // 两个被本批一并换骨的子组件同理：会话列表与基础服务面板的「加载中」也是同一形态。
    expect(ConversationSidebar, '会话列表加载态').toContain('AppLoading')
    expect(ServiceStatusPanel, '基础服务加载态').toContain('AppLoading')
  })

  it('R13：ServiceStatusPanel 的 AppLoading 不传 label（否则出现第二个 role="status"）', () => {
    const tags = componentTags(ServiceStatusPanel, 'AppLoading')
    expect(tags.length, '基础服务面板没有用 AppLoading').toBeGreaterThan(0)
    for (const tag of tags) {
      // ServiceStatusPanel.test.ts:47 用单数 get('[role="status"]')，多一个就抛
      // 「Found multiple elements」。AppLoading 只有在传 label 时才渲染 role="status"。
      expect(tag, `AppLoading 不该传 label：${tag}`).not.toContain('label=')
    }
  })

  it('R12：发送消息按钮与刷新状态按钮保持原生 <button disabled>，不换 AppButton', () => {
    // ChatPage.test.ts:82 断言 [aria-label="发送消息"] 的 disabled 是 defined；
    // ServiceStatusPanel.test.ts:46 断言 get('button') 的 disabled 是 defined。
    // AppButton 忙态不设原生 disabled，换过去这两条立刻炸。
    expect(stripComments(ChatPage)).toMatch(/<button\b[^>]*aria-label="发送消息"/)
    expect(stripComments(ServiceStatusPanel)).toMatch(/<button\b[^>]*:disabled=/)
    expect(
      componentTags(ChatPage, 'AppButton').join('\n'),
      '发送按钮不许换成 AppButton',
    ).not.toContain('发送消息')
  })

  it('R14：三页不引入原生 <select>（6 条 find("select").exists() === false 会炸）', () => {
    for (const [name, code] of Object.entries(FILES)) {
      expect(stripComments(code), `${name} 引入了原生 select`).not.toMatch(/<select\b/)
    }
  })

  it('R18：上传队列的状态徽章说中文，不再出现英文 Success/Failed/Canceled/Merging/Hashing/Pending', () => {
    const code = stripComments(UploadFile)
    for (const english of ['Success', 'Failed', 'Canceled', 'Merging', 'Hashing', 'Pending']) {
      expect(code, `UploadFile 还有英文徽章 ${english}`).not.toMatch(
        new RegExp(`['"\`]${english}['"\`]`),
      )
    }
    // 六个状态的中文必须都在（票面 R18 逐字指定）。
    for (const chinese of ['成功', '失败', '已取消', '合并中', '校验中', '等待中']) {
      expect(code, `UploadFile 缺中文徽章 ${chinese}`).toContain(chinese)
    }
  })

  it('R15：字段间距 24px、分节间距 48px', () => {
    // 字段 24 = gap-6：模型设置的两个检索字段、供应商编辑器的表单栅格。
    expect(stripComments(ModelSettings), '模型设置字段间距').toContain('gap-6')
    expect(stripComments(ProviderEditor), '供应商编辑器字段间距').toContain('gap-6')
    // 分节 48 = mt-12 / gap-12：供应商段与上传页的两大分节。
    expect(stripComments(ModelSettings), '模型设置分节间距').toContain('mt-12')
    expect(stripComments(UploadFile), '上传页分节间距').toContain('gap-12')
    // 旧值不许回潮。
    expect(stripComments(ModelSettings), '字段间距退回 16px').not.toContain('gap-4')
    expect(stripComments(ProviderEditor), '字段间距退回 20px').not.toContain('gap-5')
  })

  it('R15：分节之间画一条 hairline，不再只靠留白', () => {
    // DESIGN §Layout：一节一条 hairline。上传页两大分节、模型设置供应商段各一条。
    expect(stripComments(UploadFile), '上传页分节缺 hairline').toMatch(
      /border-t[^"]*border-hairline/,
    )
    expect(stripComments(ModelSettings), '模型设置分节缺 hairline').toMatch(
      /border-t[^"]*border-hairline/,
    )
  })

  it('禁用态不用 opacity 降级（DESIGN 明禁；卡面上会掉到 AA 以下）', () => {
    for (const [name, code] of Object.entries(FILES)) {
      expect(stripComments(code), `${name} 还有 disabled:opacity-*`).not.toMatch(
        /disabled:opacity-\d+/,
      )
    }
  })

  it('旧按钮类不再出现在任何 <button> 上：全部换 AppButton（发送/刷新两个例外）', () => {
    for (const [name, code] of Object.entries(FILES)) {
      for (const tag of buttonTags(code)) {
        expect(tag, `${name} 的按钮还在用旧兼容类`).not.toMatch(
          /knowledge-(?:button-primary|button-secondary|button-tint|text-action|icon-button)/,
        )
      }
    }
  })

  it('三页都用上了原子零件：AppButton / AppEmptyState / AppBadge 至少各一处', () => {
    const EXPECTED: Record<string, string[]> = {
      UploadFile: ['AppButton', 'AppEmptyState', 'AppBadge', 'AppDialog'],
      ModelSettings: ['AppButton', 'AppEmptyState', 'AppBadge', 'AppPanel'],
      ChatPage: ['AppButton', 'AppEmptyState', 'AppPanel'],
    }
    for (const [page, atoms] of Object.entries(EXPECTED)) {
      for (const atom of atoms) {
        expect(FILES[page], `${page} 没有用 ${atom}`).toContain(atom)
      }
    }
  })
})

describe('对话消息流的 measure 归属', () => {
  it('消息流收在 measure 里，且由子组件承载（页面级容器只能是 shell-wide）', () => {
    // ShellContract 断言每页恰好一个容器档。ChatPage 是 shell-wide，
    // 所以 40em 的正文列必须落在子组件里，否则两个容器档同时出现、契约立刻炸。
    const transcript = ChatPage
    expect(stripComments(transcript), 'ChatPage 出现了第二个容器档').not.toContain('shell-measure')
    expect(stripComments(transcript), 'ChatPage 手写了容器宽度').not.toMatch(
      /max-w-(?:wide|panel|measure)\b/,
    )
    // 正文列挂 text-read（16px）再配 measure —— 40em 按元素自身字号算，
    // 13px 时只有 520px，正是 R5 说的 em 陷阱。
    expect(stripComments(transcript)).toContain('ChatTranscript')
  })
})
