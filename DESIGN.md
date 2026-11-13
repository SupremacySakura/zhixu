---
name: 知序 KnowledgeFlow
description: 索引卡盒 —— 一条知识一张硬卡，交叉引用另立在右侧
colors:
  canvas: "#e9e5db"
  surface: "#f2eee4"
  surface-secondary: "#e2ddd0"
  surface-tertiary: "#fbf8ef"
  ink: "#181818"
  ink-secondary: "#605949"
  ink-tertiary: "#847c6b"
  primary: "#1f4e79"
  primary-hover: "#17405f"
  primary-pressed: "#123350"
  primary-tint: "rgb(31 78 121 / 0.12)"
  danger: "#a8322a"
  warning: "#6f4700"
  success: "#0a5f3c"
  ai: "#10b674"
typography:
  display:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "26px"
    fontWeight: 700
    lineHeight: 1.31
    letterSpacing: "0em"
  title:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "19px"
    fontWeight: 700
    lineHeight: 1.37
    letterSpacing: "0em"
  read:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.75
    letterSpacing: "0em"
  lead:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "15px"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "0em"
  body:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.46
    letterSpacing: "0em"
  note:
    fontFamily: 'PingFang SC, Hiragino Sans GB, Microsoft YaHei, system-ui, sans-serif'
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.42
    letterSpacing: "0em"
  micro:
    fontFamily: "'Courier Prime', ui-monospace, 'PingFang SC', monospace"
    fontSize: "12px"
    fontWeight: 700
    lineHeight: 1.33
    letterSpacing: "0.06em"
rounded:
  xs: "0px"
  sm: "0px"
  md: "0px"
  lg: "0px"
  xl: "0px"
  full: "0px"
spacing:
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "20px"
  xl: "24px"
  2xl: "32px"
  3xl: "48px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface-tertiary}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "32px"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-primary-active:
    backgroundColor: "{colors.primary-pressed}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "32px"
  button-quiet:
    backgroundColor: "{colors.primary-tint}"
    textColor: "{colors.primary-pressed}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "32px"
  panel:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "12px 16px"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "0 10px"
    height: "32px"
  nav-item-active:
    backgroundColor: "#e6e2d4"
    textColor: "{colors.primary}"
    rounded: "{rounded.md}"
    padding: "0 14px 0 13px"
    height: "29px"
---

> **本文件取代 2026-09 的「浅色 Apple 风格设计规范」。**
> 旧世界（Apple 液态界面十条原则、材质三档毛玻璃、弹簧参数 damping 1.0 / response 0.3–0.4、动量投影 `d=0.998`、橡皮筋 `c=0.55`、字号 34/28/22/20/17/15/14/12、圆角 8/10/14/20/full、三档模糊阴影）**已整体退役**，作为反参照保留在文末「被替换的世界」一节，不再是任何决策的依据。
> 替换原因、决策过程与证据见文末「附录：这次替换是怎么发生的」。

## Overview

**Creative North Star: 「索引卡盒」**

知序是一个人的知识库。它的第一动作不是浏览，是**读**：读自己三个月前存进来的长文，读系统替它做的整理，读它和别的条目之间被找出来的关系。所以这个界面不是一叠悬浮的白卡片，而是**一盒被认真维护的索引卡**——马尼拉卡纸的底、印着横线的卡面、打在边角的日期章、以及另立在卡片右侧的那一叠手写交叉引用小卡。

它拒绝品类默认的「圆角白卡 + 柔和阴影 + 胶囊筛选」，也拒绝把工作屏做成报纸、海报或任何世界自身器物的戏服。

**Key Characteristics:**

- **四表面几乎同色**（亮度比 1.08–1.28），层级绝不靠底色差建立，只靠规则线、留白与硬偏移。
- **圆角恒为 0**，所有浮起感由 1–2px 无模糊硬偏移承担，不存在任何柔和阴影。
- **两套字面**：西文与数字走自托管的 Courier Prime（打字机），中文走系统黑体——中文没有打字机字面，汉字 advance 恒为 1.0em，天然等宽。
- **一枚主色**：图书馆蓝 `#1f4e79` 只承担结构与选中，不承担装饰。
- **一枚警示色**：印章红 `#a8322a` 只给真需要被看见的那一条。
- **品牌绿退役**：`#10b674` 不再作主色，仅作 AI 生成内容的语义标记，且**永不作文字色**（在该世界底色上对比度 1.94–2.28，作字不合格）。

**Operate 模式的限额。** 这是工作界面，不是说服界面。世界只借四样东西给界面：**字体、配色、密度、一个招牌动作**。它不提供布局、不提供导航模型、不提供控件形态。控件必须是用户已经会的标准控件；工作屏不得读成报纸、账本或任何世界自身的器物。

**招牌动作（唯一）**：一条知识是一张**硬卡**——顶边一行打字机标题、左上分类号 `KM-0142`、右下日期章、摘要压在蓝色格线上；**交叉引用另立在卡片右侧**，是一叠手写小卡（相关 / 依赖 / 矛盾 / 补充），不是行内图标。**这个形态只属于知识库列表页**，其余 12 页不得复制（见 §Components 的独占条款）。

## Colors

**策略：Restrained。** 中性色全部从世界自身取（马尼拉卡纸的暖灰），只有一个强调色，但它必须做真事：标记当前位置、标记选中、标记主操作。

### Primary

**图书馆蓝** `#1f4e79`（hover `#17405f` / pressed `#123350` / tint `rgb(31 78 121 / 0.12)`）

它是这个世界里唯一被印上去的颜色——索引卡左上角的分类号、书脊标签、借出章。它**只承担结构与选中**：

- 当前页的侧栏激活条（3px 左边线）与激活文字
- 主操作按钮的底
- 焦点环（`outline: 2px solid var(--color-primary)` + `offset: 2px`）
- 交叉引用小卡的类别标签
- 卡片上的蓝色格线（30% 透明）

四表面上的对比度 6.39–8.16，远超 3:1；蓝底反白 8.66。**不做装饰性用色**：不用它画插图、不用它做渐变、不用它填大面积背景。

### Neutral

四档表面，温度是马尼拉卡纸：

| 令牌 | 值 | 角色 | 说明 |
|---|---|---|---|
| `canvas` | `#e9e5db` | 地 | 应用最底层 |
| `surface` | `#f2eee4` | 卡面 | 卡片、面板、输入框 |
| `surface-secondary` | `#e2ddd0` | 侧栏 / 次级面 | 导航栏、表头 |
| `surface-tertiary` | `#fbf8ef` | 纸条 | 交叉引用小卡、反白文字 |

**四者亮度比 1.078–1.277，全部低于 1.2**——这是刻意的。它们读起来是同一张纸的不同部位，不是四个浮起的层。**任何"靠换个底色表示层级"的做法都是错的**（见 §Elevation & Depth）。

墨色三档，各有法定用途：

| 令牌 | 值 | 四表面最差对比度 | 法定用途 |
|---|---|---|---|
| `ink` | `#181818` | **13.09** | 正文、标题、任何要紧的字 |
| `ink-secondary` | `#605949` | **5.12** | 次级信息：meta 行、计数、辅助说明 |
| `ink-tertiary` | `#847c6b` | **3.05** | 仅非关键文本：分组小标签、占位符、省略号。**不得承载任何用户需要读懂的句子** |

