/**
 * 外壳落位契约（#39）
 *
 * 这一组测的是「十二个页面是否真的住进了同一个外壳」，不是任何一页的业务。
 *
 * 为什么用**读源码**而不是挂载页面：
 * 1. 外壳几何的载体就是 class（`shell-band` / `shell-page` / `shell-wide` …）。
 *    happy-dom 不做布局、测试进程里也没有样式表，挂载后 `getComputedStyle`
 *    拿不到 64px，断言不出任何东西。
 * 2. 要覆盖 12 个页面就得 mock 每个页面背后的 5–10 个 API，成本高且脆弱；
 *    而这里关心的是「页面有没有用外壳的类」，与页面业务无关。
 * 3. 这类检查在本仓已有先例：`scripts/check-world-drift.mjs` 就是对源码跑
 *    Linter 断言规则还活着。结构契约属于同一层级的东西。
 *
 * 用 Vite 的 `?raw` 而不是 `node:fs`：tsconfig.app.json 的 `types` 只有
 * `vite/client`，没有 `@types/node`，`node:fs` 过不了类型检查；`?raw` 是
 * vite/client 声明的合法模块，同一份源码在测试与构建里走同一条解析路径。
 *
 * 它拦得住什么：有人给某个页面重新抄一遍 `<main class="p-4 sm:p-6 lg:p-8">`
 * 加 `max-w-wide`、有人把页头带改成 `sticky`、有人在新页面里塞 `max-w-6xl`。
 * 它拦不住什么：类写对了但内容排版难看 —— 那是截图复核的事。
 */
import { describe, expect, it } from 'vitest'

import KnowledgeLibrary from '../KnowledgeLibrary.vue?raw'
import KnowledgeDetail from '../KnowledgeDetail.vue?raw'
import KnowledgeChunks from '../KnowledgeChunks.vue?raw'
import KnowledgeEdit from '../KnowledgeEdit.vue?raw'
import KnowledgeGraph from '../KnowledgeGraph.vue?raw'
import EvolutionCenter from '../EvolutionCenter.vue?raw'
import ModelSettings from '../ModelSettings.vue?raw'
import DashboardPage from '../DashboardPage.vue?raw'
import UploadFile from '../UploadFile.vue?raw'
import ChatPage from '../ChatPage.vue?raw'
import PickupCodes from '../PickupCodes.vue?raw'
import FileManagement from '../FileManagement.vue?raw'
import FileManagerHeader from '../../components/file-management/FileManagerHeader.vue?raw'

/**
 * 页头带在 FileManagement 里不在页面文件中：它抽在 FileManagerHeader.vue。
 * 这里显式映射，而不是放宽断言 —— 放宽会让「页头带被搬走」变成合法状态。
 */
const SOURCES: Record<string, string> = {
  KnowledgeLibrary,
  KnowledgeDetail,
  KnowledgeChunks,
  KnowledgeEdit,
  KnowledgeGraph,
  EvolutionCenter,
  ModelSettings,
  DashboardPage,
  UploadFile,
  ChatPage,
  PickupCodes,
  FileManagement: FileManagement + FileManagerHeader,
}

/**
 * 只取 `class="…"` / `:class="…"` 属性值，去掉 HTML 注释。
 *
 * 为什么不能直接对整份源码跑正则：页面注释里会写「比原来的 max-w-wide 明显窄」
 * 「面包屑已取消」，图标名里带 `material-symbols:` —— 纯文本匹配会把说明文字
 * 和图标库名当成违规。契约要盯的是**真正作用在 DOM 上的类**。
 */
function classAttrs(page: string) {
  const code = SOURCES[page]!.replace(/<!--[\s\S]*?-->/g, '')
  const found = code.match(/(?::|v-bind:)?class="[^"]*"/g) ?? []
  return found.join('\n')
}

/** 带页头带的页面：索引列表 4 + 表单 3 + 画布 1 + 对话 1 + 仪表盘 1 = 10。 */
const BAND_PAGES = [
  'KnowledgeLibrary',
  'EvolutionCenter',
  'KnowledgeGraph',
  'DashboardPage',
  'ModelSettings',
  'KnowledgeEdit',
  'UploadFile',
  'ChatPage',
  'PickupCodes',
  'FileManagement',
]

/** 无页头带的长文原型（DESIGN.md 原型 ②「无带」）。 */
const HEADLESS_PAGES = ['KnowledgeDetail', 'KnowledgeChunks']