### Named Rules

**The Rule-Vs-Hairline Rule.** 世界只有两种线，它们的名字就是它们的用途。`--color-hairline`（墨 26%，对比度 1.72）是**装饰纹理**——卡片描边、分组小标签下的短横，**永远不能当控件边界**（低于 3:1）。`--color-rule`（墨 48%，对比度 **3.01**）是**控件边界**——按钮描边、输入框描边、选中的框。48% 是实测出的 3:1 最小 alpha 值，不要向下取。

**The One Stamp Rule.** 一枚屏幕里只能有一处用印章红。它的意思是"这条真的需要你现在看一眼"——向量失败、矛盾关系、危险操作。它不是强调色，不是装饰色，不能用来做 hover 态或图标色。

**The Green Is Not Text Rule.** `#10b674` 在该世界底色上作文字只有 1.94–2.28，作按钮底上的白字 2.64。它**唯一合格的用法是当底色、墨字压其上**（6.74）。因此：绿只作 `--color-ai` 与 `--color-ai-tint`，只用于标记 AI 生成的内容，**永不作为文字色、永不作为主按钮底色**。

## Typography

### Hierarchy

八档，没有第九档。字号单位全部是像素，行高全部是绝对像素——**不用无单位行高**，因为中文的行高必须与栅格对齐。

| 令牌 | 字号/行高/字重 | 用途 |
|---|---|---|
| `display` | 26/34/700 | 页面主标题（每页最多一个）、长文 h1 |
| `title` | 19/26/700 | 区块标题、页头带标题、弹窗标题 |
| `read` | 16/28/400 | **长文正文唯一合法档位**。28px 行高对中文刚刚好（1.75） |
| `lead` | 15/21/700 | 按钮文字、字段标签、强调短语 |
| `body` | 13/19/400 | 列表正文、表格单元格、常规界面文字 |
| `note` | 12/17/400 | meta 行、计数、辅助说明 |
| `micro` | 12/16/700/`0.06em` | 等宽小标签：分类号、日期章、分组名。**必走 `--font-type`** |

**两套字面，各自的职责不可混**：

- `--font-sans`：系统黑体栈（`-apple-system` → `PingFang SC` → `Hiragino Sans GB` → `Microsoft YaHei`）。承担所有中文与正文。
- `--font-type`：**Courier Prime 自托管**（woff2 子集，两档共 38.1 KB，SIL OFL 1.1），栈尾接系统中文字体。承担所有数字、编号、计数、日期、英文小标签。

`size-adjust: 122%` 是必需的补偿：Courier Prime 的大写墨迹只有 0.580em，而常用汉字墨迹中位 0.916em，两者相差 1.53 倍。122% 落在「与汉字视觉重量匹配」的合理区间（120–125%），**不要用放大 `font-size` 的方式替代 `size-adjust`**（那会破坏行高与栅格）。`font-display: swap` 已定，不用 `optional`——`optional` 会让首次会话整场失去打字机字声音，而 38.1 KB 同源自托管没有慢到需要它的程度。

### Named Rules

**The One Measure Rule.** 长文正文只能有一个宽度：`--container-measure: 40em`（约 640px）。中文页面最舒服的是每行 36–40 字，**硬上限 48 字**。超过 48 字的行，中文眼睛会在换行处丢失位置。`max-w-6xl`（1152px ≈ 72 字）是旧世界的错误，已在令牌层封死。

**The Zero Letter-Spacing Rule.** 中文的 `letter-spacing` **恒为 0**，标题也不许取负值。中文不是西文，加字距会把汉字拆成孤立的方块，读起来像密码。两处例外：①`micro` 档的等宽小标签（`0.06em`）——那是西文排版习惯借来的，用在全是数字与拉丁字母的串上；②**品牌字标「知序」（`.2em`）**——它不是正文，是被当作图形使用的两字标志，见 §Brand Assets。除这两处外，全仓不许再出现非零字距。

**The No-Indent Rule.** 中文长文**不缩进**。段与段之间用 1em 的段距（即 16px）分，不靠首行缩进 2em。这是业界统一做法，也是本仓 `docs/tasks/` 里那份中文排版事实基线的结论。

## Layout

> **本章状态：骨架已定，几何已实测回填（2026-11-13，1440 / 1024 / 768 / 375 四视口 CDP 实测）；三处验收口径仍未验。**
> 下面一至七节的规则本身是冻结的（来自 [#21](https://github.com/SupremacySakura/zhixu/issues/21) 的逐项定案）。**外壳几何与容器宽度已按生产页面实测数字校正**（见 §外壳 与 §容器），但「六个原型在真实生产页面上落地后是否真的彼此可辨」这类**主观可辨度**验收仍未验——见本章末。

### 外壳

**桌面 ≥1024px**：左侧栏 **200px**（原 256px），右边线 1px hairline，底色 `surface-secondary`。品牌区高 62px：「知序」21px/600/`letter-spacing: .2em`，紧跟一个 **16px 蓝底反白的「序」方章**。导航分**五组**（总览 / 知识 / 来源 / 对话 / 系统），每组一行 12px 等宽小组标。每项高 29px，激活态 = 3px 蓝色左边条 + 底 `#e6e2d4` + 文字蓝，**不用整块 tint 填充**。底部固定一行等宽计数：`索引 142 · 分块 1,286`。

**桌面端没有全局顶栏。** 导航只有侧栏这一条轴，每个页面自带一条 64px 的页头带。这不是省事——加一条全局顶栏会形成双层栏杆，吃掉 60px 内容高度，而且 Operate 模式明确要求导航模型停留在用户已会的形态。

**移动 <1024px**：60px 全局条（「知序 + 当前页名」+ 40px 汉堡）+ 左滑抽屉 `min(15rem, 84vw)`。抽屉拖拽关闭的动量阈值不变（`velocity.x < -400 || (offset.x < -80 && velocity.x <= 0)`）。

**硬约束**：`<aside>` 语义必须保留。`ChatPage.test.ts` 用 `findAll('aside')` 计数，期望桌面 1 个、抽屉打开 2 个、关闭后 1 个。

#### 实测几何（2026-11-13，headless Chrome CDP 量生产页面）

| 元素 | 实测值 | 验于 |
|---|---|---|
| 侧栏宽 | **200px**（`--spacing-rail`）；1023 及以下为 0，不占位 | 1440 / 1024 / 1023 / 768 / 375 |
| 侧栏底色 | `rgb(226, 221, 208)` = `surface-secondary` | 1440 |
| 品牌区高 | **62px**（`--spacing-brand`） | 1440 |
| 字标「知序」 | **50×26**，21px，`letter-spacing: 4.2px` = **0.2em** | 1440 |
| 方章「序」 | **16×16**，12px 字 | 1440 |
| 导航项 | **175×29**（`--spacing-nav-item`） | 1440 |
| 底部计数行 | 175×25，12px 等宽，`letter-spacing: 0.72px` = **0.06em** | 1440 |
| 页头带高 | **64px**（`--spacing-band`），所有视口一致 | 1440 / 1024 / 768 / 375 |
| 抽屉宽 | **240px**（= `min(15rem, 84vw)` 在 375 下取 15rem） | 375 |

**容器四档实测宽度**（`padding-inline` 随视口变，故容器宽度 = 视口 − 2×内边距）：

| 视口 | 内边距 | `wide`(72rem) | `panel`(56rem) | `measure`(40rem) |
|---|---|---|---|---|
| 1440 | 24px | **1152px** | 896px | 640px |
| 1024 | 24px | **776px** | — | — |
| 1023 | 20px | **983px** | — | — |
| 768 | 20px | **728px** | — | — |
| 375 | 16px | **343px** | — | — |

**内边距三档实测断点落在 640 / 1024，不是 768**：375→16px、639→16px、**640→20px**、1023→20px、**1024→24px**、1440→24px。也就是说 `wide` 容器在 768–1023 这一档的实际渲染宽度取决于**父级 `shell-page` 的内边距**，而不是容器自己的 `max-width`。

**工具栏实测高**（`.shell-toolbar`，量于知识库页）：1440 下 **85px**（搜索 + 8 个筛选 + 视图切换 + 计数挤成两行）、1024 下 **113px**（三行）、768 下 **53px**（一行）、375 下 **77px**（两行）。页头带下沿到工具栏上沿还有 32px（1440 / 1024）、24px（768）、16px（375）的间距。DESIGN.md 正文写的「工具栏 48px」是**设计意图值**，生产实测没有一处是 48px —— 这一点记为**已知偏离**，未在本次修正（属布局层，需改控件密度才收得住）。

**横向溢出**：1440 / 1024 / 1023 / 768 / 767 / 375 全部 `document.documentElement.scrollWidth === innerWidth`，无横向滚动。

### 六个布局原型（13 个页面全部归位）

| # | 原型 | 页面 | 容器 | 页头带 | 密度 |
|---|---|---|---|---|---|
| ① | 索引列表 | 知识库、文件管理、取件码、演化中心 | `wide` | 64px 实心 | 32/40 行；知识库为 118px 硬卡 |
| ①b | 索引列表·总览变体 | 仪表盘 | `wide` | 64px | 计数块，无行 |
| ② | 长文阅读 | 知识详情、分块结构 | `measure` | 无带 | 正文 16/28 |
| ③ | 画布 | 知识图谱 | `full` | 64px **悬浮** | 满铺 |
| ④ | 表单 | 上传、设置、知识编辑 | `panel` | 64px | 字段 24 / 分节 48 |
| ⑤ | 对话 | AI 问答 | `wide` | 64px | 消息流，输入区固定底 |
| ⑥ | 认证独立页 | 登录、注册、需要登录、取件 | 480px 单卡 | 无带 | 单卡 |

**仪表盘不是第七个原型**，它是 ① 的总览变体：同容器、同页头带、同工具栏纪律，只把行换成计数块。**分块结构页**是 ② 的双列变体：主列 `measure`，右侧分块列表可用到 `panel`（896px）——这是**唯一**被授权突破 `measure` 的长文场景。

### 四级容器

| 角色 | 令牌 | 值 | 用途 |
|---|---|---|---|
| `measure` | `--container-measure` | 40em / 640px | **长文正文唯一合法宽度** |
| `panel` | `--container-panel` | 56rem / 896px | 表单、设置、双列对照 |
| `wide` | `--container-wide` | 72rem / 1152px | 索引列表、表格、对话 |
| `full` | 无 max-width | 100% | 知识图谱画布 |

`wide` 的 1152px **只允许出现在表格行与卡片行上**；页内一旦出现连续正文段落，那段必须收进 `measure`。容器水平内边距：桌面 24px、平板 20px、移动 16px，与页头带同一条左边线。

**内边距三档实测断点在 640 / 1024，不是 768**（`padding-inline: max(16px, (100% - var(--container-wide)) / 2)` 的实际行为）：视口 375 与 639 → **16px**；640 到 1023 → **20px**；1024 及以上 → **24px**。所以「平板 20px」覆盖的是 640–1023，比 §Responsive 的 `md`（768–1023）宽了 128px —— 这是 `max()` 计算的副产物，不是刻意设计，但**实测如此，按实测记录**。三档容器在 1440 视口的实际渲染宽度：`measure` **640px**、`panel` **896px**、`wide` **1152px**；在 1024 视口 `wide` 只剩 **764px**（视口 − 2×24 − 200 侧栏 − 12 滚动条）。

> **12px 滚动条已经计入这些数字**（2026-11-13 实测）：`wide` 在 1024 视口是 764 而非原先记的 776——自绘滚动条恒占 12px 布局宽（见 §Shapes → 滚动条），容器可用宽度随之少 12px。1440 下 `wide` 仍是 1152，因为那时 `max()` 取的是 24px 下限、不取居中值，滚动条被吸收在两侧留白里。**页头带的 `padding-inline` 已改成左右不对称**（左减 12、右加 12），否则它会与容器错开一个滚动条宽度——实测 1440 下 `bandPadL=38 / bandPadR=50`，容器左右缘与页头带内容边**逐像素对齐**。

### 四段结构

统一 **页头带 → 工具栏 → 内容 →（无页脚）**：

1. **页头带 64px**：左标题（`title`）+ 可选等宽计数行（`micro`），右主操作（实心蓝 32px）+ 次操作（描边 32px）。**主次上限 1+2**，超出进溢出菜单。下沿一条 hairline。**不吸顶**——滚走时腾出高度。
2. **工具栏 48px**：搜索 + 筛选 + 右端命中计数（`命中 9 / 142`，等宽）。**吸顶**，`z-toolbar`，下沿一条 hairline。筛选控件统一 26px 高、直角、1px 描边，**禁止胶囊**。
3. **内容区**：索引列表按原型密度；长文无行框、段距 1em、零缩进；表单字段间距 24px、分节 48px + 一节一条 hairline；画布满铺。
4. **不做页面级页脚**。页面级计数与同步时间下沉到侧栏底部。弹窗内的 footer 保留（按钮行是标准弹窗结构）。

**z 序落位**：侧栏 `z-rail`(10) → 吸顶工具栏 `z-toolbar`(20) → 悬浮纸条 `z-slip`(30) → 下拉 `z-menu`(40) → 弹窗 `z-dialog`(50) → Toast `z-toast`(60)。

### Hairline 纪律

**只画在四种交界**：①外壳与内容之间（侧栏右边线）②页头带下沿 ③工具栏下沿 ④表格行与行之间 / 列表项之间。

**一律不画**：表格列与列之间（禁止全网格——会读成账本）、同级并列块之间（用 `gap` 与留白分组）、每个容器的四周（卡片只用 1px hairline 单层描边 + 硬偏移，不加内框）。

**硬上限：任一屏 1440×900 内可见水平线 ≤ 5 条。** 表格行间线豁免（一屏 20 行的表格自然有 20 条行线），但**列线一条都不许有**——实测证明 20 行无列线仍可横向扫读。

### 面包屑与招牌动作独占

**面包屑只在三层以上页面保留**：知识详情、分块结构、知识编辑，共 3 处（现状 69 处）。索引页全部取消——侧栏已指示位置，再写一遍每页浪费 24px。

**招牌动作独占条款**：「硬卡 + 右侧交叉引用纸条」**只属于知识库列表页**。其余 12 页不得复制该形态（禁止硬偏移叠卡 + 右侧纸条的组合），否则立刻退回到旧世界「八个页面一个骨架」的同构。其余页面允许使用的世界材料仅限：字体、配色、密度，以及**自己页面的一个**小动作（文件管理的行首缩略方格、演化中心的裁决按钮位）。

### 页面级节奏

每页的"长相"由 **容器档 × 页头带 × 密度档** 三元组决定，不靠换内容区分。**验收口径：六种节奏在 25% 缩略图下必须彼此可分辨**。

### 待回填（三项）→ 实测到哪一步了

**2026-11-13 更新**：几何已实测回填（见 §外壳 → 实测几何、§四级容器），下面三项里**只有第 3 项拿到了部分证据**，另两项仍是未验。

1. **六个原型的 25% 缩略图可辨度 —— 未验。** 需要把六个原型各截一张 1440×900 再缩到 25% 并排看，本次修正没有做这项（不在修复范围内）。**不要用「四个容器宽度不同」推断它们可辨**——可辨度取决于页头带、密度、留白的组合观感，不是单一数值。
2. **单屏水平线 ≤5 条 —— 未验。** 取件码与图谱两页的实测水平线数没有量过。
3. **118px 硬卡 + 右侧纸条在长滚动中是否可扫读 —— 部分证据。** 知识库当前实测 **32 条**（`GET /knowledge` 返回 `total: 32`），远未到原设想的 142 条。本次给同一批 20 条做了**表格视图**（#35 用户故事 4）：一屏 20 行、行高 32px、表头 33px、无列线，1440×900 下表格整体 1150×673px（底边 875px，20 行全部落在视口内）。也就是说「条数多了怎么办」现在有了第二条路——切表格，而不是折叠卡片。**但 142 条量级的长滚动仍未实测**（没有这么多数据）。

> **修正原则**：不确定就写「未验」，不要用设计意图冒充实测。上面第 1、2 项没有证据，就保持「未验」。

## Elevation & Depth

**这个世界没有投影，只有偏移。** 纸浮在纸上时，露出的不是影子，是**错开的那 2 像素**。

三档，全部无模糊、无扩散、无色彩：

| 令牌 | 值 | 用于 |
|---|---|---|
| `shadow-card` | `2px 2px 0 rgb(24 24 24 / .13), 3px 3px 0 rgb(24 24 24 / .05)` | 卡片、面板。**双层**——第二层错得更开，造出纸的厚度 |
| `shadow-popover` | `2px 2px 0 rgb(24 24 24 / .16)` | 输入框、次按钮、下拉 |
| `shadow-floating` | `2px 2px 0 rgb(24 24 24 / .26)` | 主按钮、弹窗。最深的一档 |

**Tailwind v4 的默认七档阴影全部被封死为 `none`**（`--shadow-2xs` 至 `--shadow-2xl`）。误用 `shadow-md` 不会出错，它会**静默地什么都不画**——这是刻意的陷阱：宁可什么都不出现，也不要出现旧世界的柔和阴影。

### Named Rules

**The Three-Means Rule.** 层级只能由三种手段承担，且**同一元素上最多用两种**：

1. **线**负责分隔——1px `hairline`（装饰，1.72）或 `rule`（控件边界，3.01）
2. **留白**负责分组——16 / 24 / 32 / 48 四档，用 `gap` 表达同级节奏
3. **硬偏移**负责浮起——三档，见上表

**绝不允许第四种手段：换底色。** 四表面亮度比 1.078–1.277，全部低于 1.2，人眼在正常观看条件下几乎分辨不出。想靠把卡面从 `#f2eee4` 换成 `#fbf8ef` 来表示"选中"，是无效的——实测卡面与纸条面的分离度只有 1.091。

**The No-Glass Rule.** 毛玻璃已退役。`backdrop-blur` / `backdrop-saturate` / `--color-material*` 全部不存在于这个世界。半透明模糊是屏幕的材质，不是纸的材质。

## Shapes

**圆角恒为 0。** 九个 Tailwind v4 圆角键全部写死为 `0px`，包括 `--radius-full`（v4 默认没有这个键，不显式写就杀不掉 112 处 `rounded-full`）。

这不是"极简风格"，是世界的形状语言：**索引卡是切的，不是压的**。切角在物理世界里意味着它是从整张纸上裁下来的一块，有确定的边界。

**形语言三件套**：

- **方章**：16px，1px 蓝描边 + 蓝底 + 反白「序」字。用于品牌位与 favicon（16/32/180 三尺寸同源，`fill-rule` 不加——实测 Chrome 走 `nonzero`，与其渲染仅差 0.36%）。
- **规则线母题**：1px 的短横，出现在分组小标签下方、引文带上下、页脚之上。这是**唯一的第二识别元素**——不做纸纹、不做噪点、不做纹理叠加（`mode-operate.md`：「A variant that adds world material, texture, or ornament goes further in the wrong direction」）。
- **纸条**：`surface-tertiary` 底 + 1px `hairline` 描边 + `1px 1px 0` 偏移，17px 高，逐张 `+5px` 阶梯缩进叠放。

### 滚动条

**滚动条属于形语言，不是浏览器的私事。** macOS 原生滚动条是**灰色圆角胶囊**——圆角、灰、无纸面，与这个世界的三条硬规则（圆角恒 0 / 纸色表面 / 浮起只有硬偏移）同时冲突，所以必须自绘。

**做法：12px 槽位里浮一条 8px 纸条。**

| 部分 | 取值 | 为什么 |
|---|---|---|
| 槽位 | `--scrollbar-size: 12px` | 它同时是**外壳几何**（页头带的居中公式要减掉它） |
| thumb 底 | `--color-surface-tertiary` | 与交叉引用纸条同一个纸面 |
| thumb 描边 | `--color-rule`（48%，3.01:1） | thumb 是**控件**不是装饰纹理；hairline 只有 1.72，不够当边界 |
| 浮起 | 左右各 2px 透明边（`background-clip: padding-box`） | 露出的那圈底色就是世界里唯一的浮起手段——**错开的那 2 像素**，与 `shadow-popover` 同一句话 |
| hover | 底换 `surface-secondary` + 描边转 `primary` | 与「选中」同一套语汇 |
| active | 底换 `primary-tint` + 描边 `primary-pressed` | 同上 |
| 圆角 | 0 | 无例外 |

**两条路径互斥，不许同时声明。** WebKit/Blink 走 `::-webkit-scrollbar` 全套；Firefox 只有 `scrollbar-width: thin` + `scrollbar-color` 两个属性（画不出 1px 描边，退到「纸色 thumb + 透明槽」）。用 `@supports selector(::-webkit-scrollbar)` 分流。

**实测警告（Chrome 141，2026-11-13）**：一旦声明 `scrollbar-width` 或 `scrollbar-color`，`::-webkit-scrollbar` 的**全部**自定义会被静默丢弃、退回原生圆角胶囊。这不是优先级问题，是特性开关——写错就是「看起来什么都没做」。

**副作用：滚动条恒占 12px 布局宽。** 只要声明了 `::-webkit-scrollbar` 的宽度，Chrome 就一律改用占位滚动条，`--enable-features=OverlayScrollbar` 也不覆盖（三组 flag 实测同结果）。因此：

1. 主滚动容器一律 `scrollbar-gutter: stable`（`shell-page` / `shell-scroll` 已内置），让槽位恒存在——否则同一套居中公式会在「有滚动」和「没滚动」两种状态下给出两个结果。
2. 页头带的 `padding-left` 减 `--scrollbar-size`、`padding-right` 加 `--scrollbar-size`（见 §Layout）。只修左边会让「左边对齐、右边多出 12px」，比两边都错更难发现。

**唯一的例外**：`SettingsModal` 的 dialog 壳自己会滚，但它挂在**遮罩**上、没有纸面承载，纸色 thumb 在那里会显得悬空，所以那一处保留隐藏。**这是全仓唯一允许隐藏滚动条的地方。**

**已知未覆盖**：横向滚动条用的是同一套样式（已实测 8px 纸条 + 2px 留白），但代码块 `pre`、表格 `table` 这些 `overflow-x-auto` 的内嵌容器在窄屏下的实际观感**未逐页截图复核**。

## Components

**原子层是 13 个，分三层。** 全部落在 `src/components/ui/`（该目录现有 18 个 `.vue`，其中 5 个是组装件而非原子：`ConfirmDialog` / `FilterMenu` / `ProviderEditor` / `ServiceStatusPanel` / `SettingsModal`）。新增组件前先读这一节——**大部分"我们还需要一个组件"的需求，都能被下面 13 个或"明确不建"清单回答**。

这份清单是**按落地后的实况修正过的**，不是最初设想的 11 个：

- 原清单里的 **`AppSelect` 至今零调用点，未建**（见下）。「建一个零调用点的原子正是这套规范要防的事」——这条原则对它自己同样成立。
- **`AppIcon` 是换图标族时被迫新增的**（17 个测试文件 mock `@iconify/vue`，见下节）。
- **`BrandMark` 是品牌位的组装件**，但它是全仓唯一的字标形态，不抽出来就会被 4 个页面各写一遍。
- **`AppFilterBar` 是页面级筛选行的包装**（计数 chip + 一排 `FilterMenu`），3 个页面在用，属于「外壳与原子之间的薄壳」——它不承载视觉，只承载「筛选行怎么排」。

**零调用点的原子是负债，不是资产。** 落地后实测：`AppMenu` 0 处调用、`AppInput` 0 处、`AppTextarea` 0 处。这三个都是按规范先建、但调用点从未出现。它们留着（prop 面已冻结，等真实调用点），但**不许再有第四个**：新增原子必须先有调用点，再有组件。

### 控制层

**Button** — `AppButton`

三个维度，全原子共用同一套词：

- `tone`：`primary` | `secondary` | `quiet` | `danger`（**只有 4 个**，默认 `secondary`）
- `size`：`sm` | `md` | `lg` → **26 / 32 / 36 px**（默认 `md`）
- 状态：不设 `state` prop。由 `disabled` 与 `loading` 两个布尔驱动，`loading` 时组件内部也置 `disabled`。

**禁止的维度**：不设 `rounded`（圆角恒 0，不是可选项）、不设 `shadow`（七档已封死）、不设 `color`（色板只有四枚）、**不使用 `variant` 这个词**（一个概念一个词）。

`iconOnly` 必须给 `ariaLabel`，在 TS 层做成联合类型报错，不靠运行时警告。

**Input / Textarea** — `AppInput` / `AppTextarea`

`invalid` 时边框走 `--color-danger` 并带 `aria-invalid="true"`。滑块、开关、复选框、单选框**一律用原生控件**，不进原子层（`role="switch"` 全仓 1 处、`type="checkbox"` 6 处、`type="radio"` 0 处）。

**Badge** — `AppBadge`

`tone`：`neutral` | `primary` | `info` | `warning` | `danger`。原来的圆点改 **4px 方块**（圆角恒 0）。

**Select** — `AppSelect`（**保留在清单里，但第一批不建**）

全仓 `<select>` 实测 **0 处**——原生下拉早已被筛选菜单取代。等第一次出现"表单里真的需要下拉"的调用点再落地。**建一个零调用点的原子正是这套规范要防的事。**

### 容器层

**Panel** — `AppPanel`：`as`（渲染标签）、`padded`、`tone`、`interactive`。1px hairline 边框 + `shadow-card`；只有 `interactive` 时才加 hover 偏移。**没有 `icon` prop**——需要图标的场景由调用方在 slot 里放 `AppIcon`，保持 prop 面最小。

**Dialog** — `AppDialog`：`role="dialog"` + `aria-modal="true"` + `Teleport`。**`role="dialog"` 与内部按钮结构必须保留**（`EvolutionCenter.test.ts` 有 15+ 处 `[role="dialog"] button[data-action=…]` 断言）。遮罩用实色 `--color-scrim`（55% 墨），**不用 backdrop-blur**。

**Menu** — `AppMenu`：触发器 `<button aria-haspopup="menu" aria-expanded aria-controls>`，面板 `role="menu"`，选项 `<button role="menuitemradio" aria-checked>`。

**`AppMenu` 落地后零调用点**——它没能吸收原 `FilterMenu`：`FilterMenu` 至今仍被 **9 个文件**引用（`AppFilterBar` / `FileToolbar` / `KnowledgeLibrary` / `KnowledgeGraph` / `EvolutionCenter` / `KnowledgeRelationsPanel` / `ModelSettings` / `ProviderEditor` / `ChatPage`）。原因不是漏做，是两者形态不同：`FilterMenu` 是「一个筛选 chip 弹一排单选项」，`AppMenu` 是「一个动作菜单」。真要合并得先决定筛选 chip 的选中态怎么表达——这件事没做，所以现在有**两套菜单**。**新增下拉时用 `FilterMenu`（选值）或 `AppMenu`（选动作），别再写第三套。**

**Toast** — `AppToast`：**单例，只在 `HomePage` 挂一次**。`role="alert"`（error）/ `role="status"`（其余）。订阅 `src/utils/toastBus.ts`；`useToast()` 的五导出 API 保持不变（页面调用方式不动、测试一字不改）。

**EmptyState** — `AppEmptyState`：`title` / `description` / `icon` / `query`（命中搜索时切文案）+ `action` slot。左内边距线与五态对齐。

**Loading** — `AppLoading`：**三档尺寸 14 / 16 / 20**。内部是 1s linear 的 1px 圆环。**`animation` 只允许出现在这个组件内部**——20 处 `animate-spin` 全部收敛到这里。

### Icon — `AppIcon`

不在原始 11 个清单里，是换图标族时必须新增的：17 个测试文件 `vi.mock('@iconify/vue')`，模块路径一变就集体失效。抽 `AppIcon` 后，mock 目标从第三方包换成自己的模块，那 17 个文件只需改 mock 行。

图标族自托管（106 个图标，本地 JSON），三档 16 / 18 / 20。**图标的验收只能靠断网**：`@iconify/vue/offline` 缺图时不报错、不 warn、不占位，只输出空注释——所以"测试绿 + 页面能开"证明不了图标在。

### 明确不建（边界写死，防通胀）

**Table**（页面级结构，不是原子）、**Tabs**（全仓 0 调用）、**Tooltip**（`role="tooltip"` 0 处）、**Pagination**（2 页用，形态各异）、**Drawer**（属外壳）、**Switch / Checkbox / Radio**（用原生）、**表单校验容器**、**`AppPageHeader` / `AppToolbar`**（属外壳，页面直接按 class 写）。

### 样式承载

组件内**只允许** `@apply` 组合两种来源：v4 内置 utility（`bg-surface` / `text-lead` / `gap-2`）+ 自定义 `@utility`（`cabinet-btn` 等五个骨架类）。

**这不是实现细节，是规范条目。** Tailwind v4 的 `@apply` 只能引用真正的 utility——`@layer components` 里的类引不了，会抛 `Cannot apply unknown utility class`。所以五个 `.cabinet-*` 骨架**必须**位于 `@utility` 而非 `@layer components`，否则任何组件内组合都写不出来。

不引 `tailwind-variants` / `cva`：变体映射写成 `Record<Tone, string>` 常量表放组件内。

### 防通胀守卫

`pnpm lint` 里挂一条本地规则（20 行，`defineTemplateBodyVisitor` 读 `class` 属性、剥变体前缀后正则匹配），拦截六类：`text-[NNpx]`、`max-w-6xl`、`rounded-full`、`shadow-md/lg/xl`、`bg-material*`、`backdrop-blur`。**先 `warn` 后 `error` 两阶段**，迁移期白名单挂账。

## Do's and Don'ts

### Do:

- 用 `--color-rule`（48%）画任何控件边界，用 `--color-hairline`（26%）画纯装饰线。
- 长文正文用 `--text-read` + `max-w-measure`，中文每行 36–40 字。
- 层级用线、留白、硬偏移三选二，绝不用第四种手段。
- 数字、编号、计数、日期一律走 `--font-type`（Courier Prime）。
- 主操作放页头带右端，最多 1 主 + 2 次，其余进溢出菜单。
- 新增原子前先读 §Components 的 13 个清单与"明确不建"清单；**没有调用点就不建原子**。
- 空态、加载态、错误态都朝同一左内边距线对齐。
- 用 `AppLoading` 表达加载，用静态 `bg-hairline` 条表达骨架屏。

### Don't:

- 不要用底色差表示层级——四表面亮度比全部低于 1.2，人眼分不出。
- 不要用柔和阴影、模糊阴影、`shadow-md` 一类（七档已封死为 `none`）。
- 不要用圆角、胶囊、`rounded-full`（九个圆角键全部为 0）。
- 不要用毛玻璃、`backdrop-blur`、`--color-material*`。
- 不要把 `#10b674` 用作文字色或主按钮底色（对比度 1.94–2.28）。
- 不要用 `text-[13px]` 一类任意值字号（八档之外没有字级）。
- 不要给表格画列线（会读成账本）；不要让一屏超过 5 条水平线（表格行间线除外）。
- 不要在知识库列表页之外复制"硬卡 + 右侧纸条"形态。
- 不要用弹簧、弹性、回弹、动量投影、橡皮筋——纸会弯会落，但不会弹。
- 不要用 `transition-all`、`active:scale-*`、`hover:-translate-y-*`、`group-hover:scale-*`。
- 不要在组件内自写 `ring-*` / `outline-*` 焦点样式（base 层已统一）。
- 不要把 `loading` 状态的按钮文案换成"处理中…"（文案抖动源，用线环叠加）。
- 不要用 `opacity` 表达禁用态（会把文字压到 AA 以下），用规则线 + 颜色降级。
- 不要建零调用点的原子。`AppMenu` / `AppInput` / `AppTextarea` 是规范先于调用点留下的三个负债——不再添第四个。
- 不要在正文与标题上加 `letter-spacing`（中文恒 0）。全仓只有两处非零：`micro` 档等宽小标签的 `0.06em`，与品牌字标的 `.2em`。

## Motion

**这个世界没有弹簧。** 纸会弯、会落、会错开，但不会回弹、不会自己浮起来、不会带着手指的动量继续走。所以旧世界的 `bounce` / `damping` / `response` / 动量投影 / 橡皮筋**全部不存在**——这不是技术选择，是世界属性。

三档时长、两条曲线，**不新增第四档第三条**：

- `micro` **90ms** — 状态切换、淡入、按压反馈
- `standard` **160ms** — 弹窗、进度推进
- `sheet` **220ms** — 抽屉、贴底面板、拖拽归位
- `--ease-settle` `cubic-bezier(0.2, 0.8, 0.2, 1)` — 所有入场
- `--ease-exit` `cubic-bezier(0.4, 0, 1, 1)` — 所有退场。**退场一律 90ms，永远比入场短**

`@theme` 里 `--default-transition-duration` 与 `--default-transition-timing-function` 已指向 `micro` 与 `settle`：**不带 `duration-*` 的 `transition-*` 自动就是 90ms + settle**，所以 `duration-150` 这类类的正确处置是**删掉，不是换算**。

### 场景节奏

| 场景 | 时长 | 曲线 | 位移 |
|---|---|---|---|
| 列表首次进入 | 90ms | settle | 无，整列统一，**不做 stagger** |
| 筛选变化（结果区） | 90ms | settle | 无 |
| Toast 出现 | 90ms | settle | `y: -8 → 0` |
| Menu | 90ms | settle | `scale: .98 → 1` + 淡入 |
| Dialog / 遮罩 | 160ms | settle | 遮罩纯淡入，面板**不加缩放** |
| 抽屉 / 贴底 sheet | 220ms | settle | `x` 或 `y` 归位 |
| 拖拽松手归位 | 220ms | settle | `x` / `y` → 0 |
| 侧栏激活条 | 90ms | settle | 宽度/位置 |
| 进度条 / 存储环 | 160ms | settle | 数值推进 |
| 工具栏吸顶 / 页头带 / 页面切换 | **0** | — | 无动效 |
| 退场（所有浮层） | 90ms | **exit** | 一律比入场短 |

### 实现载体

`motion-v` 是唯一实现，**禁止 Vue 原生 `<Transition>`**（现状 0 处，保持）。`<MotionConfig reduced-motion="user">` 保留在 `App.vue`。

`src/utils/motion.ts` 三个旧导出名**保留**（改名要动 24 个文件，收益为零），值换成 tween；新增 `overlayIn` / `overlayExit` / `fadeIn` / `snapBack`。

### Reduced Motion

**双轨互补，不重复**：CSS 侧 @layer base 一处全局兜底（`animation-duration` / `transition-duration` / `scroll-behavior` 全部压到 0.01ms），JS 侧 `<MotionConfig reduced-motion="user">` 统一接管。需要读这个值的地方走 `useReducedMotion()`。

## States & Accessibility

> **本章状态：骨架已定；触控热区已在知识库一页实测回填（2026-11-13，375 视口 + `pointer: coarse`）；五态可辨度与两档偏好仍**未验**。**
> 下面五态与地板是冻结的。**可访问性地板里的触控热区一条已有实测数字**（见下），但「在 32 条真实知识、向量态混杂的库上，五种状态是否真的分得清」仍未验——本次修正没有截五态矩阵。回填时回答：见本章末。

### 五态

空、加载、错误、长文、密集列表——**五态朝同一条左内边距线对齐**（24 / 20 / 16 三档），这是它们彼此"是一家人"的唯一线索。

**空态禁止「居中大图标 + 胶囊按钮」**——那是旧世界的同构空态，也是 AI 生成界面的标志性形状。空态用一句话说清"这里为什么是空的" + 一个主操作，靠左，走文本左对齐。

**一个真实的例外**：向量列全为 0 在**新库上是常态，不是异常**。空态文案必须能表达"还没有开始"与"失败了"的差别，不能用同一句"暂无数据"糊过去。

### 可访问性地板

**对比度取 AA**（正文 4.5:1、非文本 3:1），`prefers-contrast: more` 时抬到 AAA。

- 控件边界必须 ≥3:1 → 只能用 `--color-rule`（48%，3.01），**不能用 hairline**（26%，1.72）
- `ink-tertiary`（3.05）**只够 3:1**，因此只用于非关键文本；任何用户需要读懂的句子必须用 `ink` 或 `ink-secondary`
- 焦点环：2px 实心蓝 + `offset: 2px`，**全局一处**，组件内不自写
- **禁用态不用 `opacity`** —— `opacity: 0.5` 会把 `ink-secondary`（5.12）压到 2.5 以下。禁用态用规则线降级 + 颜色降级表达
- 触控热区 **≥44×44 CSS px**（padding 撑开，视觉尺寸可保持 26/32）。**实测（375 视口 + `pointer: coarse`，知识库一页）：49 个 `<button>` 里 45 个达标，4 个不达标** —— 「从网页导入」34×32（页头带次操作）、「筛选」64×36（工具栏筛选 chip）、分页「第 1 页 / 第 2 页」32×32。这 4 个是**已知未达标项**，不在本次修复范围，记为遗留。旧数字「51 个方形图标按钮里 45 个低于 44px，达标率 11.8%」是迁移前的量，已失效。
- **语义标签结构不能变**：`find('button')` 35 处、`aside` 9 处、`select`/`input` 各 6 处断言依赖它

### 待回填（三项）→ 现状

**2026-11-13 更新**：三项**全部仍未验**，本次修正没有做其中任何一项。

1. **五态在真实数据下的实际可辨度 —— 未验。** 库已涨到 **32 条**（`GET /knowledge` 实测 `total: 32`，不是本章状态行原先写的 31 条），向量态也不再是全 pending。哪个状态最容易被误读成另一个，需要五态各截一张并排看，没做。
2. **`prefers-contrast: more` 与 `prefers-reduced-motion` 两档 —— 仍未验。** 全仓从未截图验证过，本次也没有。
3. **五态 × 三视口完整矩阵 —— 未验。** 「五态截图只截成功态等于没做」这条纪律仍然成立。

> 不要因为本章状态行改了「已实测」就以为五态验过了：**已实测的只有触控热区一条**，五态一项都没动。

## Responsive

> **本章状态：骨架已定；三档断点与容器内边距已在生产页面实测回填（2026-11-13，375 / 639 / 640 / 641 / 767 / 768 / 1023 / 1024 / 1279 / 1280 十个宽度逐一量过）；四档验收的截图矩阵只做了本次修复涉及的四处，未做完整矩阵。**
> 三档断点不再只是「原型切片上验过」——生产页面的 `padding-inline`、侧栏出现/消失、对话框形态翻转都已实测。**但实测暴露了一个与本文档不一致的事实**：内边距的断点落在 640 与 1024，不是 768（见下）。

**三档分叉，不多不少**：`base`（<768）/ `md`（768–1023）/ `lg`（≥1024）。`xl` / `2xl` 保留定义但**不作分叉点**，只用于 `max-w-wide` 之外的留白。

**实测（2026-11-13）**：`base` 与 `md` 的分界确实在 **768**——对话框在 767 是贴底全宽抽屉（`top 547 / h 353 / w 767`，`max-width: none`），在 768 是居中卡片（`512px` 宽，上下留白）。`md` 与 `lg` 的分界确实在 **1024**——侧栏 1023 及以下宽 0（不占位）、1024 起 200px 常驻。**但容器水平内边距的断点不是 768**：实测 375→16px、639→16px、**640→20px**、1023→20px、**1024→24px**。也就是说内边距多出一个 640 分界，`base` 档内部被它切成了两半。这不是刻意设计（来自 `padding-inline: max(16px, (100% - var(--container-wide)) / 2)` 的计算行为），但**它确实存在**，故按实测记录，不按设计意图写。

**一处已修正的断点错位**：`atoms.css` 里对话框的贴底抽屉媒体查询原先写的是 `max-width: 640px`，与「`base` = <768」这条唯一分叉不符——640–767 这 128px 宽的区间里，页面已经是 base 档（侧栏没了、全局条顶上），对话框却还是桌面居中卡片。本次改成 `767px`，实测 767 贴底全宽、768 居中 512px，与三档定义对齐。

`lg = 1024px` 是**唯一开关**：≥1024 侧栏 200px 常驻，<1024 全局条 60px + 抽屉。这是全仓唯一的「桌面/移动」判定点。

**验收四档、必过三档**：375×667（无横向溢出 + 抽屉 + 单列）、768×1024（双列或单列 + 密度降为 md）、1440×900（满配）；1024×768 只验「侧栏恰出现、抽屉恰消失」。

**实测（2026-11-13）**：375 / 768 / 1440 三档**均无横向溢出**（`document.documentElement.scrollWidth === innerWidth`，六个宽度全部相等）；375 下抽屉宽 **240px**、`<aside>` 计数 2；1024 与 1023 各量一次，侧栏在 1024 恰好出现（200px）、1023 恰好为 0。

**窄屏的结构性降级**（不是缩放，是换结构）：筛选条收成「筛选 (N)」单按钮 → 底部 sheet；表格变两列卡列表（**不做横向滚动**）；交叉引用纸条从卡右侧移到卡内下方。

**一个必须记住的前提**：`<meta name="viewport">` 在全仓实测 **0 处**。缺它时移动浏览器按 980px 布局视口渲染，所有 375px 判据在真机上失效——DevTools 响应式模式会掩盖这一点。这是批 0 的一行前置。

**明确不做**：原生 App 壳、横屏平板专版、320px 以下、print、RTL。

## Brand Assets

**品牌 = 一枚 16px 方章。** 蓝底 + 1px 蓝描边 + 反白「序」字。它是索引卡左上角分类号与日期章的同一套语言——品牌不是贴上去的标志，是这个世界本来就有的东西。

**字标「知序」**：系统黑体 700 + `letter-spacing: .2em`。**不用 Courier Prime**——中文没有打字机字面，硬凑只会得到错字距的中西混排。

**favicon**：同一方章，16 / 32 / 180 三尺寸同源 + PNG fallback。旧的紫色渐变模板（`#863bff` / `#7e14ff` / `#47bfff`）与 `public/icons.svg`（六个第三方社交图标 sprite，全仓 0 引用）**均已删除**。

**主色即强调色**：蓝 `#1f4e79` 同时承担品牌与强调，不引入第二品牌色。绿只作 `--color-ai`。

## Degradation

四档，全部落在 `@layer base` 与令牌层，页面不需要知道它们存在：

- **`prefers-reduced-motion: reduce`** → 全局 `animation-duration` / `transition-duration` 压到 `0.01ms`，`scroll-behavior: auto`
- **`prefers-reduced-transparency: reduce`** → 本世界无半透明层，此档只保留为将来兼容
- **`prefers-contrast: more`** → 抬到 AAA，`hairline` 补成 `rule`，`ink-tertiary` 升 `ink-secondary`
- **`forced-colors: active`** → 补 `border: 1px solid CanvasText` 兜底

## 分批路线图

**扩展—收缩三段式**——这不是可选写法，是本仓 UI 替换的事实结构：

1. **扩展**（批 0–1）：新令牌与原子层落地，**旧代码一律不动**，视觉零变化。此阶段两条路并存。
2. **迁移**（批 2–4）：按影响范围分批改调用点，每批之间 CI 可过、364 用例不破。
3. **收缩**（批 5）：删兼容层。**必须等 4A–4F 全完**——否则 4 个页面的 `<h1>` 会从 26px 塌成默认字号。

| 批 | 名称 | 内容 |
|---|---|---|
| 0 | 地基 | viewport meta、两个容器令牌、两个默认过渡令牌、`@utility` 迁移、删死代码、守卫骨架。**不碰任何 `.vue`** |
| 1 | 原子层 | 11 个组件 + `AppIcon`、`motion.ts` 全文替换、`toastBus`、`useReducedMotion`、图标数据接入 |
| 2 | 机械收敛 | 全仓 class 替换（出口数字见 #28 评论），只改 `class` 不碰模板结构 |
| 3 | 外壳 | 侧栏 200px 五组、移动条与抽屉、页头带、favicon |
| 4 | 六个页面群 | 4A 索引列表 / 4B 长文 / 4C 画布 / 4D 认证取件 / 4E 表单对话 / 4F 文件取件 |
| 5 | 收缩 | 删兼容层、守卫转 `error` |

**依赖**：`批0 → 批1 → 批2 → 批3 → {4A,4B,4C,4E,4F} → 批5`；**4D 只等批 2，可与批 3 并行**（独立路由，不经 `HomePage`、不引用 `SideBar`）。

## 证据边界

这一节记录**本规范哪些部分是验过的、哪些是推断**。执行时不得把推断当结论引用。

- 三视口截图全部来自 headless Chrome，**未在真机 Safari / 微信内置浏览器验过**；`100dvh` 与抽屉拖拽手感在真机上可能不同。
- `@media (pointer: coarse)` 自动 44px 热区**全仓 0 处，属新增**，口径已定但未验证实际触感。
- 兼容层删除后的塌陷范围是**静态推断**（来自 `text-title-1` 映射到 26px 的静态分析），非运行观察。
- 六类关系在暗色画布上的「线宽轴」是**四轴里最弱的一轴**：DPR=1 下实测 1px/1.5px/2px 的 FWHM 为 1.07/1.18/1.48，全轴跨度只有 1.38×（标称 2×）。`depends_on` 与 `updates` 在四轴里三轴相同，只差线型。
- `related` 关系在 0.55 透明度下有效对比度仅 **2.00:1**，低于非文本 3:1 下限。验收口径：**实线不得低于 0.7**。
- 中文排版数值来自 W3C clreq 与业界样本推导；clreq 全文 `px` 命中 0，表中 px 值均为 pt×4/3 推导。

## 速查表

| 我要… | 用 |
|---|---|
| 画控件边框 | `border border-rule`（48%，3.01） |
| 画装饰线 | `border-hairline`（26%，1.72） |
| 浮起一个卡片 | `shadow-card` + `bg-surface` |
| 主按钮 | `<AppButton tone="primary">`（蓝底反白，32px） |
| 数字/编号/日期 | `font-type` + `text-micro` |
| 长文正文 | `text-read` + `max-w-measure` |
| 页面容器 | `max-w-wide`（列表）/ `max-w-panel`（表单）/ `max-w-measure`（长文）/ 无（画布） |
| 表示选中 | 3px 蓝左边条 + `#e6e2d4` 底，**不是换整个底色** |
| 表示焦点 | 什么都不做（base 层已统一） |
| 表示加载 | `<AppLoading>` |
| 表示禁用 | 规则线降级 + 颜色降级，**不是 `opacity`** |
| 表示 AI 内容 | `bg-ai-tint` + `border-t-2 border-ai`，**不是绿字** |

---

## 附录：被替换的世界

以下内容**整体退役**，仅作反参照保留。它们描述的是 2026-09 那份「浅色 Apple 风格设计规范」所实现的世界，也就是这次替换的对象。

- **十条原则**：响应、直接操纵、可打断性、弹簧优先、速度交接、动量投影、空间一致性、橡皮筋、材质与深度、动效服务于人
- **材质**：`material` 三档毛玻璃（`blur(20px) saturate(180%)`），透明度 0.72 / 0.85
- **弹簧**：默认 `damping 1.0` / `response 0.3–0.4`；抽屉 `0.8 / 0.3`
- **动量投影**：`projected = current + (v/1000) * d / (1-d)`，`d = 0.998`
- **橡皮筋**：`rubberband = (o*d*c) / (d + c*|o|)`，`c = 0.55`
- **字号**：34 / 28 / 22 / 20 / 17 / 15 / 14 / 12（八档，实测被绕过成 17 种）
- **圆角**：8 / 10 / 14 / 20 / full
- **阴影**：card / popover / floating 三档**柔和模糊**双层
- **底色**：`#f5f5f7` 画布 + `#ffffff` 白卡
- **主色**：品牌绿 `#10b674`
- **实施状态**：旧文件 §7 记为「2026-09：已全部完成」——**该状态现记为「已被替换」**，不是被删除。那一轮改造确实完成了，只是它完成的世界不再是这个世界。

## 附录：这次替换是怎么发生的

留档，供后来者判断本规范的来源是否可信。

1. **诊断**（前期审计）：整个应用是"白卡铺在浅灰画布上"的同构样板，17 个页面共享同一骨架；视觉身份近乎为零（无第二品牌色、无插画、无图形语言、无品牌字体）；设计系统没随功能生长（262 处 Tailwind 任意值、17 种字号、196 个裸 `<button>`、主按钮 class 串 52 种近重复）。
2. **判定为 redesign 而非 refinement**：`DESIGN.md` 描述的世界 == 当前实现，用户不满的正是这个世界本身，所以是替换而非打磨。
3. **PRODUCT.md** 先立（`apps/zhixu-frontend/PRODUCT.md`）：产品真相、硬约束、证据边界。它是设计的前置，不是设计的产物。
4. **方向轮掷骰**：无 pin，走 `concept-seed --scope direction --mode operate`，`ASSIGNED INDEX: 7`，`seed:3e5b4b087e29fc24`。七候选跨三个材料家族（纺织、图书馆、记谱法、唱片、网格学、数据艺术…），用户从决策页九张卡里改选了**索引卡盒**——用户的改选覆盖掷骰，这是规则允许且优先的。
5. **方向契约**写在 `.impeccable/surfaces/src-pages-knowledgelibrary-vue.md` 的六区块（THESIS / OWN-WORLD / STORY / FIRST VIEWPORT / FORM / FINISH），**不写在本文件里**——方向契约是建造期的临时约定，本文件是建成后的规范。
6. **令牌与原型先行**：令牌经 WCAG 实测与 Tailwind v4.2.1 编译验证（51/51 工具类）；布局原型做了 27 张切图（3 视口 × 3 视图 × 3 变体）验证世界在真实中文内容上是否立得住。
7. **本文件**：按 impeccable 的规矩，`DESIGN.md` 在第一批构建完成后、从已构建的世界写出来。三章（版面 / 状态 / 响应式）现在只能写骨架，坑位已标。