/** 页面级容器落位：每页恰好一档，且只有四种。 */
const CONTAINERS: Record<string, string> = {
  // ① 索引列表 / ①b 仪表盘 / ⑤ 对话
  KnowledgeLibrary: 'shell-wide',
  EvolutionCenter: 'shell-wide',
  DashboardPage: 'shell-wide',
  ChatPage: 'shell-wide',
  PickupCodes: 'shell-wide',
  FileManagement: 'shell-wide',
  // ③ 画布：无 max-width，画布要铺满
  KnowledgeGraph: 'shell-full',
  // ② 长文
  KnowledgeDetail: 'shell-measure',
  // 分块结构是 ② 的双列变体：左 15rem 大纲 + 右正文列，
  // 896 − 240 − 24 = 632，正文列天然落在 measure 640 内，所以容器档是 panel。
  KnowledgeChunks: 'shell-panel',
  // ④ 表单
  ModelSettings: 'shell-panel',
  KnowledgeEdit: 'shell-panel',
  UploadFile: 'shell-panel',
}

const ALL_PAGES = [...BAND_PAGES, ...HEADLESS_PAGES]

describe('外壳落位契约', () => {
  it('带页头带的页面都用 shell-band（64px / hairline / 不吸顶）', () => {
    for (const page of BAND_PAGES) {
      expect(SOURCES[page], `${page} 缺 shell-band`).toContain('shell-band')
    }
  })

  it('页头带一律不吸顶：外壳里 sticky 只允许出现在工具栏', () => {
    for (const page of ALL_PAGES) {
      const code = SOURCES[page]!.replace(/<!--[\s\S]*?-->/g, '')
      // 页头带是 `<header class="…">` 的第一个类串；sticky 出现在 header 上就是违规。
      const headerTags = code.match(/<header[^>]*>/g) ?? []
      for (const tag of headerTags) {
        expect(tag, `${page} 的页头带不许 sticky`).not.toContain('sticky')
      }
    }
  })

  it('每页只有一个页面级容器，且是四档之一', () => {
    for (const page of ALL_PAGES) {
      const code = SOURCES[page]!
      const containerClasses = ['shell-measure', 'shell-panel', 'shell-wide', 'shell-full'].filter(
        name => code.includes(name),
      )
      expect(containerClasses, `${page} 的容器档`).toEqual([CONTAINERS[page]])
    }
  })

  it('旧容器宽度归零：页面级不再出现 max-w-wide/panel/measure 的手写版本', () => {
    for (const page of ALL_PAGES) {
      // 容器档已经收进 shell-* 载体类；页面里再手写 max-w-wide 就是绕过载体。
      expect(classAttrs(page), `${page} 手写了容器宽度`).not.toMatch(
        /max-w-(?:wide|panel|measure)\b/,
      )
    }
  })

  it('旧容器宽度归零：页面里不再有 max-w-6xl / max-w-[960px] / max-w-3xl', () => {
    for (const page of ALL_PAGES) {
      const classes = classAttrs(page)
      expect(classes, `${page} 还有 6xl`).not.toContain('max-w-6xl')
      expect(classes, `${page} 还有 960px`).not.toContain('max-w-[960px]')
      expect(classes, `${page} 还有 3xl`).not.toContain('max-w-3xl')
    }
  })

  it('毛玻璃死类已清干净（material / backdrop-blur / backdrop-saturate）', () => {
    for (const page of ALL_PAGES) {
      const classes = classAttrs(page)
      // material / material-thick 是死类：style.css 与 atoms.css 都没有定义，
      // 视觉全靠同串的 backdrop-blur + backdrop-saturate。批 2 删了模糊，
      // 这批把它们换成显式底色令牌。
      expect(classes, `${page} 还有 material 死类`).not.toMatch(
        /(?:^|["'\s])material(?:-thick)?(?:["'\s]|$)/,
      )
      expect(classes, `${page} 还有 backdrop-blur`).not.toContain('backdrop-blur')
      expect(classes, `${page} 还有 backdrop-saturate`).not.toContain('backdrop-saturate')
    }
  })

  it('页面滚动容器统一用 shell-page（内边距 16/20/24 与页头带同一条左边线）', () => {
    for (const page of ALL_PAGES) {
      // 旧写法是 p-4 sm:p-6 lg:p-8 散落在每个页面；现在收进 shell-page。
      expect(classAttrs(page), `${page} 还留着旧的内边距串`).not.toMatch(
        /p-4 sm:p-6 lg:p-8|p-4 sm:p-6 lg:space-y-8 lg:p-8/,
      )
    }
  })

  it('面包屑只在三层以上页面保留：知识详情 / 分块结构 / 知识编辑，共 3 处', () => {
    const withBreadcrumb = ALL_PAGES.filter(page =>
      SOURCES[page]!.includes('data-test="breadcrumb"'),
    )
    expect(withBreadcrumb.sort()).toEqual(
      ['KnowledgeChunks', 'KnowledgeDetail', 'KnowledgeEdit'].sort(),
    )
  })
})
